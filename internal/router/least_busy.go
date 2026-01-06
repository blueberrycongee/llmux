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

	// Find deployment with minimum active requests
	var minDeployment *ExtendedDeployment
	minRequests := int64(-1)

	// Shuffle first to randomize selection among equal candidates
	shuffled := make([]*ExtendedDeployment, len(healthy))
	copy(shuffled, healthy)
	r.rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	for _, d := range shuffled {
		stats := r.stats[d.ID]
		var activeRequests int64
		if stats != nil {
			activeRequests = stats.ActiveRequests
		}

		// Initialize or update minimum
		if minRequests < 0 || activeRequests < minRequests {
			minRequests = activeRequests
			minDeployment = d
		}
	}

	if minDeployment == nil {
		// Fallback to random selection (shouldn't happen)
		return healthy[r.rng.Intn(len(healthy))].Deployment, nil
	}

	return minDeployment.Deployment, nil
}
