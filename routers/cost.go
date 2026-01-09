package routers

import (
	"context"
	"sort"

	"github.com/blueberrycongee/llmux/pkg/pricing"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// DefaultCostPerToken is used when no cost is configured for a deployment.
// Set high to deprioritize deployments without cost configuration.
const DefaultCostPerToken = 5.0

// CostRouter selects the deployment with lowest cost per token.
// Cost is calculated as: input_cost_per_token + output_cost_per_token
//
// This strategy is useful for cost optimization when you have multiple
// deployments with different pricing (e.g., different regions, providers).
type CostRouter struct {
	*BaseRouter
	registry *pricing.Registry
}

// NewCostRouter creates a new cost router with default config.
func NewCostRouter(cooldownPeriod ...interface{}) *CostRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyLowestCost
	return &CostRouter{
		BaseRouter: NewBaseRouter(config),
		registry:   pricing.NewRegistry(),
	}
}

// NewCostRouterWithConfig creates a new cost router with custom config.
func NewCostRouterWithConfig(config router.Config) *CostRouter {
	config.Strategy = router.StrategyLowestCost
	return &CostRouter{
		BaseRouter: NewBaseRouter(config),
		registry:   pricing.NewRegistry(),
	}
}

// Pick selects the deployment with lowest cost.
func (r *CostRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest cost per token.
func (r *CostRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
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

	type deploymentCost struct {
		deployment *ExtendedDeployment
		cost       float64
	}

	candidates := make([]deploymentCost, 0, len(healthy))

	for _, d := range healthy {
		inputCost := d.Config.InputCostPerToken
		outputCost := d.Config.OutputCostPerToken

		// If cost is not configured, try to fetch from registry
		if inputCost == 0 && outputCost == 0 {
			if price, ok := r.registry.GetPrice(d.ModelName, d.ProviderName); ok {
				inputCost = price.InputCostPerToken
				outputCost = price.OutputCostPerToken
			}
		}

		if inputCost == 0 {
			inputCost = DefaultCostPerToken
		}
		if outputCost == 0 {
			outputCost = DefaultCostPerToken
		}

		totalCost := inputCost + outputCost

		candidates = append(candidates, deploymentCost{
			deployment: d,
			cost:       totalCost,
		})
	}
	r.mu.RUnlock()

	// Shuffle first to randomize order for equal costs
	r.randShuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Sort by cost (stable sort preserves random order for equal values)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].cost < candidates[j].cost
	})

	return candidates[0].deployment.Deployment, nil
}
