package routers

import (
	"context"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// ShuffleRouter implements random selection with optional weighted picking.
// Weights can be specified via weight, rpm, or tpm parameters in deployment config.
type ShuffleRouter struct {
	*BaseRouter
}

// NewShuffleRouter creates a new shuffle router with default config.
func NewShuffleRouter(cooldownPeriod ...interface{}) *ShuffleRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategySimpleShuffle
	return &ShuffleRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// NewShuffleRouterWithConfig creates a new shuffle router with custom config.
func NewShuffleRouterWithConfig(config router.Config) *ShuffleRouter {
	config.Strategy = router.StrategySimpleShuffle
	return &ShuffleRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// newShuffleRouterWithStore creates a new shuffle router with optional distributed StatsStore.
func newShuffleRouterWithStore(config router.Config, store router.StatsStore) *ShuffleRouter {
	config.Strategy = router.StrategySimpleShuffle
	var base *BaseRouter
	if store != nil {
		base = NewBaseRouterWithStore(config, store)
	} else {
		base = NewBaseRouter(config)
	}
	return &ShuffleRouter{BaseRouter: base}
}

// Pick selects a random deployment, optionally weighted.
func (r *ShuffleRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects a deployment using weighted random selection if weights are configured.
func (r *ShuffleRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
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
	healthyCopy := make([]*ExtendedDeployment, len(healthy))
	copy(healthyCopy, healthy)

	// Try weighted selection by weight, rpm, or tpm (in that order)
	if deployment := r.weightedPick(healthyCopy, "weight"); deployment != nil {
		return deployment, nil
	}
	if deployment := r.weightedPick(healthyCopy, "rpm"); deployment != nil {
		return deployment, nil
	}
	if deployment := r.weightedPick(healthyCopy, "tpm"); deployment != nil {
		return deployment, nil
	}

	// Fall back to uniform random selection
	return healthyCopy[r.randIntn(len(healthyCopy))].Deployment, nil
}

// weightedPick performs weighted random selection based on the specified weight type.
func (r *ShuffleRouter) weightedPick(deployments []*ExtendedDeployment, weightType string) *provider.Deployment {
	weights := make([]float64, len(deployments))
	hasWeights := false

	for i, d := range deployments {
		var weight float64
		switch weightType {
		case "weight":
			weight = d.Config.Weight
		case "rpm":
			weight = float64(d.Config.RPMLimit)
		case "tpm":
			weight = float64(d.Config.TPMLimit)
		}
		weights[i] = weight
		if weight > 0 {
			hasWeights = true
		}
	}

	if !hasWeights {
		return nil
	}

	var totalWeight float64
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight == 0 {
		return nil
	}

	for i := range weights {
		weights[i] /= totalWeight
	}

	randVal := r.randFloat64()
	var cumulative float64
	for i, w := range weights {
		cumulative += w
		if randVal <= cumulative {
			return deployments[i].Deployment
		}
	}

	return deployments[len(deployments)-1].Deployment
}
