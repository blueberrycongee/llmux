package auth

import (
	"context"
	"time"
)

// Store defines the interface for API key and tenant data persistence.
type Store interface {
	// API Key operations
	GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	CreateAPIKey(ctx context.Context, key *APIKey) error
	UpdateAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error
	UpdateAPIKeySpent(ctx context.Context, keyID string, amount float64) error
	DeleteAPIKey(ctx context.Context, keyID string) error
	ListAPIKeys(ctx context.Context, teamID *string, limit, offset int) ([]*APIKey, error)

	// Team operations
	GetTeam(ctx context.Context, teamID string) (*Team, error)
	CreateTeam(ctx context.Context, team *Team) error
	UpdateTeamSpent(ctx context.Context, teamID string, amount float64) error
	DeleteTeam(ctx context.Context, teamID string) error
	ListTeams(ctx context.Context, limit, offset int) ([]*Team, error)

	// User operations
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID string) error

	// Usage logging
	LogUsage(ctx context.Context, log *UsageLog) error
	GetUsageStats(ctx context.Context, filter UsageFilter) (*UsageStats, error)

	// Health check
	Ping(ctx context.Context) error
	Close() error
}

// UsageFilter defines filters for querying usage statistics.
type UsageFilter struct {
	APIKeyID  *string
	TeamID    *string
	Model     *string
	Provider  *string
	StartTime time.Time
	EndTime   time.Time
}

// UsageStats contains aggregated usage statistics.
type UsageStats struct {
	TotalRequests   int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	TotalCost       float64 `json:"total_cost"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	SuccessRate     float64 `json:"success_rate"`
	UniqueModels    int     `json:"unique_models"`
	UniqueProviders int     `json:"unique_providers"`
}
