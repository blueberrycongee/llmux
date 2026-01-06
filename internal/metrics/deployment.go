// Package metrics provides deployment-related Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DeploymentState constants for deployment health status.
const (
	DeploymentStateHealthy       = 0 // All healthy
	DeploymentStatePartialFailed = 1 // Some failures
	DeploymentStateFailed        = 2 // Complete failure / cooldown
)

// =============================================================================
// Deployment Health Metrics
// =============================================================================

var (
	// DeploymentState tracks deployment health state.
	// 0 = healthy, 1 = partial failure, 2 = complete failure/cooldown
	DeploymentState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "deployment_state",
			Help:      "Deployment health state (0=healthy, 1=partial_failure, 2=failed)",
		},
		[]string{"deployment_id", "model", "model_group", "api_provider", "api_base"},
	)

	// DeploymentSuccessResponses counts successful responses per deployment.
	DeploymentSuccessResponses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deployment_success_responses",
			Help:      "Total successful responses per deployment",
		},
		[]string{"deployment_id", "model", "model_group", "api_provider", "api_base"},
	)

	// DeploymentFailureResponses counts failed responses per deployment.
	DeploymentFailureResponses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deployment_failure_responses",
			Help:      "Total failed responses per deployment",
		},
		[]string{"deployment_id", "model", "model_group", "api_provider", "api_base", "exception_status"},
	)

	// DeploymentTotalRequests counts total requests per deployment.
	DeploymentTotalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deployment_total_requests",
			Help:      "Total requests per deployment",
		},
		[]string{"deployment_id", "model", "model_group", "api_provider", "api_base"},
	)

	// DeploymentCooledDown counts cooldown events per deployment.
	DeploymentCooledDown = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deployment_cooled_down",
			Help:      "Number of times deployment entered cooldown",
		},
		[]string{"deployment_id", "model", "model_group", "api_provider", "api_base"},
	)

	// DeploymentLatencyPerOutputToken tracks latency per output token.
	DeploymentLatencyPerOutputToken = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "deployment_latency_per_output_token_seconds",
			Help:      "Latency per output token for deployment",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5},
		},
		[]string{"deployment_id", "model", "model_group", "api_provider"},
	)
)

// =============================================================================
// Fallback Metrics
// =============================================================================

var (
	// FallbackSuccessful counts successful fallback attempts.
	FallbackSuccessful = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "fallback_successful",
			Help:      "Number of successful fallback attempts",
		},
		[]string{
			"original_model", "fallback_model", "api_provider",
			"exception_status", "exception_class",
		},
	)

	// FallbackFailed counts failed fallback attempts.
	FallbackFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "fallback_failed",
			Help:      "Number of failed fallback attempts",
		},
		[]string{
			"original_model", "fallback_model", "api_provider",
			"exception_status", "exception_class",
		},
	)
)

// =============================================================================
// Rate Limit Metrics (from Provider responses)
// =============================================================================

var (
	// RemainingRequests tracks remaining requests from provider headers.
	RemainingRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "remaining_requests",
			Help:      "Remaining requests from provider rate limit headers",
		},
		[]string{"model", "api_provider", "api_base"},
	)

	// RemainingTokens tracks remaining tokens from provider headers.
	RemainingTokens = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "remaining_tokens",
			Help:      "Remaining tokens from provider rate limit headers",
		},
		[]string{"model", "api_provider", "api_base"},
	)

	// APIKeyRemainingRequestsForModel tracks remaining requests per API key per model.
	APIKeyRemainingRequestsForModel = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "api_key_remaining_requests_for_model",
			Help:      "Remaining requests for API key on specific model",
		},
		[]string{"hashed_api_key", "api_key_alias", "model"},
	)

	// APIKeyRemainingTokensForModel tracks remaining tokens per API key per model.
	APIKeyRemainingTokensForModel = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "api_key_remaining_tokens_for_model",
			Help:      "Remaining tokens for API key on specific model",
		},
		[]string{"hashed_api_key", "api_key_alias", "model"},
	)
)

// =============================================================================
// Active Requests Metrics
// =============================================================================

var (
	// ActiveRequests tracks currently processing requests.
	ActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_requests",
			Help:      "Number of currently active requests",
		},
		[]string{"deployment_id", "model", "api_provider"},
	)
)
