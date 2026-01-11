package routers

import (
	"context"
	"fmt"
	"sort"

	"github.com/blueberrycongee/llmux/pkg/pricing"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// UnknownModelCost is the default cost used when no pricing information is available.
// It is set to 1.0 USD per token, which is significantly higher than the most expensive
// models in 2025 (e.g., o1-pro output is ~$0.0006/token).
// This ensures that unconfigured models are deprioritized by the CostRouter.
const UnknownModelCost = 1.0

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
	r := &CostRouter{
		BaseRouter: NewBaseRouter(config),
		registry:   pricing.NewRegistry(),
	}
	if config.PricingFile != "" {
		if err := r.registry.Load(config.PricingFile); err != nil {
			panic(fmt.Errorf("failed to load pricing file %s: %w", config.PricingFile, err))
		}
	}
	return r
}

// newCostRouterWithStore creates a new cost router with optional distributed StatsStore.
func newCostRouterWithStore(config router.Config, store router.StatsStore) *CostRouter {
	config.Strategy = router.StrategyLowestCost
	var base *BaseRouter
	if store != nil {
		base = NewBaseRouterWithStore(config, store)
	} else {
		base = NewBaseRouter(config)
	}
	r := &CostRouter{
		BaseRouter: base,
		registry:   pricing.NewRegistry(),
	}
	if config.PricingFile != "" {
		if err := r.registry.Load(config.PricingFile); err != nil {
			panic(fmt.Errorf("failed to load pricing file %s: %w", config.PricingFile, err))
		}
	}
	return r
}

// Pick selects the deployment with lowest cost.
func (r *CostRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest cost per token.
func (r *CostRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
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
			inputCost = UnknownModelCost
		}
		if outputCost == 0 {
			outputCost = UnknownModelCost
		}

		totalCost := inputCost + outputCost

		candidates = append(candidates, deploymentCost{
			deployment: d,
			cost:       totalCost,
		})
	}

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
