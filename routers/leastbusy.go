package routers

import (
	"context"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
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

// NewLeastBusyRouter creates a new least busy router with default config.
func NewLeastBusyRouter(cooldownPeriod ...interface{}) *LeastBusyRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyLeastBusy
	return &LeastBusyRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// NewLeastBusyRouterWithConfig creates a new least busy router with custom config.
func NewLeastBusyRouterWithConfig(config router.Config) *LeastBusyRouter {
	config.Strategy = router.StrategyLeastBusy
	return &LeastBusyRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// newLeastBusyRouterWithStore creates a new least busy router with optional distributed StatsStore.
func newLeastBusyRouterWithStore(config router.Config, store router.StatsStore) *LeastBusyRouter {
	config.Strategy = router.StrategyLeastBusy
	var base *BaseRouter
	if store != nil {
		base = NewBaseRouterWithStore(config, store)
	} else {
		base = NewBaseRouter(config)
	}
	return &LeastBusyRouter{BaseRouter: base}
}

// Pick selects the deployment with fewest active requests.
func (r *LeastBusyRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects the deployment with fewest active requests.
func (r *LeastBusyRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	deployments := r.snapshotDeployments(reqCtx.Model)
	if len(deployments) == 0 {
		return nil, ErrNoAvailableDeployment
	}
	statsByID := r.statsSnapshot(ctx, deployments)
	healthy := r.getHealthyDeployments(deployments, statsByID)
	if len(healthy) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	if r.config.EnableTagFiltering && len(reqCtx.Tags) > 0 {
		healthy = r.filterByTags(healthy, reqCtx.Tags)
		if len(healthy) == 0 {
			return nil, ErrNoDeploymentsWithTag
		}
	}

	if reqCtx.EstimatedInputTokens > 0 {
		healthy = r.filterByTPMRPM(healthy, statsByID, reqCtx.EstimatedInputTokens)
		if len(healthy) == 0 {
			return nil, ErrNoAvailableDeployment
		}
	}

	healthy = r.filterByDefaultProvider(healthy)
	type deploymentInfo struct {
		deployment     *ExtendedDeployment
		activeRequests int64
	}
	candidates := make([]deploymentInfo, len(healthy))
	for i, d := range healthy {
		var activeRequests int64
		if stats := statsByID[d.ID]; stats != nil {
			activeRequests = stats.ActiveRequests
		}
		candidates[i] = deploymentInfo{deployment: d, activeRequests: activeRequests}
	}

	// Shuffle first to randomize selection among equal candidates
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
		return candidates[r.randIntn(len(candidates))].deployment.Deployment, nil
	}

	return minDeployment.Deployment, nil
}
