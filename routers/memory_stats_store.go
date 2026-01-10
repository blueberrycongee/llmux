package routers

import (
	"context"
	"sync"
	"time"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// MemoryStatsStore is an in-memory implementation of StatsStore.
// It stores all statistics in local memory using maps protected by RWMutex.
//
// Characteristics:
//   - Fast: No network calls, nanosecond latency
//   - Local-only: Stats are not shared across multiple instances
//   - No persistence: Stats are lost on restart
//
// Use Cases:
//   - Single-instance deployments
//   - Development and testing
//   - Fallback when Redis is unavailable
type MemoryStatsStore struct {
	mu    sync.RWMutex
	stats map[string]*DeploymentStats

	maxLatencyListSize int
}

// NewMemoryStatsStore creates a new in-memory stats store.
func NewMemoryStatsStore() *MemoryStatsStore {
	return &MemoryStatsStore{
		stats:              make(map[string]*DeploymentStats),
		maxLatencyListSize: 10, // Default: keep last 10 latency samples
	}
}

// NewMemoryStatsStoreWithConfig creates a new in-memory stats store with custom config.
func NewMemoryStatsStoreWithConfig(maxLatencyListSize int) *MemoryStatsStore {
	return &MemoryStatsStore{
		stats:              make(map[string]*DeploymentStats),
		maxLatencyListSize: maxLatencyListSize,
	}
}

// GetStats retrieves statistics for a deployment.
func (m *MemoryStatsStore) GetStats(ctx context.Context, deploymentID string) (*DeploymentStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, ok := m.stats[deploymentID]
	if !ok {
		return nil, ErrStatsNotFound
	}

	// Return a deep copy to prevent external modification
	statsCopy := *stats
	statsCopy.LatencyHistory = append([]float64{}, stats.LatencyHistory...)
	statsCopy.TTFTHistory = append([]float64{}, stats.TTFTHistory...)

	return &statsCopy, nil
}

// IncrementActiveRequests atomically increments the active request count.
func (m *MemoryStatsStore) IncrementActiveRequests(ctx context.Context, deploymentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.getOrCreateStatsLocked(deploymentID)
	stats.ActiveRequests++

	return nil
}

// DecrementActiveRequests atomically decrements the active request count.
func (m *MemoryStatsStore) DecrementActiveRequests(ctx context.Context, deploymentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.getOrCreateStatsLocked(deploymentID)
	if stats.ActiveRequests > 0 {
		stats.ActiveRequests--
	}

	return nil
}

// RecordSuccess records a successful request with its metrics.
func (m *MemoryStatsStore) RecordSuccess(ctx context.Context, deploymentID string, metrics *ResponseMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.getOrCreateStatsLocked(deploymentID)
	stats.TotalRequests++
	stats.SuccessCount++
	stats.LastRequestTime = time.Now()

	// Update latency history
	latencyMs := float64(metrics.Latency.Milliseconds())
	m.appendToHistoryLocked(&stats.LatencyHistory, latencyMs)

	// Update TTFT history for streaming requests
	if metrics.TimeToFirstToken > 0 {
		ttftMs := float64(metrics.TimeToFirstToken.Milliseconds())
		m.appendToHistoryLocked(&stats.TTFTHistory, ttftMs)
	}

	// Update average latency (exponential moving average)
	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = latencyMs
	} else {
		stats.AvgLatencyMs = stats.AvgLatencyMs*0.9 + latencyMs*0.1
	}

	// Update average TTFT
	if metrics.TimeToFirstToken > 0 {
		ttftMs := float64(metrics.TimeToFirstToken.Milliseconds())
		if stats.AvgTTFTMs == 0 {
			stats.AvgTTFTMs = ttftMs
		} else {
			stats.AvgTTFTMs = stats.AvgTTFTMs*0.9 + ttftMs*0.1
		}
	}

	// Update TPM/RPM for current minute
	m.updateUsageStatsLocked(stats, metrics.TotalTokens)

	return nil
}

// RecordFailure records a failed request.
func (m *MemoryStatsStore) RecordFailure(ctx context.Context, deploymentID string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.getOrCreateStatsLocked(deploymentID)
	stats.TotalRequests++
	stats.FailureCount++
	stats.LastRequestTime = time.Now()

	// Check if error is cooldown-worthy (e.g., 500, 503, timeout)
	if llmErr, ok := err.(*llmerrors.LLMError); ok {
		// Add penalty latency for timeout errors (helps lowest-latency routing avoid slow deployments)
		if llmErr.StatusCode == 408 || llmErr.StatusCode == 504 {
			m.appendToHistoryLocked(&stats.LatencyHistory, 1000000.0) // 1000s penalty
		}
	}

	return nil
}

// SetCooldown manually sets a cooldown period for a deployment.
func (m *MemoryStatsStore) SetCooldown(ctx context.Context, deploymentID string, until time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.getOrCreateStatsLocked(deploymentID)
	stats.CooldownUntil = until

	return nil
}

// GetCooldownUntil returns the cooldown expiration time for a deployment.
func (m *MemoryStatsStore) GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, ok := m.stats[deploymentID]
	if !ok {
		return time.Time{}, nil
	}

	return stats.CooldownUntil, nil
}

// ListDeployments returns all deployment IDs that have stats recorded.
func (m *MemoryStatsStore) ListDeployments(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	deploymentIDs := make([]string, 0, len(m.stats))
	for id := range m.stats {
		deploymentIDs = append(deploymentIDs, id)
	}

	return deploymentIDs, nil
}

// DeleteStats removes all stats for a deployment.
func (m *MemoryStatsStore) DeleteStats(ctx context.Context, deploymentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.stats, deploymentID)
	return nil
}

// Close releases any resources held by the store.
func (m *MemoryStatsStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear all stats
	m.stats = make(map[string]*DeploymentStats)
	return nil
}

// getOrCreateStatsLocked returns existing stats or creates new ones.
// MUST be called with m.mu locked.
func (m *MemoryStatsStore) getOrCreateStatsLocked(deploymentID string) *DeploymentStats {
	stats, ok := m.stats[deploymentID]
	if !ok {
		stats = &DeploymentStats{
			MaxLatencyListSize: m.maxLatencyListSize,
			LatencyHistory:     make([]float64, 0, m.maxLatencyListSize),
			TTFTHistory:        make([]float64, 0, m.maxLatencyListSize),
		}
		m.stats[deploymentID] = stats
	}
	return stats
}

// appendToHistoryLocked adds a value to a rolling history slice.
// MUST be called with m.mu locked.
func (m *MemoryStatsStore) appendToHistoryLocked(history *[]float64, value float64) {
	maxSize := m.maxLatencyListSize
	if maxSize <= 0 {
		maxSize = 10 // Default size
	}

	if len(*history) < maxSize {
		*history = append(*history, value)
	} else {
		// Shift left and append
		copy((*history)[0:], (*history)[1:])
		(*history)[len(*history)-1] = value
	}
}

// updateUsageStatsLocked updates TPM/RPM counters for the current minute.
// MUST be called with m.mu locked.
func (m *MemoryStatsStore) updateUsageStatsLocked(stats *DeploymentStats, tokens int) {
	currentMinute := time.Now().Format("2006-01-02-15-04")

	if stats.CurrentMinuteKey != currentMinute {
		// New minute, reset counters
		stats.CurrentMinuteKey = currentMinute
		stats.CurrentMinuteTPM = 0
		stats.CurrentMinuteRPM = 0
	}

	stats.CurrentMinuteTPM += int64(tokens)
	stats.CurrentMinuteRPM++
}
