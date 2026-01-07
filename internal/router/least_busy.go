package router

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// LeastBusyRouter selects the deployment with the fewest active requests.
// This strategy helps distribute load evenly across deployments.
//
// Usage:
//   - Call ReportRequestStart() when a request begins
//   - Call ReportRequestEnd() when a request completes (success or failure)
type LeastBusyRouter struct {
	*BaseRouter
}

// NewLeastBusyRouter creates a new least busy router.
func NewLeastBusyRouter(config RouterConfig) *LeastBusyRouter {
	config.Strategy = StrategyLeastBusy
	return &LeastBusyRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects the deployment with fewest active requests.
func (r *LeastBusyRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext selects the deployment with fewest active requests.
func (r *LeastBusyRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
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

	// Apply TPM/RPM filtering
	if reqCtx.EstimatedInputTokens > 0 {
		healthy = r.filterByTPMRPM(healthy, reqCtx.EstimatedInputTokens)
		if len(healthy) == 0 {
			r.mu.RUnlock()
			return nil, ErrNoAvailableDeployment
		}
	}

	// Copy data needed for selection
	type deploymentInfo struct {
		deployment     *ExtendedDeployment
		activeRequests int64
	}
	candidates := make([]deploymentInfo, len(healthy))
	for i, d := range healthy {
		var activeRequests int64
		if stats := r.stats[d.ID]; stats != nil {
			activeRequests = stats.ActiveRequests
		}
		candidates[i] = deploymentInfo{deployment: d, activeRequests: activeRequests}
	}
	r.mu.RUnlock()

	// Shuffle first to randomize selection among equal candidates (thread-safe)
	r.randShuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Find deployment with minimum active requests
	var minDeployment *ExtendedDeployment
	minRequests := int64(-1)

	for _, c := range candidates {
		if minRequests < 0 || c.activeRequests < minRequests {
			minRequests = c.activeRequests
			minDeployment = c.deployment
		}
	}

	if minDeployment == nil {
		// Fallback to random selection (shouldn't happen, thread-safe)
		return candidates[r.randIntn(len(candidates))].deployment.Deployment, nil
	}

	return minDeployment.Deployment, nil
}
