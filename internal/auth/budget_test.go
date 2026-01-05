package auth

import (
	"testing"
	"time"
)

func TestBudgetDuration_DurationSeconds(t *testing.T) {
	tests := []struct {
		name     string
		duration BudgetDuration
		expected int64
	}{
		{"daily", BudgetDurationDaily, 86400},
		{"weekly", BudgetDurationWeekly, 604800},
		{"monthly", BudgetDurationMonthly, 2592000},
		{"never", BudgetDurationNever, 0},
		{"invalid", BudgetDuration("invalid"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.duration.DurationSeconds()
			if result != tt.expected {
				t.Errorf("DurationSeconds() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestBudgetDuration_NextResetTime(t *testing.T) {
	t.Run("daily returns future time", func(t *testing.T) {
		before := time.Now()
		result := BudgetDurationDaily.NextResetTime()
		after := time.Now()

		if result == nil {
			t.Fatal("expected non-nil time for daily duration")
		}

		expectedMin := before.Add(86400 * time.Second)
		expectedMax := after.Add(86400 * time.Second)

		if result.Before(expectedMin) || result.After(expectedMax) {
			t.Errorf("NextResetTime() = %v, expected between %v and %v", result, expectedMin, expectedMax)
		}
	})

	t.Run("never returns nil", func(t *testing.T) {
		result := BudgetDurationNever.NextResetTime()
		if result != nil {
			t.Errorf("NextResetTime() = %v, want nil", result)
		}
	})
}

func TestBudgetDuration_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		duration BudgetDuration
		expected bool
	}{
		{"daily", BudgetDurationDaily, true},
		{"weekly", BudgetDurationWeekly, true},
		{"monthly", BudgetDurationMonthly, true},
		{"never", BudgetDurationNever, true},
		{"invalid", BudgetDuration("2d"), false},
		{"empty string is never", BudgetDuration(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.duration.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBudget_NeedsReset(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name      string
		resetAt   *time.Time
		expected  bool
	}{
		{"nil reset time", nil, false},
		{"past reset time", &past, true},
		{"future reset time", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := &Budget{BudgetResetAt: tt.resetAt}
			result := budget.NeedsReset()
			if result != tt.expected {
				t.Errorf("NeedsReset() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBudget_CalculateNextReset(t *testing.T) {
	t.Run("daily duration sets reset time", func(t *testing.T) {
		budget := &Budget{BudgetDuration: BudgetDurationDaily}
		before := time.Now()
		budget.CalculateNextReset()
		after := time.Now()

		if budget.BudgetResetAt == nil {
			t.Fatal("expected non-nil BudgetResetAt")
		}

		expectedMin := before.Add(86400 * time.Second)
		expectedMax := after.Add(86400 * time.Second)

		if budget.BudgetResetAt.Before(expectedMin) || budget.BudgetResetAt.After(expectedMax) {
			t.Errorf("BudgetResetAt = %v, expected between %v and %v",
				budget.BudgetResetAt, expectedMin, expectedMax)
		}
	})

	t.Run("never duration clears reset time", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		budget := &Budget{
			BudgetDuration: BudgetDurationNever,
			BudgetResetAt:  &past,
		}
		budget.CalculateNextReset()

		if budget.BudgetResetAt != nil {
			t.Errorf("BudgetResetAt = %v, want nil", budget.BudgetResetAt)
		}
	})
}

func TestBudget_CheckBudgetStatus(t *testing.T) {
	maxBudget := 100.0
	softBudget := 80.0

	tests := []struct {
		name             string
		maxBudget        *float64
		softBudget       *float64
		spent            float64
		wantOverBudget   bool
		wantOverSoft     bool
		wantRemaining    float64
		wantUsagePercent float64
	}{
		{
			name:             "under both budgets",
			maxBudget:        &maxBudget,
			softBudget:       &softBudget,
			spent:            50,
			wantOverBudget:   false,
			wantOverSoft:     false,
			wantRemaining:    50,
			wantUsagePercent: 50,
		},
		{
			name:             "over soft budget only",
			maxBudget:        &maxBudget,
			softBudget:       &softBudget,
			spent:            85,
			wantOverBudget:   false,
			wantOverSoft:     true,
			wantRemaining:    15,
			wantUsagePercent: 85,
		},
		{
			name:             "at max budget",
			maxBudget:        &maxBudget,
			softBudget:       &softBudget,
			spent:            100,
			wantOverBudget:   true,
			wantOverSoft:     true,
			wantRemaining:    0,
			wantUsagePercent: 100,
		},
		{
			name:             "over max budget",
			maxBudget:        &maxBudget,
			softBudget:       &softBudget,
			spent:            150,
			wantOverBudget:   true,
			wantOverSoft:     true,
			wantRemaining:    -50,
			wantUsagePercent: 150,
		},
		{
			name:             "no max budget",
			maxBudget:        nil,
			softBudget:       &softBudget,
			spent:            1000,
			wantOverBudget:   false,
			wantOverSoft:     true,
			wantRemaining:    0,
			wantUsagePercent: 0,
		},
		{
			name:             "no budgets set",
			maxBudget:        nil,
			softBudget:       nil,
			spent:            1000,
			wantOverBudget:   false,
			wantOverSoft:     false,
			wantRemaining:    0,
			wantUsagePercent: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := &Budget{
				MaxBudget:  tt.maxBudget,
				SoftBudget: tt.softBudget,
			}
			status := budget.CheckBudgetStatus(tt.spent)

			if status.IsOverBudget != tt.wantOverBudget {
				t.Errorf("IsOverBudget = %v, want %v", status.IsOverBudget, tt.wantOverBudget)
			}
			if status.IsOverSoftBudget != tt.wantOverSoft {
				t.Errorf("IsOverSoftBudget = %v, want %v", status.IsOverSoftBudget, tt.wantOverSoft)
			}
			if status.SpentAmount != tt.spent {
				t.Errorf("SpentAmount = %v, want %v", status.SpentAmount, tt.spent)
			}
			if status.RemainingBudget != tt.wantRemaining {
				t.Errorf("RemainingBudget = %v, want %v", status.RemainingBudget, tt.wantRemaining)
			}
			if status.UsagePercent != tt.wantUsagePercent {
				t.Errorf("UsagePercent = %v, want %v", status.UsagePercent, tt.wantUsagePercent)
			}
		})
	}
}

func TestBudget_GetModelBudget(t *testing.T) {
	gpt4Budget := 50.0
	budget := &Budget{
		ModelMaxBudget: map[string]float64{
			"gpt-4":         gpt4Budget,
			"gpt-3.5-turbo": 20.0,
		},
	}

	t.Run("existing model", func(t *testing.T) {
		result := budget.GetModelBudget("gpt-4")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if *result != gpt4Budget {
			t.Errorf("GetModelBudget() = %v, want %v", *result, gpt4Budget)
		}
	})

	t.Run("non-existing model", func(t *testing.T) {
		result := budget.GetModelBudget("claude-3")
		if result != nil {
			t.Errorf("GetModelBudget() = %v, want nil", *result)
		}
	})

	t.Run("nil model budget map", func(t *testing.T) {
		emptyBudget := &Budget{}
		result := emptyBudget.GetModelBudget("gpt-4")
		if result != nil {
			t.Errorf("GetModelBudget() = %v, want nil", *result)
		}
	})
}

func TestRateLimitType_Values(t *testing.T) {
	// Ensure constants have expected values
	if RateLimitGuaranteed != "guaranteed_throughput" {
		t.Errorf("RateLimitGuaranteed = %q, want %q", RateLimitGuaranteed, "guaranteed_throughput")
	}
	if RateLimitBestEffort != "best_effort_throughput" {
		t.Errorf("RateLimitBestEffort = %q, want %q", RateLimitBestEffort, "best_effort_throughput")
	}
	if RateLimitDynamic != "dynamic" {
		t.Errorf("RateLimitDynamic = %q, want %q", RateLimitDynamic, "dynamic")
	}
}
