package router

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// LowestTPMRPMRouter selects the deployment with lowest TPM/RPM usage.
// This strategy helps stay within rate limits by distributing requests
// to deployments with the most available capacity.
//
// TPM (Tokens Per Minute) and RPM (Requests Per Minute) are tracked per deployment
// and reset at the start of each minute.
type LowestTPMRPMRouter struct {
	*BaseRouter
}

// NewLowestTPMRPMRouter creates a new lowest TPM/RPM router.
func NewLowestTPMRPMRouter(config RouterConfig) *LowestTPMRPMRouter {
	config.Strategy = StrategyLowestTPMRPM
	return &LowestTPMRPMRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects the deployment with lowest TPM usage.
func (r *LowestTPMRPMRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext selects the deployment with lowest TPM/RPM usage.
func (r *LowestTPMRPMRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
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

	// Filter by TPM/RPM limits and find lowest usage
	var bestDeployment *ExtendedDeployment
	lowestTPM := int64(-1)

	// Shuffle first to randomize selection among equal candidates
	shuffled := make([]*ExtendedDeployment, len(healthy))
	copy(shuffled, healthy)
	r.rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	for _, d := range shuffled {
		stats := r.stats[d.ID]
		var currentTPM, currentRPM int64

		if stats != nil {
			currentTPM = stats.CurrentMinuteTPM
			currentRPM = stats.CurrentMinuteRPM
		}

		// Check if adding this request would exceed limits
		estimatedTokens := int64(reqCtx.EstimatedInputTokens)
		if estimatedTokens == 0 {
			estimatedTokens = 100 // Default estimate
		}

		// Skip if would exceed TPM limit
		if d.Config.TPMLimit > 0 && currentTPM+estimatedTokens > d.Config.TPMLimit {
			continue
		}

		// Skip if would exceed RPM limit
		if d.Config.RPMLimit > 0 && currentRPM+1 >= d.Config.RPMLimit {
			continue
		}

		// Select deployment with lowest TPM
		if lowestTPM < 0 || currentTPM < lowestTPM {
			lowestTPM = currentTPM
			bestDeployment = d
		}
	}

	if bestDeployment == nil {
		return nil, ErrNoAvailableDeployment
	}

	return bestDeployment.Deployment, nil
}
