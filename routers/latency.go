package routers

import (
	"context"
	"sort"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// LatencyRouter selects deployments based on response latency.
// For streaming requests, it uses Time To First Token (TTFT) instead of total latency.
// A configurable buffer allows random selection among deployments within X% of the lowest latency.
type LatencyRouter struct {
	*BaseRouter
}

// NewLatencyRouter creates a new latency router with default config.
func NewLatencyRouter(cooldownPeriod ...interface{}) *LatencyRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyLowestLatency
	return &LatencyRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// NewLatencyRouterWithConfig creates a new latency router with custom config.
func NewLatencyRouterWithConfig(config router.Config) *LatencyRouter {
	config.Strategy = router.StrategyLowestLatency
	return &LatencyRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects the deployment with lowest latency.
func (r *LatencyRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest latency, considering streaming mode.
func (r *LatencyRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
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

	if reqCtx.EstimatedInputTokens > 0 {
		healthy = r.filterByTPMRPM(healthy, reqCtx.EstimatedInputTokens)
		if len(healthy) == 0 {
			r.mu.RUnlock()
			return nil, ErrNoAvailableDeployment
		}
	}

	type deploymentLatency struct {
		deployment *ExtendedDeployment
		latency    float64
	}

	candidates := make([]deploymentLatency, 0, len(healthy))

	for _, d := range healthy {
		stats := r.stats[d.ID]
		var latency float64

		switch {
		case stats == nil:
			latency = 0
		case reqCtx.IsStreaming && len(stats.TTFTHistory) > 0:
			latency = calculateAverageLatency(stats.TTFTHistory)
		case len(stats.LatencyHistory) > 0:
			latency = calculateAverageLatency(stats.LatencyHistory)
		default:
			latency = 0
		}

		candidates = append(candidates, deploymentLatency{
			deployment: d,
			latency:    latency,
		})
	}

	latencyBuffer := r.config.LatencyBuffer
	r.mu.RUnlock()

	// Shuffle first to randomize order for equal latencies
	r.randShuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Sort by latency (stable sort preserves random order for equal values)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].latency < candidates[j].latency
	})

	lowestLatency := candidates[0].latency

	// If lowest latency is 0, just pick randomly from all candidates
	if lowestLatency == 0 {
		return candidates[r.randIntn(len(candidates))].deployment.Deployment, nil
	}

	// Find all deployments within the buffer threshold
	buffer := latencyBuffer * lowestLatency
	threshold := lowestLatency + buffer

	validCandidates := make([]deploymentLatency, 0)
	for _, c := range candidates {
		if c.latency <= threshold {
			validCandidates = append(validCandidates, c)
		}
	}

	selected := validCandidates[r.randIntn(len(validCandidates))]
	return selected.deployment.Deployment, nil
}
