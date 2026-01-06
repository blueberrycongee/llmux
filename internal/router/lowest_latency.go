package router

import (
	"context"
	"sort"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// LowestLatencyRouter selects deployments based on response latency.
// For streaming requests, it uses Time To First Token (TTFT) instead of total latency.
// A configurable buffer allows random selection among deployments within X% of the lowest latency.
type LowestLatencyRouter struct {
	*BaseRouter
}

// NewLowestLatencyRouter creates a new lowest latency router.
func NewLowestLatencyRouter(config RouterConfig) *LowestLatencyRouter {
	config.Strategy = StrategyLowestLatency
	return &LowestLatencyRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects the deployment with lowest latency.
func (r *LowestLatencyRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest latency, considering streaming mode.
func (r *LowestLatencyRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthy := r.getHealthyDeployments(reqCtx.Model)
	if len(healthy) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	// Apply tag filtering if enabled
	if r.config.EnableTagFiltering && len(reqCtx.Tags) > 0 {
		healthy = r.filterByTags(healthy, reqCtx.Tags)
		if len(healthy) == 0 {
			return nil, ErrNoDeploymentsWithTag
		}
	}

	// Apply TPM/RPM filtering
	if reqCtx.EstimatedInputTokens > 0 {
		healthy = r.filterByTPMRPM(healthy, reqCtx.EstimatedInputTokens)
		if len(healthy) == 0 {
			return nil, ErrNoAvailableDeployment
		}
	}

	// Calculate latency for each deployment
	type deploymentLatency struct {
		deployment *ExtendedDeployment
		latency    float64
	}

	candidates := make([]deploymentLatency, 0, len(healthy))

	for _, d := range healthy {
		stats := r.stats[d.ID]
		var latency float64

		if stats == nil {
			// No stats yet, use 0 latency (prioritize untested deployments)
			latency = 0
		} else if reqCtx.IsStreaming && len(stats.TTFTHistory) > 0 {
			// Use TTFT for streaming requests
			latency = calculateAverageLatency(stats.TTFTHistory)
		} else if len(stats.LatencyHistory) > 0 {
			// Use regular latency
			latency = calculateAverageLatency(stats.LatencyHistory)
		} else {
			latency = 0
		}

		candidates = append(candidates, deploymentLatency{
			deployment: d,
			latency:    latency,
		})
	}

	// Shuffle first to randomize order for equal latencies
	r.rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Sort by latency (stable sort preserves random order for equal values)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].latency < candidates[j].latency
	})

	// Find lowest latency
	lowestLatency := candidates[0].latency

	// If lowest latency is 0, just pick randomly from all candidates
	if lowestLatency == 0 {
		return candidates[r.rng.Intn(len(candidates))].deployment.Deployment, nil
	}

	// Find all deployments within the buffer threshold
	buffer := r.config.LatencyBuffer * lowestLatency
	threshold := lowestLatency + buffer

	validCandidates := make([]deploymentLatency, 0)
	for _, c := range candidates {
		if c.latency <= threshold {
			validCandidates = append(validCandidates, c)
		}
	}

	// Random selection from valid candidates
	selected := validCandidates[r.rng.Intn(len(validCandidates))]
	return selected.deployment.Deployment, nil
}
