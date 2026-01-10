package routers

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// ErrNoAvailableDeployment is returned when no healthy deployment is available.
var ErrNoAvailableDeployment = errors.New("no available deployment for model")

// ErrNoDeploymentsWithTag is returned when no deployments match the requested tags.
var ErrNoDeploymentsWithTag = errors.New("no deployments match the requested tags")

// statsEntry tracks performance metrics for a deployment.
type statsEntry struct {
	TotalRequests      int64
	SuccessCount       int64
	FailureCount       int64
	ActiveRequests     int64
	LatencyHistory     []float64
	TTFTHistory        []float64
	AvgLatencyMs       float64
	AvgTTFTMs          float64
	MaxLatencyListSize int
	CurrentMinuteTPM   int64
	CurrentMinuteRPM   int64
	CurrentMinuteKey   string
	LastRequestTime    time.Time
	CooldownUntil      time.Time
}

// BaseRouter provides common functionality for all routing strategies.
// Specific strategies embed this and override the selection logic.
//
// BaseRouter supports two modes of operation:
//   - Local mode (default): Stats are stored in memory, suitable for single-instance deployments
//   - Distributed mode: Stats are stored in a StatsStore (e.g., Redis), suitable for multi-instance deployments
type BaseRouter struct {
	mu          sync.RWMutex
	rngMu       sync.Mutex
	deployments map[string][]*ExtendedDeployment
	stats       map[string]*statsEntry
	config      router.Config
	rng         *rand.Rand
	strategy    router.Strategy

	// statsStore is an optional distributed stats store.
	// When nil, local stats map is used (backward compatible).
	// When set, stats operations delegate to the store (distributed mode).
	statsStore router.StatsStore
}

// NewBaseRouter creates a new base router with the given configuration.
// This creates a router in local mode (stats stored in memory).
func NewBaseRouter(config router.Config) *BaseRouter {
	return &BaseRouter{
		deployments: make(map[string][]*ExtendedDeployment),
		stats:       make(map[string]*statsEntry),
		config:      config,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		strategy:    config.Strategy,
		statsStore:  nil, // Local mode
	}
}

// NewBaseRouterWithStore creates a new base router with a distributed stats store.
// This enables multi-instance deployments to share routing statistics.
func NewBaseRouterWithStore(config router.Config, store router.StatsStore) *BaseRouter {
	r := NewBaseRouter(config)
	r.statsStore = store
	return r
}

// GetStrategy returns the current routing strategy.
func (r *BaseRouter) GetStrategy() router.Strategy {
	return r.strategy
}

func (r *BaseRouter) randIntn(n int) int {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng.Intn(n)
}

func (r *BaseRouter) randFloat64() float64 {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng.Float64()
}

func (r *BaseRouter) randShuffle(n int, swap func(i, j int)) {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	r.rng.Shuffle(n, swap)
}

// AddDeployment registers a new deployment with default configuration.
func (r *BaseRouter) AddDeployment(deployment *provider.Deployment) {
	r.AddDeploymentWithConfig(deployment, router.DeploymentConfig{})
}

// AddDeploymentWithConfig registers a deployment with routing configuration.
func (r *BaseRouter) AddDeploymentWithConfig(deployment *provider.Deployment, config router.DeploymentConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	model := deployment.ModelName
	if deployment.ModelAlias != "" {
		model = deployment.ModelAlias
	}

	extended := &ExtendedDeployment{
		Deployment: deployment,
		Config:     config,
	}

	r.deployments[model] = append(r.deployments[model], extended)
	r.stats[deployment.ID] = &statsEntry{
		MaxLatencyListSize: r.config.MaxLatencyListSize,
		LatencyHistory:     make([]float64, 0, r.config.MaxLatencyListSize),
		TTFTHistory:        make([]float64, 0, r.config.MaxLatencyListSize),
	}
}

