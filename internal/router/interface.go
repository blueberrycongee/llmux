// Package router provides request routing and load balancing for LLM deployments.
// It supports multiple strategies including simple shuffle, lowest latency, and least busy.
package router

import (
	"context"
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
)

// Router selects the best deployment for a given request.
// It tracks deployment health and performance metrics for intelligent routing.
type Router interface {
	// Pick selects the best available deployment for the given model.
	// Returns ErrNoAvailableDeployment if all deployments are unavailable.
	Pick(ctx context.Context, model string) (*provider.Deployment, error)

	// ReportSuccess records a successful request to update routing metrics.
	ReportSuccess(deployment *provider.Deployment, latency time.Duration)

	// ReportFailure records a failed request and potentially triggers cooldown.
	ReportFailure(deployment *provider.Deployment, err error)

	// IsCircuitOpen checks if the circuit breaker is open for a deployment.
	IsCircuitOpen(deployment *provider.Deployment) bool

	// AddDeployment registers a new deployment with the router.
	AddDeployment(deployment *provider.Deployment)

	// RemoveDeployment removes a deployment from the router.
	RemoveDeployment(deploymentID string)

	// GetDeployments returns all deployments for a model.
	GetDeployments(model string) []*provider.Deployment
}

// Strategy defines the routing strategy type.
type Strategy string

const (
	// StrategySimpleShuffle randomly selects from available deployments.
	StrategySimpleShuffle Strategy = "simple-shuffle"

	// StrategyLowestLatency selects the deployment with lowest average latency.
	StrategyLowestLatency Strategy = "lowest-latency"

	// StrategyLeastBusy selects the deployment with fewest active requests.
	StrategyLeastBusy Strategy = "least-busy"
)

// DeploymentStats tracks performance metrics for a deployment.
type DeploymentStats struct {
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	ActiveRequests  int64
	AvgLatencyMs    float64
	LastRequestTime time.Time
	CooldownUntil   time.Time
}

// RouterConfig contains router configuration options.
type RouterConfig struct {
	Strategy       Strategy
	CooldownPeriod time.Duration
	LatencyBuffer  float64 // For lowest-latency: select randomly within this % of lowest
}

// DefaultRouterConfig returns sensible default router configuration.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		Strategy:       StrategySimpleShuffle,
		CooldownPeriod: 60 * time.Second,
		LatencyBuffer:  0.1, // 10% buffer
	}
}
