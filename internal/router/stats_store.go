// Package router provides request routing and load balancing for LLM deployments.
package router

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrStatsNotFound is returned when stats for a deployment are not found.
	ErrStatsNotFound = errors.New("stats not found for deployment")

	// ErrStoreNotAvailable is returned when the stats store is not available.
	ErrStoreNotAvailable = errors.New("stats store not available")
)

// StatsStore defines the interface for storing and retrieving deployment statistics.
// Implementations can be in-memory (MemoryStatsStore) or distributed (RedisStatsStore).
//
// Design Principles:
//   - Thread-safe: All methods must be safe for concurrent use
//   - Fail-safe: Errors should not cause request failures, only logging
//   - Context-aware: All methods accept context for cancellation and tracing
type StatsStore interface {
	// GetStats retrieves statistics for a deployment.
	// Returns ErrStatsNotFound if the deployment has no recorded stats.
	GetStats(ctx context.Context, deploymentID string) (*DeploymentStats, error)

	// IncrementActiveRequests atomically increments the active request count.
	// This is called when a request starts routing to a deployment.
	IncrementActiveRequests(ctx context.Context, deploymentID string) error

	// DecrementActiveRequests atomically decrements the active request count.
	// This is called when a request completes (success or failure).
	DecrementActiveRequests(ctx context.Context, deploymentID string) error

	// RecordSuccess records a successful request with its metrics.
	// This updates:
	//   - Total request count
	//   - Success count
	//   - Latency history
	//   - TTFT history (for streaming requests)
	//   - TPM/RPM for current minute
	RecordSuccess(ctx context.Context, deploymentID string, metrics *ResponseMetrics) error

	// RecordFailure records a failed request.
	// This updates:
	//   - Total request count
	//   - Failure count
	//   - Cooldown status (if error is cooldown-worthy)
	RecordFailure(ctx context.Context, deploymentID string, err error) error

	// SetCooldown manually sets a cooldown period for a deployment.
	// The deployment will be excluded from routing until the specified time.
	SetCooldown(ctx context.Context, deploymentID string, until time.Time) error

	// GetCooldownUntil returns the cooldown expiration time for a deployment.
	// Returns zero time if no cooldown is active.
	GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error)

	// ListDeployments returns all deployment IDs that have stats recorded.
	// This is useful for monitoring and cleanup operations.
	ListDeployments(ctx context.Context) ([]string, error)

	// DeleteStats removes all stats for a deployment.
	// This is called when a deployment is removed from the router.
	DeleteStats(ctx context.Context, deploymentID string) error

	// Close releases any resources held by the store.
	// After Close is called, the store should not be used.
	Close() error
}
