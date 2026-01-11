package routers

import (
	"context"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// TagBasedRouter filters deployments based on request tags before applying
// random selection.
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

// NewTagBasedRouter creates a new tag-based router with default config.
func NewTagBasedRouter(cooldownPeriod ...interface{}) *TagBasedRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyTagBased
	config.EnableTagFiltering = true
	return &TagBasedRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// NewTagBasedRouterWithConfig creates a new tag-based router with custom config.
func NewTagBasedRouterWithConfig(config router.Config) *TagBasedRouter {
	config.Strategy = router.StrategyTagBased
	config.EnableTagFiltering = true
	return &TagBasedRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// newTagBasedRouterWithStore creates a new tag-based router with optional distributed StatsStore.
func newTagBasedRouterWithStore(config router.Config, store router.StatsStore) *TagBasedRouter {
	config.Strategy = router.StrategyTagBased
	config.EnableTagFiltering = true
	var base *BaseRouter
	if store != nil {
		base = NewBaseRouterWithStore(config, store)
	} else {
		base = NewBaseRouter(config)
	}
	return &TagBasedRouter{BaseRouter: base}
}

// Pick selects a random deployment (tag filtering requires context).
func (r *TagBasedRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext filters deployments by tags and selects randomly.
func (r *TagBasedRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	deployments := r.snapshotDeployments(reqCtx.Model)
	if len(deployments) == 0 {
		return nil, ErrNoAvailableDeployment
	}
	statsByID := r.statsSnapshot(ctx, deployments)
	healthy := r.getHealthyDeployments(deployments, statsByID)
	if len(healthy) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	// Apply tag filtering
	filtered := r.filterByTags(healthy, reqCtx.Tags)
	if len(filtered) == 0 {
		return nil, ErrNoDeploymentsWithTag
	}

	if reqCtx.EstimatedInputTokens > 0 {
		filtered = r.filterByTPMRPM(filtered, statsByID, reqCtx.EstimatedInputTokens)
		if len(filtered) == 0 {
			return nil, ErrNoAvailableDeployment
		}
	}

	return filtered[r.randIntn(len(filtered))].Deployment, nil
}