// RemoveDeployment removes a deployment from the router.
func (r *BaseRouter) RemoveDeployment(deploymentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for model, deps := range r.deployments {
		for i, d := range deps {
			if d.ID == deploymentID {
				r.deployments[model] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}
	delete(r.stats, deploymentID)
}

// GetDeployments returns all deployments for a model.
func (r *BaseRouter) GetDeployments(model string) []*provider.Deployment {
	r.mu.RLock()
	defer r.mu.RUnlock()

	deps := r.deployments[model]
	result := make([]*provider.Deployment, len(deps))
	for i, d := range deps {
		result[i] = d.Deployment
	}
	return result
}

// GetStats returns the current stats for a deployment.
func (r *BaseRouter) GetStats(deploymentID string) *router.DeploymentStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, ok := r.stats[deploymentID]
	if !ok {
		return nil
	}
	return &router.DeploymentStats{
		TotalRequests:    stats.TotalRequests,
		SuccessCount:     stats.SuccessCount,
		FailureCount:     stats.FailureCount,
		ActiveRequests:   stats.ActiveRequests,
		AvgLatencyMs:     stats.AvgLatencyMs,
		AvgTTFTMs:        stats.AvgTTFTMs,
		CurrentMinuteTPM: stats.CurrentMinuteTPM,
		CurrentMinuteRPM: stats.CurrentMinuteRPM,
		LastRequestTime:  stats.LastRequestTime,
		CooldownUntil:    stats.CooldownUntil,
	}
}

// IsCircuitOpen checks if the deployment is in cooldown.
func (r *BaseRouter) IsCircuitOpen(deployment *provider.Deployment) bool {
	// Distributed mode: check via StatsStore
	if r.statsStore != nil {
		cooldownUntil, err := r.statsStore.GetCooldownUntil(context.Background(), deployment.ID)
		if err != nil {
			// Fail-safe: assume not in cooldown if store error
			return false
		}
		return time.Now().Before(cooldownUntil)
	}

	// Local mode: use local stats map
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, ok := r.stats[deployment.ID]
	if !ok {
		return false
	}
	return time.Now().Before(stats.CooldownUntil)
}

// ReportRequestStart increments the active request count.
func (r *BaseRouter) ReportRequestStart(deployment *provider.Deployment) {
	// Distributed mode: delegate to StatsStore
	if r.statsStore != nil {
		// Fail-safe: ignore errors
		_ = r.statsStore.IncrementActiveRequests(context.Background(), deployment.ID)
		return
	}

	// Local mode: use local stats map
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.ActiveRequests++
}

// ReportRequestEnd decrements the active request count.
func (r *BaseRouter) ReportRequestEnd(deployment *provider.Deployment) {
	// Distributed mode: delegate to StatsStore
	if r.statsStore != nil {
		// Fail-safe: ignore errors
		_ = r.statsStore.DecrementActiveRequests(context.Background(), deployment.ID)
		return
	}

	// Local mode: use local stats map
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	if stats.ActiveRequests > 0 {
		stats.ActiveRequests--
	}
}

// ReportSuccess records a successful request with metrics.
func (r *BaseRouter) ReportSuccess(deployment *provider.Deployment, metrics *router.ResponseMetrics) {
	// Distributed mode: delegate to StatsStore
	if r.statsStore != nil {
		// Fail-safe: ignore errors
		_ = r.statsStore.RecordSuccess(context.Background(), deployment.ID, metrics)
		return
	}

	// Local mode: use local stats map
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.SuccessCount++
	stats.LastRequestTime = time.Now()

	latencyMs := float64(metrics.Latency.Milliseconds())
	r.appendToHistory(&stats.LatencyHistory, latencyMs, stats.MaxLatencyListSize)

	if metrics.TimeToFirstToken > 0 {
		ttftMs := float64(metrics.TimeToFirstToken.Milliseconds())
		r.appendToHistory(&stats.TTFTHistory, ttftMs, stats.MaxLatencyListSize)
	}

	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = latencyMs
	} else {
		stats.AvgLatencyMs = stats.AvgLatencyMs*0.9 + latencyMs*0.1
	}

	r.updateUsageStats(stats, metrics.TotalTokens)
}

// ReportFailure records a failed request and triggers cooldown if needed.
// Cooldown is triggered based on LiteLLM-style failure rate logic:
//   - Immediate cooldown on 429 (Rate Limit) if ImmediateCooldownOn429 is true
//   - Immediate cooldown on non-retryable errors (401, 404)
//   - Failure rate based cooldown when rate exceeds FailureThresholdPercent
func (r *BaseRouter) ReportFailure(deployment *provider.Deployment, err error) {
	// Distributed mode: delegate to StatsStore
	if r.statsStore != nil {
		// Fail-safe: ignore errors
		_ = r.statsStore.RecordFailure(context.Background(), deployment.ID, err)
		return
	}

	// Local mode: use local stats map
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.FailureCount++
	stats.LastRequestTime = time.Now()

	var llmErr *llmerrors.LLMError
	if errors.As(err, &llmErr) {
		// Immediate cooldown: 429 Rate Limit
		if r.config.ImmediateCooldownOn429 && llmErr.StatusCode == 429 {
			stats.CooldownUntil = time.Now().Add(r.config.CooldownPeriod)
			return
		}

		// Immediate cooldown: Non-retryable errors (401, 404)
		if llmErr.StatusCode == 401 || llmErr.StatusCode == 404 {
			stats.CooldownUntil = time.Now().Add(r.config.CooldownPeriod)
			return
		}

		// Record high latency for timeout errors
		if llmErr.StatusCode == 408 || llmErr.StatusCode == 504 {
			r.appendToHistory(&stats.LatencyHistory, 1000000.0, stats.MaxLatencyListSize)
		}

		// Failure rate based cooldown
		if r.shouldCooldownByFailureRate(stats) {
			stats.CooldownUntil = time.Now().Add(r.config.CooldownPeriod)
		}
	}
}

