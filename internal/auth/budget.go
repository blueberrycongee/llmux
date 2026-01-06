// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"time"
)

// BudgetDuration represents the budget reset period.
type BudgetDuration string

const (
	BudgetDurationDaily   BudgetDuration = "1d"
	BudgetDurationWeekly  BudgetDuration = "7d"
	BudgetDurationMonthly BudgetDuration = "30d"
	BudgetDurationNever   BudgetDuration = ""
)

// Budget represents a reusable budget configuration that can be shared
// across multiple entities (keys, teams, users).
// This follows LiteLLM's LiteLLM_BudgetTable pattern.
type Budget struct {
	ID                  string             `json:"budget_id"`
	MaxBudget           *float64           `json:"max_budget,omitempty"`            // Hard budget limit
	SoftBudget          *float64           `json:"soft_budget,omitempty"`           // Alert threshold (doesn't block)
	MaxParallelRequests *int               `json:"max_parallel_requests,omitempty"` // Concurrent request limit
	TPMLimit            *int64             `json:"tpm_limit,omitempty"`             // Tokens per minute
	RPMLimit            *int64             `json:"rpm_limit,omitempty"`             // Requests per minute
	ModelMaxBudget      map[string]float64 `json:"model_max_budget,omitempty"`      // Per-model budget limits
	BudgetDuration      BudgetDuration     `json:"budget_duration,omitempty"`       // Reset period
	BudgetResetAt       *time.Time         `json:"budget_reset_at,omitempty"`       // Next reset time
	CreatedAt           time.Time          `json:"created_at"`
	CreatedBy           string             `json:"created_by"`
	UpdatedAt           time.Time          `json:"updated_at"`
	UpdatedBy           string             `json:"updated_by"`
}

// RateLimitType defines how rate limits are enforced.
type RateLimitType string

const (
	// RateLimitGuaranteed raises error if allocation exceeds limit.
	RateLimitGuaranteed RateLimitType = "guaranteed_throughput"
	// RateLimitBestEffort allows over-allocation, enforced at runtime.
	RateLimitBestEffort RateLimitType = "best_effort_throughput"
	// RateLimitDynamic adjusts limits based on usage patterns.
	RateLimitDynamic RateLimitType = "dynamic"
)

// RateLimits holds rate limiting configuration for an entity.
type RateLimits struct {
	TPMLimit      *int64           `json:"tpm_limit,omitempty"`       // Tokens per minute
	RPMLimit      *int64           `json:"rpm_limit,omitempty"`       // Requests per minute
	ModelTPMLimit map[string]int64 `json:"model_tpm_limit,omitempty"` // Per-model TPM
	ModelRPMLimit map[string]int64 `json:"model_rpm_limit,omitempty"` // Per-model RPM
	TPMLimitType  RateLimitType    `json:"tpm_limit_type,omitempty"`
	RPMLimitType  RateLimitType    `json:"rpm_limit_type,omitempty"`
}

// BudgetStatus represents the current budget state.
type BudgetStatus struct {
	IsOverBudget     bool    `json:"is_over_budget"`
	IsOverSoftBudget bool    `json:"is_over_soft_budget"`
	SpentAmount      float64 `json:"spent_amount"`
	RemainingBudget  float64 `json:"remaining_budget"`
	UsagePercent     float64 `json:"usage_percent"`
}

// DurationSeconds returns the budget duration in seconds.
func (d BudgetDuration) DurationSeconds() int64 {
	switch d {
	case BudgetDurationDaily:
		return 86400 // 24 * 60 * 60
	case BudgetDurationWeekly:
		return 604800 // 7 * 24 * 60 * 60
	case BudgetDurationMonthly:
		return 2592000 // 30 * 24 * 60 * 60
	default:
		return 0
	}
}

// NextResetTime calculates the next budget reset time from now.
func (d BudgetDuration) NextResetTime() *time.Time {
	if d == BudgetDurationNever {
		return nil
	}
	next := time.Now().Add(time.Duration(d.DurationSeconds()) * time.Second)
	return &next
}

// IsValid checks if the budget duration is valid.
func (d BudgetDuration) IsValid() bool {
	switch d {
	case BudgetDurationDaily, BudgetDurationWeekly, BudgetDurationMonthly, BudgetDurationNever:
		return true
	default:
		return false
	}
}

// NeedsReset checks if the budget needs to be reset based on reset time.
func (b *Budget) NeedsReset() bool {
	if b.BudgetResetAt == nil {
		return false
	}
	return time.Now().After(*b.BudgetResetAt)
}

// CalculateNextReset updates the budget reset time based on duration.
func (b *Budget) CalculateNextReset() {
	if b.BudgetDuration == BudgetDurationNever {
		b.BudgetResetAt = nil
		return
	}
	b.BudgetResetAt = b.BudgetDuration.NextResetTime()
}

// CheckBudgetStatus evaluates the budget status for a given spent amount.
func (b *Budget) CheckBudgetStatus(spent float64) BudgetStatus {
	status := BudgetStatus{
		SpentAmount: spent,
	}

	if b.MaxBudget != nil && *b.MaxBudget > 0 {
		status.RemainingBudget = *b.MaxBudget - spent
		status.UsagePercent = (spent / *b.MaxBudget) * 100
		status.IsOverBudget = spent >= *b.MaxBudget
	}

	if b.SoftBudget != nil && *b.SoftBudget > 0 {
		status.IsOverSoftBudget = spent >= *b.SoftBudget
	}

	return status
}

// GetModelBudget returns the budget limit for a specific model.
// Returns nil if no model-specific budget is set.
func (b *Budget) GetModelBudget(model string) *float64 {
	if b.ModelMaxBudget == nil {
		return nil
	}
	if budget, ok := b.ModelMaxBudget[model]; ok {
		return &budget
	}
	return nil
}
