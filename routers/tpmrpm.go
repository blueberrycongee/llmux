package routers

import (
	"context"

	internalRouter "github.com/blueberrycongee/llmux/internal/router"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// TPMRPMRouter selects the deployment with lowest TPM/RPM usage.
// This strategy helps stay within rate limits by distributing requests
// to deployments with the most available capacity.
//
// TPM (Tokens Per Minute) and RPM (Requests Per Minute) are tracked per deployment
// and reset at the start of each minute.
type TPMRPMRouter struct {
	*BaseRouter
}

// NewTPMRPMRouter creates a new TPM/RPM router with default config.
func NewTPMRPMRouter(cooldownPeriod ...interface{}) *TPMRPMRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyLowestTPMRPM
	return &TPMRPMRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// NewTPMRPMRouterWithConfig creates a new TPM/RPM router with custom config.
func NewTPMRPMRouterWithConfig(config router.Config) *TPMRPMRouter {
	config.Strategy = router.StrategyLowestTPMRPM
	return &TPMRPMRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// newTPMRPMRouterWithStore creates a new TPM/RPM router with optional distributed StatsStore.
func newTPMRPMRouterWithStore(config router.Config, store internalRouter.StatsStore) *TPMRPMRouter {
	config.Strategy = router.StrategyLowestTPMRPM
	var base *BaseRouter
	if store != nil {
		base = NewBaseRouterWithStore(config, store)
	} else {
		base = NewBaseRouter(config)
	}
	return &TPMRPMRouter{BaseRouter: base}
}

// Pick selects the deployment with lowest TPM usage.
func (r *TPMRPMRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest TPM/RPM usage.
func (r *TPMRPMRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
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

	type deploymentInfo struct {
		deployment *ExtendedDeployment
		currentTPM int64
		currentRPM int64
	}
	candidates := make([]deploymentInfo, len(healthy))
	for i, d := range healthy {
		var currentTPM, currentRPM int64
		if stats := r.stats[d.ID]; stats != nil {
			currentTPM = stats.CurrentMinuteTPM
			currentRPM = stats.CurrentMinuteRPM
		}
		candidates[i] = deploymentInfo{deployment: d, currentTPM: currentTPM, currentRPM: currentRPM}
	}
	r.mu.RUnlock()

	// Shuffle first to randomize selection among equal candidates
	r.randShuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Filter by TPM/RPM limits and find lowest usage
	var bestDeployment *ExtendedDeployment
	lowestTPM := int64(-1)

	for _, c := range candidates {
		estimatedTokens := int64(reqCtx.EstimatedInputTokens)
		if estimatedTokens == 0 {
			estimatedTokens = 100 // Default estimate
		}

		// Skip if would exceed TPM limit
		if c.deployment.Config.TPMLimit > 0 && c.currentTPM+estimatedTokens > c.deployment.Config.TPMLimit {
			continue
		}

		// Skip if would exceed RPM limit
		if c.deployment.Config.RPMLimit > 0 && c.currentRPM+1 >= c.deployment.Config.RPMLimit {
			continue
		}

		// Select deployment with lowest TPM
		if lowestTPM < 0 || c.currentTPM < lowestTPM {
			lowestTPM = c.currentTPM
			bestDeployment = c.deployment
		}
	}

	if bestDeployment == nil {
		return nil, ErrNoAvailableDeployment
	}

	return bestDeployment.Deployment, nil
}
