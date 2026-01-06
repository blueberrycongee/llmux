// Package router provides request routing and load balancing for LLM deployments.
// It supports multiple strategies including simple shuffle, lowest latency, least busy,
// lowest TPM/RPM, lowest cost, and tag-based routing.
package router

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// Router selects the best deployment for a given request.
// It tracks deployment health and performance metrics for intelligent routing.
type Router interface {
	// Pick selects the best available deployment for the given model.
	// Returns ErrNoAvailableDeployment if all deployments are unavailable.
	Pick(ctx context.Context, model string) (*provider.Deployment, error)

	// PickWithContext selects the best deployment using request context.
	// This enables advanced routing strategies like tag-based and streaming-aware routing.
	PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error)

	// ReportSuccess records a successful request to update routing metrics.
	ReportSuccess(deployment *provider.Deployment, metrics *ResponseMetrics)

	// ReportFailure records a failed request and potentially triggers cooldown.
	ReportFailure(deployment *provider.Deployment, err error)

	// ReportRequestStart records when a request starts (for least-busy tracking).
	ReportRequestStart(deployment *provider.Deployment)

	// ReportRequestEnd records when a request ends (for least-busy tracking).
	ReportRequestEnd(deployment *provider.Deployment)

	// IsCircuitOpen checks if the circuit breaker is open for a deployment.
	IsCircuitOpen(deployment *provider.Deployment) bool

	// AddDeployment registers a new deployment with the router.
	AddDeployment(deployment *provider.Deployment)

	// AddDeploymentWithConfig registers a deployment with routing configuration.
	AddDeploymentWithConfig(deployment *provider.Deployment, config DeploymentConfig)

	// RemoveDeployment removes a deployment from the router.
	RemoveDeployment(deploymentID string)

	// GetDeployments returns all deployments for a model.
	GetDeployments(model string) []*provider.Deployment

	// GetStats returns the current stats for a deployment.
	GetStats(deploymentID string) *DeploymentStats

	// GetStrategy returns the current routing strategy.
	GetStrategy() Strategy
}
