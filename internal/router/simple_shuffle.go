package router

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// SimpleShuffleRouter implements random selection with optional weighted picking.
// Weights can be specified via weight, rpm, or tpm parameters in deployment config.
type SimpleShuffleRouter struct {
	*BaseRouter
}

// NewSimpleShuffleRouter creates a new simple shuffle router.
func NewSimpleShuffleRouter(config RouterConfig) *SimpleShuffleRouter {
	config.Strategy = StrategySimpleShuffle
	return &SimpleShuffleRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects a random deployment, optionally weighted.
func (r *SimpleShuffleRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext selects a deployment using weighted random selection if weights are configured.
func (r *SimpleShuffleRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
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

	// Try weighted selection by weight, rpm, or tpm (in that order)
	if deployment := r.weightedPick(healthy, "weight"); deployment != nil {
		return deployment, nil
	}
	if deployment := r.weightedPick(healthy, "rpm"); deployment != nil {
		return deployment, nil
	}
	if deployment := r.weightedPick(healthy, "tpm"); deployment != nil {
		return deployment, nil
	}

	// Fall back to uniform random selection
	return healthy[r.rng.Intn(len(healthy))].Deployment, nil
}

// weightedPick performs weighted random selection based on the specified weight type.
// Returns nil if no weights are configured for the given type.
func (r *SimpleShuffleRouter) weightedPick(deployments []*ExtendedDeployment, weightType string) *provider.Deployment {
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

	// Calculate total weight
	var totalWeight float64
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight == 0 {
		return nil
	}

	// Normalize weights
	for i := range weights {
		weights[i] /= totalWeight
	}

	// Weighted random selection
	randVal := r.rng.Float64()
	var cumulative float64
	for i, w := range weights {
		cumulative += w
		if randVal <= cumulative {
			return deployments[i].Deployment
		}
	}

	// Fallback to last deployment (shouldn't happen due to floating point)
	return deployments[len(deployments)-1].Deployment
}
