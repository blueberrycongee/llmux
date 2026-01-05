// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"time"
)

// APIKey represents an API key with its associated permissions and limits.
type APIKey struct {
	ID            string     `json:"id"`
	KeyHash       string     `json:"-"`          // Never expose hash
	KeyPrefix     string     `json:"key_prefix"` // First 8 chars for identification
	Name          string     `json:"name"`
	TeamID        *string    `json:"team_id,omitempty"`
	UserID        *string    `json:"user_id,omitempty"`
	AllowedModels []string   `json:"allowed_models,omitempty"` // Empty = all models
	RateLimit     int        `json:"rate_limit,omitempty"`     // Requests per minute, 0 = use default
	MaxBudget     float64    `json:"max_budget,omitempty"`     // Monthly budget in USD, 0 = unlimited
	SpentBudget   float64    `json:"spent_budget"`
	Metadata      Metadata   `json:"metadata,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	IsActive      bool       `json:"is_active"`
}

// Team represents a team/organization for multi-tenant isolation.
type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MaxBudget   float64   `json:"max_budget,omitempty"` // Monthly budget
	SpentBudget float64   `json:"spent_budget"`
	RateLimit   int       `json:"rate_limit,omitempty"` // Team-wide rate limit
	Metadata    Metadata  `json:"metadata,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	IsActive    bool      `json:"is_active"`
}

// User represents a user within a team.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TeamID    *string   `json:"team_id,omitempty"`
	Role      string    `json:"role"` // admin, member
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
}

// UsageLog records API usage for billing and analytics.
type UsageLog struct {
	ID           int64     `json:"id"`
	APIKeyID     string    `json:"api_key_id"`
	TeamID       *string   `json:"team_id,omitempty"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	Cost         float64   `json:"cost"`
	LatencyMs    int       `json:"latency_ms"`
	StatusCode   int       `json:"status_code"`
	RequestID    string    `json:"request_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Metadata is a flexible key-value store for custom attributes.
type Metadata map[string]any

// AuthContext holds authentication information for a request.
type AuthContext struct {
	APIKey    *APIKey
	Team      *Team
	User      *User
	RequestID string
}

// CanAccessModel checks if the API key is allowed to use the specified model.
func (k *APIKey) CanAccessModel(model string) bool {
	if len(k.AllowedModels) == 0 {
		return true // No restrictions
	}
	for _, m := range k.AllowedModels {
		if m == model || m == "*" {
			return true
		}
	}
	return false
}

// IsExpired checks if the API key has expired.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsOverBudget checks if the API key has exceeded its budget.
func (k *APIKey) IsOverBudget() bool {
	if k.MaxBudget <= 0 {
		return false // No budget limit
	}
	return k.SpentBudget >= k.MaxBudget
}

// IsOverBudget checks if the team has exceeded its budget.
func (t *Team) IsOverBudget() bool {
	if t.MaxBudget <= 0 {
		return false
	}
	return t.SpentBudget >= t.MaxBudget
}
