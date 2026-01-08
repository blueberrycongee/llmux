package router

import (
	"context"
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// SimpleRouter is a backward-compatible wrapper around SimpleShuffleRouter.
//
// Deprecated: Use NewSimpleShuffleRouter or New(config) instead.
type SimpleRouter struct {
	*SimpleShuffleRouter
}

// NewSimpleRouter creates a new simple router with the given cooldown period.
//
// Deprecated: Use NewSimpleShuffleRouter or New(config) instead.
func NewSimpleRouter(cooldownPeriod time.Duration) *SimpleRouter {
	config := RouterConfig{
		Strategy:           StrategySimpleShuffle,
		CooldownPeriod:     cooldownPeriod,
		MaxLatencyListSize: 10,
	}
	return &SimpleRouter{
		SimpleShuffleRouter: NewSimpleShuffleRouter(config),
	}
}

// Pick selects a random healthy deployment for the given model.
func (r *SimpleRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.SimpleShuffleRouter.Pick(ctx, model)
}

// PickWithContext selects a deployment using request context.
func (r *SimpleRouter) PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error) {
	return r.SimpleShuffleRouter.PickWithContext(ctx, reqCtx)
}

// ReportSuccess records a successful request with metrics.
func (r *SimpleRouter) ReportSuccess(deployment *provider.Deployment, metrics *ResponseMetrics) {
	r.SimpleShuffleRouter.ReportSuccess(deployment, metrics)
}

// ReportFailure records a failed request.
func (r *SimpleRouter) ReportFailure(deployment *provider.Deployment, err error) {
	r.SimpleShuffleRouter.ReportFailure(deployment, err)
}

// ReportRequestStart records when a request starts.
func (r *SimpleRouter) ReportRequestStart(deployment *provider.Deployment) {
	r.SimpleShuffleRouter.ReportRequestStart(deployment)
}

// ReportRequestEnd records when a request ends.
func (r *SimpleRouter) ReportRequestEnd(deployment *provider.Deployment) {
	r.SimpleShuffleRouter.ReportRequestEnd(deployment)
}

// IsCircuitOpen checks if the deployment is in cooldown.
func (r *SimpleRouter) IsCircuitOpen(deployment *provider.Deployment) bool {
	return r.SimpleShuffleRouter.IsCircuitOpen(deployment)
}

// AddDeployment registers a new deployment.
func (r *SimpleRouter) AddDeployment(deployment *provider.Deployment) {
	r.SimpleShuffleRouter.AddDeployment(deployment)
}

// AddDeploymentWithConfig registers a deployment with routing configuration.
func (r *SimpleRouter) AddDeploymentWithConfig(deployment *provider.Deployment, config DeploymentConfig) {
	r.SimpleShuffleRouter.AddDeploymentWithConfig(deployment, config)
}

// RemoveDeployment removes a deployment from the router.
func (r *SimpleRouter) RemoveDeployment(deploymentID string) {
	r.SimpleShuffleRouter.RemoveDeployment(deploymentID)
}

// GetDeployments returns all deployments for a model.
func (r *SimpleRouter) GetDeployments(model string) []*provider.Deployment {
	return r.SimpleShuffleRouter.GetDeployments(model)
}

// GetStats returns the current stats for a deployment.
func (r *SimpleRouter) GetStats(deploymentID string) *DeploymentStats {
	return r.SimpleShuffleRouter.GetStats(deploymentID)
}

// GetStrategy returns the current routing strategy.
func (r *SimpleRouter) GetStrategy() Strategy {
	return r.SimpleShuffleRouter.GetStrategy()
}