// shouldCooldownByFailureRate checks if deployment should enter cooldown based on failure rate.
// Returns true if failure rate exceeds threshold AND minimum request count is met.
func (r *BaseRouter) shouldCooldownByFailureRate(stats *statsEntry) bool {
	total := stats.SuccessCount + stats.FailureCount
	if total < int64(r.config.MinRequestsForThreshold) {
		return false // Not enough requests to determine failure rate
	}

	failureRate := float64(stats.FailureCount) / float64(total)
	return failureRate > r.config.FailureThresholdPercent
}

func (r *BaseRouter) getHealthyDeployments(model string) []*ExtendedDeployment {
	deps, ok := r.deployments[model]
	if !ok || len(deps) == 0 {
		return nil
	}

	now := time.Now()
	healthy := make([]*ExtendedDeployment, 0, len(deps))
	for _, d := range deps {
		stats := r.stats[d.ID]
		if stats == nil || now.After(stats.CooldownUntil) {
			healthy = append(healthy, d)
		}
	}
	return healthy
}

func (r *BaseRouter) filterByTags(deployments []*ExtendedDeployment, tags []string) []*ExtendedDeployment {
	if len(tags) == 0 {
		defaults := make([]*ExtendedDeployment, 0)
		for _, d := range deployments {
			if containsTag(d.Config.Tags, "default") {
				defaults = append(defaults, d)
			}
		}
		if len(defaults) > 0 {
			return defaults
		}
		return deployments
	}

	matched := make([]*ExtendedDeployment, 0)
	defaults := make([]*ExtendedDeployment, 0)

	for _, d := range deployments {
		if len(d.Config.Tags) == 0 {
			continue
		}
		if hasMatchingTag(d.Config.Tags, tags) {
			matched = append(matched, d)
		}
		if containsTag(d.Config.Tags, "default") {
			defaults = append(defaults, d)
		}
	}

	if len(matched) > 0 {
		return matched
	}
	if len(defaults) > 0 {
		return defaults
	}
	return nil
}

func (r *BaseRouter) filterByTPMRPM(deployments []*ExtendedDeployment, inputTokens int) []*ExtendedDeployment {
	filtered := make([]*ExtendedDeployment, 0, len(deployments))

	for _, d := range deployments {
		stats := r.stats[d.ID]
		if stats == nil {
			filtered = append(filtered, d)
			continue
		}

		if d.Config.TPMLimit > 0 && stats.CurrentMinuteTPM+int64(inputTokens) > d.Config.TPMLimit {
			continue
		}

		if d.Config.RPMLimit > 0 && stats.CurrentMinuteRPM+1 > d.Config.RPMLimit {
			continue
		}

		filtered = append(filtered, d)
	}

	return filtered
}

func (r *BaseRouter) getOrCreateStats(deploymentID string) *statsEntry {
	stats, ok := r.stats[deploymentID]
	if !ok {
		stats = &statsEntry{
			MaxLatencyListSize: r.config.MaxLatencyListSize,
			LatencyHistory:     make([]float64, 0, r.config.MaxLatencyListSize),
			TTFTHistory:        make([]float64, 0, r.config.MaxLatencyListSize),
		}
		r.stats[deploymentID] = stats
	}
	return stats
}

func (r *BaseRouter) appendToHistory(history *[]float64, value float64, maxSize int) {
	if maxSize <= 0 {
		maxSize = 10
	}
	if len(*history) < maxSize {
		*history = append(*history, value)
	} else {
		copy((*history)[0:], (*history)[1:])
		(*history)[len(*history)-1] = value
	}
}

func (r *BaseRouter) updateUsageStats(stats *statsEntry, tokens int) {
	currentMinute := time.Now().Format("2006-01-02-15-04")

	if stats.CurrentMinuteKey != currentMinute {
		stats.CurrentMinuteKey = currentMinute
		stats.CurrentMinuteTPM = 0
		stats.CurrentMinuteRPM = 0
	}

	stats.CurrentMinuteTPM += int64(tokens)
	stats.CurrentMinuteRPM++
}

func calculateAverageLatency(history []float64) float64 {
	if len(history) == 0 {
		return 0
	}
	var sum float64
	for _, v := range history {
		sum += v
	}
	return sum / float64(len(history))
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func hasMatchingTag(deploymentTags, requestTags []string) bool {
	for _, dt := range deploymentTags {
		for _, rt := range requestTags {
			if dt == rt {
				return true
			}
		}
	}
	return false
}

// Pick implements basic random selection (used as fallback).
func (r *BaseRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext implements basic random selection with context.
func (r *BaseRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	r.mu.RLock()
	healthy := r.getHealthyDeployments(reqCtx.Model)
	if len(healthy) == 0 {
		r.mu.RUnlock()
		return nil, ErrNoAvailableDeployment
	}

	if r.config.EnableTagFiltering && len(reqCtx.Tags) > 0 {
		healthy = r.filterByTags(healthy, reqCtx.Tags)
		if len(healthy) == 0 {
			r.mu.RUnlock()
			return nil, ErrNoDeploymentsWithTag
		}
	}

	n := len(healthy)
	r.mu.RUnlock()

	return healthy[r.randIntn(n)].Deployment, nil
}
