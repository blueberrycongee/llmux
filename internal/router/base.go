package router

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// ErrNoAvailableDeployment is returned when no healthy deployment is available.
var ErrNoAvailableDeployment = errors.New("no available deployment for model")

// ErrNoDeploymentsWithTag is returned when no deployments match the requested tags.
var ErrNoDeploymentsWithTag = errors.New("no deployments match the requested tags")

// BaseRouter provides common functionality for all routing strategies.
// Specific strategies embed this and override the selection logic.
type BaseRouter struct {
	mu          sync.RWMutex
	rngMu       sync.Mutex // Separate mutex for rng (math/rand.Rand is not thread-safe)
	deployments map[string][]*ExtendedDeployment // model -> deployments
	stats       map[string]*DeploymentStats      // deploymentID -> stats
	config      RouterConfig
	rng         *rand.Rand
	strategy    Strategy
}

// NewBaseRouter creates a new base router with the given configuration.
func NewBaseRouter(config RouterConfig) *BaseRouter {
	return &BaseRouter{
		deployments: make(map[string][]*ExtendedDeployment),
		stats:       make(map[string]*DeploymentStats),
		config:      config,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		strategy:    config.Strategy,
	}
}

// GetStrategy returns the current routing strategy.
func (r *BaseRouter) GetStrategy() Strategy {
	return r.strategy
}

// randIntn returns a random int in [0, n) in a thread-safe manner.
func (r *BaseRouter) randIntn(n int) int {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng.Intn(n)
}

// randFloat64 returns a random float64 in [0.0, 1.0) in a thread-safe manner.
func (r *BaseRouter) randFloat64() float64 {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	return r.rng.Float64()
}

// randShuffle shuffles a slice in a thread-safe manner.
func (r *BaseRouter) randShuffle(n int, swap func(i, j int)) {
	r.rngMu.Lock()
	defer r.rngMu.Unlock()
	r.rng.Shuffle(n, swap)
}

// AddDeployment registers a new deployment with default configuration.
func (r *BaseRouter) AddDeployment(deployment *provider.Deployment) {
	r.AddDeploymentWithConfig(deployment, DeploymentConfig{})
}

// AddDeploymentWithConfig registers a deployment with routing configuration.
func (r *BaseRouter) AddDeploymentWithConfig(deployment *provider.Deployment, config DeploymentConfig) {
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
	r.stats[deployment.ID] = &DeploymentStats{
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
func (r *BaseRouter) GetStats(deploymentID string) *DeploymentStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if stats, ok := r.stats[deploymentID]; ok {
		// Return a copy to prevent external modification
		statsCopy := *stats
		return &statsCopy
	}
	return nil
}

// IsCircuitOpen checks if the deployment is in cooldown.
func (r *BaseRouter) IsCircuitOpen(deployment *provider.Deployment) bool {
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
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.ActiveRequests++
}

// ReportRequestEnd decrements the active request count.
func (r *BaseRouter) ReportRequestEnd(deployment *provider.Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	if stats.ActiveRequests > 0 {
		stats.ActiveRequests--
	}
}

// ReportSuccess records a successful request with metrics.
func (r *BaseRouter) ReportSuccess(deployment *provider.Deployment, metrics *ResponseMetrics) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.SuccessCount++
	stats.LastRequestTime = time.Now()

	// Update latency history
	latencyMs := float64(metrics.Latency.Milliseconds())
	r.appendToHistory(&stats.LatencyHistory, latencyMs, stats.MaxLatencyListSize)

	// Update TTFT history for streaming requests
	if metrics.TimeToFirstToken > 0 {
		ttftMs := float64(metrics.TimeToFirstToken.Milliseconds())
		r.appendToHistory(&stats.TTFTHistory, ttftMs, stats.MaxLatencyListSize)
	}

	// Update average latency (exponential moving average)
	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = latencyMs
	} else {
		stats.AvgLatencyMs = stats.AvgLatencyMs*0.9 + latencyMs*0.1
	}

	// Update TPM/RPM for current minute
	r.updateUsageStats(stats, metrics.TotalTokens)
}

