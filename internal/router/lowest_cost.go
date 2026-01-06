package router

import (
	"context"
	"sort"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// DefaultCostPerToken is used when no cost is configured for a deployment.
// Set high to deprioritize deployments without cost configuration.
const DefaultCostPerToken = 5.0

// LowestCostRouter selects the deployment with lowest cost per token.
// Cost is calculated as: input_cost_per_token + output_cost_per_token
//
// This strategy is useful for cost optimization when you have multiple
// deployments with different pricing (e.g., different regions, providers).
type LowestCostRouter struct {
	*BaseRouter
}

// NewLowestCostRouter creates a new lowest cost router.
func NewLowestCostRouter(config RouterConfig) *LowestCostRouter {
	config.Strategy = StrategyLowestCost
	return &LowestCostRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects the deployment with lowest cost.
func (r *LowestCostRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest cost per token.
func (r *LowestCostRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
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

	// Calculate cost for each deployment
	type deploymentCost struct {
		deployment *ExtendedDeployment
		cost       float64
	}

	candidates := make([]deploymentCost, 0, len(healthy))

	for _, d := range healthy {
		inputCost := d.Config.InputCostPerToken
		outputCost := d.Config.OutputCostPerToken

		// Use default cost if not configured
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

	// Shuffle first to randomize order for equal costs
	r.rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Sort by cost (stable sort preserves random order for equal values)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].cost < candidates[j].cost
	})

	// Return the lowest cost deployment
	return candidates[0].deployment.Deployment, nil
}
