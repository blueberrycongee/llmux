package llmux

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// ErrNoAvailableDeployment is returned when no deployment is available for routing.
var ErrNoAvailableDeployment = errors.New("no available deployment")

// simpleRouter is a basic router implementation for library mode.
// It supports simple shuffle strategy with cooldown.
type simpleRouter struct {
	deployments    map[string][]*provider.Deployment // model -> deployments
	stats          map[string]*router.DeploymentStats
	cooldownPeriod time.Duration
	strategy       router.Strategy
	mu             sync.RWMutex
}

// newSimpleRouter creates a new simple router.
func newSimpleRouter(cooldownPeriod time.Duration, strategy router.Strategy) *simpleRouter {
	return &simpleRouter{
		deployments:    make(map[string][]*provider.Deployment),
		stats:          make(map[string]*router.DeploymentStats),
		cooldownPeriod: cooldownPeriod,
		strategy:       strategy,
	}
}

func (r *simpleRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	deployments, ok := r.deployments[model]
	if !ok || len(deployments) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	// Filter available deployments (not in cooldown)
	now := time.Now()
	var available []*provider.Deployment
	for _, d := range deployments {
		stats := r.stats[d.ID]
		if stats == nil || now.After(stats.CooldownUntil) {
			available = append(available, d)
		}
	}

	if len(available) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	// Simple shuffle: random selection
	idx := rand.Intn(len(available))
	return available[idx], nil
}

func (r *simpleRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	return r.Pick(ctx, reqCtx.Model)
}

func (r *simpleRouter) ReportSuccess(deployment *provider.Deployment, metrics *router.ResponseMetrics) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.SuccessCount++
	stats.LastRequestTime = time.Now()

	if metrics != nil {
		// Update latency history
		latencyMs := float64(metrics.Latency.Milliseconds())
		stats.LatencyHistory = append(stats.LatencyHistory, latencyMs)
		if len(stats.LatencyHistory) > 10 {
			stats.LatencyHistory = stats.LatencyHistory[1:]
		}

		// Calculate average latency
		var sum float64
		for _, l := range stats.LatencyHistory {
			sum += l
		}
		stats.AvgLatencyMs = sum / float64(len(stats.LatencyHistory))

		// Update TTFT if available
		if metrics.TimeToFirstToken > 0 {
			ttftMs := float64(metrics.TimeToFirstToken.Milliseconds())
			stats.TTFTHistory = append(stats.TTFTHistory, ttftMs)
			if len(stats.TTFTHistory) > 10 {
				stats.TTFTHistory = stats.TTFTHistory[1:]
			}
			var ttftSum float64
			for _, t := range stats.TTFTHistory {
				ttftSum += t
			}
			stats.AvgTTFTMs = ttftSum / float64(len(stats.TTFTHistory))
		}
	}
}

func (r *simpleRouter) ReportFailure(deployment *provider.Deployment, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.FailureCount++
	stats.LastRequestTime = time.Now()

	// Apply cooldown
	stats.CooldownUntil = time.Now().Add(r.cooldownPeriod)
}

func (r *simpleRouter) ReportRequestStart(deployment *provider.Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.ActiveRequests++
}

func (r *simpleRouter) ReportRequestEnd(deployment *provider.Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	if stats.ActiveRequests > 0 {
		stats.ActiveRequests--
	}
}

func (r *simpleRouter) IsCircuitOpen(deployment *provider.Deployment) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := r.stats[deployment.ID]
	if stats == nil {
		return false
	}
	return time.Now().Before(stats.CooldownUntil)
}

func (r *simpleRouter) AddDeployment(deployment *provider.Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deployments[deployment.ModelName] = append(r.deployments[deployment.ModelName], deployment)
	r.stats[deployment.ID] = &router.DeploymentStats{
		MaxLatencyListSize: 10,
	}
}

func (r *simpleRouter) AddDeploymentWithConfig(deployment *provider.Deployment, config router.DeploymentConfig) {
	r.AddDeployment(deployment)
}

func (r *simpleRouter) RemoveDeployment(deploymentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find and remove deployment
	for model, deployments := range r.deployments {
		var remaining []*provider.Deployment
		for _, d := range deployments {
			if d.ID != deploymentID {
				remaining = append(remaining, d)
			}
		}
		if len(remaining) == 0 {
			delete(r.deployments, model)
		} else {
			r.deployments[model] = remaining
		}
	}

	delete(r.stats, deploymentID)
}

func (r *simpleRouter) GetDeployments(model string) []*provider.Deployment {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.deployments[model]
}

func (r *simpleRouter) GetStats(deploymentID string) *router.DeploymentStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if stats, ok := r.stats[deploymentID]; ok {
		// Return a copy
		statsCopy := *stats
		return &statsCopy
	}
	return nil
}

func (r *simpleRouter) GetStrategy() router.Strategy {
	return r.strategy
}

func (r *simpleRouter) getOrCreateStats(deploymentID string) *router.DeploymentStats {
	stats, ok := r.stats[deploymentID]
	if !ok {
		stats = &router.DeploymentStats{
			MaxLatencyListSize: 10,
		}
		r.stats[deploymentID] = stats
	}
	return stats
}