// ReportFailure records a failed request and triggers cooldown if needed.
func (r *BaseRouter) ReportFailure(deployment *provider.Deployment, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.FailureCount++
	stats.LastRequestTime = time.Now()

	// Add penalty latency for timeout errors (helps lowest-latency routing avoid slow deployments)
	var llmErr *llmerrors.LLMError
	if errors.As(err, &llmErr) {
		if llmerrors.IsCooldownRequired(llmErr.StatusCode) {
			stats.CooldownUntil = time.Now().Add(r.config.CooldownPeriod)
		}
		// Add high latency penalty for timeouts
		if llmErr.StatusCode == 408 || llmErr.StatusCode == 504 {
			r.appendToHistory(&stats.LatencyHistory, 1000000.0, stats.MaxLatencyListSize) // 1000s penalty
		}
	}
}

// getHealthyDeployments returns deployments that are not in cooldown.
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

// filterByTags filters deployments based on request tags.
func (r *BaseRouter) filterByTags(deployments []*ExtendedDeployment, tags []string) []*ExtendedDeployment {
	if len(tags) == 0 {
		// For untagged requests, prefer deployments with "default" tag
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

// filterByTPMRPM filters out deployments that would exceed their TPM/RPM limits.
func (r *BaseRouter) filterByTPMRPM(deployments []*ExtendedDeployment, inputTokens int) []*ExtendedDeployment {
	filtered := make([]*ExtendedDeployment, 0, len(deployments))

	for _, d := range deployments {
		stats := r.stats[d.ID]
		if stats == nil {
			filtered = append(filtered, d)
			continue
		}

		// Check TPM limit
		if d.Config.TPMLimit > 0 && stats.CurrentMinuteTPM+int64(inputTokens) > d.Config.TPMLimit {
			continue
		}

		// Check RPM limit
		if d.Config.RPMLimit > 0 && stats.CurrentMinuteRPM+1 > d.Config.RPMLimit {
			continue
		}

		filtered = append(filtered, d)
	}

	return filtered
}

// getOrCreateStats returns existing stats or creates new ones.
func (r *BaseRouter) getOrCreateStats(deploymentID string) *DeploymentStats {
	stats, ok := r.stats[deploymentID]
	if !ok {
		stats = &DeploymentStats{
			MaxLatencyListSize: r.config.MaxLatencyListSize,
			LatencyHistory:     make([]float64, 0, r.config.MaxLatencyListSize),
			TTFTHistory:        make([]float64, 0, r.config.MaxLatencyListSize),
		}
		r.stats[deploymentID] = stats
	}
	return stats
}

// appendToHistory adds a value to a rolling history slice.
func (r *BaseRouter) appendToHistory(history *[]float64, value float64, maxSize int) {
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

// updateUsageStats updates TPM/RPM counters for the current minute.
func (r *BaseRouter) updateUsageStats(stats *DeploymentStats, tokens int) {
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

// calculateAverageLatency calculates the average of a latency history slice.
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

// containsTag checks if a tag list contains a specific tag.
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// hasMatchingTag checks if any deployment tag matches any request tag.
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
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext implements basic random selection with context.
func (r *BaseRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
	r.mu.RLock()
	healthy := r.getHealthyDeployments(reqCtx.Model)
	if len(healthy) == 0 {
		r.mu.RUnlock()
		return nil, ErrNoAvailableDeployment
	}

	// Apply tag filtering if enabled
	if r.config.EnableTagFiltering && len(reqCtx.Tags) > 0 {
		healthy = r.filterByTags(healthy, reqCtx.Tags)
		if len(healthy) == 0 {
			r.mu.RUnlock()
			return nil, ErrNoDeploymentsWithTag
		}
	}

	// Copy deployment pointer before releasing lock
	n := len(healthy)
	r.mu.RUnlock()

	// Random selection (thread-safe)
	return healthy[r.randIntn(n)].Deployment, nil
}
