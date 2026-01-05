package auth

import (
	"testing"
	"time"
)

func TestAPIKey_CanAccessModel(t *testing.T) {
	tests := []struct {
		name          string
		allowedModels []string
		model         string
		expected      bool
	}{
		{"empty allowed models - all access", []string{}, "gpt-4", true},
		{"nil allowed models - all access", nil, "gpt-4", true},
		{"exact match", []string{"gpt-4", "gpt-3.5-turbo"}, "gpt-4", true},
		{"no match", []string{"gpt-4", "gpt-3.5-turbo"}, "claude-3", false},
		{"wildcard", []string{"*"}, "any-model", true},
		{"wildcard with others", []string{"gpt-4", "*"}, "claude-3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{AllowedModels: tt.allowedModels}
			result := key.CanAccessModel(tt.model)
			if result != tt.expected {
				t.Errorf("CanAccessModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestAPIKey_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{"no expiration", nil, false},
		{"expired", &past, true},
		{"not expired", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{ExpiresAt: tt.expiresAt}
			result := key.IsExpired()
			if result != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAPIKey_IsOverBudget(t *testing.T) {
	tests := []struct {
		name        string
		maxBudget   float64
		spentBudget float64
		expected    bool
	}{
		{"no budget limit", 0, 1000, false},
		{"under budget", 100, 50, false},
		{"at budget", 100, 100, true},
		{"over budget", 100, 150, true},
		{"negative max budget", -10, 50, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{MaxBudget: tt.maxBudget, SpentBudget: tt.spentBudget}
			result := key.IsOverBudget()
			if result != tt.expected {
				t.Errorf("IsOverBudget() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTeam_IsOverBudget(t *testing.T) {
	tests := []struct {
		name        string
		maxBudget   float64
		spentBudget float64
		expected    bool
	}{
		{"no budget limit", 0, 5000, false},
		{"under budget", 1000, 500, false},
		{"at budget", 1000, 1000, true},
		{"over budget", 1000, 1500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := &Team{MaxBudget: tt.maxBudget, SpentBudget: tt.spentBudget}
			result := team.IsOverBudget()
			if result != tt.expected {
				t.Errorf("IsOverBudget() = %v, want %v", result, tt.expected)
			}
		})
	}
}
