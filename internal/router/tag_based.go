package router

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// TagBasedRouter filters deployments based on request tags before applying
// another routing strategy (defaults to random selection).
//
// Tag matching rules:
//   - If request has tags, only deployments with at least one matching tag are considered
//   - If no deployments match, deployments with "default" tag are used as fallback
//   - If request has no tags, deployments with "default" tag are preferred
//   - If no "default" deployments exist, all deployments are considered
//
// Example usage:
//
//	// Deployment config
//	config := DeploymentConfig{Tags: []string{"premium", "us-east"}}
//
//	// Request with tags
//	reqCtx := &RequestContext{Tags: []string{"premium"}}
type TagBasedRouter struct {
	*BaseRouter
}

// NewTagBasedRouter creates a new tag-based router.
func NewTagBasedRouter(config RouterConfig) *TagBasedRouter {
	config.Strategy = StrategyTagBased
	config.EnableTagFiltering = true // Always enable for this router
	return &TagBasedRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// Pick selects a random deployment (tag filtering requires context).
func (r *TagBasedRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &RequestContext{Model: model})
}

// PickWithContext filters deployments by tags and selects randomly.
func (r *TagBasedRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
	r.mu.RLock()
	healthy := r.getHealthyDeployments(reqCtx.Model)
	if len(healthy) == 0 {
		r.mu.RUnlock()
		return nil, ErrNoAvailableDeployment
	}

	// Apply tag filtering
	filtered := r.filterByTags(healthy, reqCtx.Tags)
	if len(filtered) == 0 {
		r.mu.RUnlock()
		return nil, ErrNoDeploymentsWithTag
	}

	// Apply TPM/RPM filtering
	if reqCtx.EstimatedInputTokens > 0 {
		filtered = r.filterByTPMRPM(filtered, reqCtx.EstimatedInputTokens)
		if len(filtered) == 0 {
			r.mu.RUnlock()
			return nil, ErrNoAvailableDeployment
		}
	}

	n := len(filtered)
	r.mu.RUnlock()

	// Random selection from filtered deployments (thread-safe)
	return filtered[r.randIntn(n)].Deployment, nil
}
