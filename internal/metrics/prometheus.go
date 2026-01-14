// Package metrics provides comprehensive Prometheus metrics collection for the LLM gateway.
// It tracks request counts, latencies, token usage, costs, budgets, and deployment health.
// This implementation aligns with LiteLLM's observability patterns.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "llmux"
)

// LatencyBuckets defines histogram buckets for latency metrics (in seconds).
// Aligned with LiteLLM's bucket configuration for consistency.
var LatencyBuckets = []float64{
	0.005, 0.00625, 0.0125, 0.025, 0.05, 0.1, 0.5,
	1.0, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5.0,
	5.5, 6.0, 6.5, 7.0, 7.5, 8.0, 8.5, 9.0, 9.5,
	10.0, 15.0, 20.0, 25.0, 30.0, 60.0, 120.0,
	180.0, 240.0, 300.0,
}

// =============================================================================
// Request Metrics
// =============================================================================

var (
	// ProxyTotalRequests counts total proxy requests.
	ProxyTotalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "proxy_total_requests",
			Help:      "Total number of proxy requests",
		},
		[]string{
			"model", "model_group", "api_provider", "status_code",
		},
	)

	// ProxyFailedRequests counts failed proxy requests.
	ProxyFailedRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "proxy_failed_requests",
			Help:      "Total number of failed proxy requests",
		},
		[]string{
			"model", "model_group", "api_provider", "exception_status", "exception_class",
		},
	)
)

// =============================================================================
// Latency Metrics
// =============================================================================

var (
	// RequestTotalLatency tracks total request latency (end-to-end).
	RequestTotalLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_total_latency_seconds",
			Help:      "Total request latency in seconds (end-to-end)",
			Buckets:   LatencyBuckets,
		},
		[]string{
			"model", "model_group", "api_provider",
		},
	)

	// LLMAPILatency tracks LLM API call latency.
	LLMAPILatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "llm_api_latency_seconds",
			Help:      "LLM API call latency in seconds",
			Buckets:   LatencyBuckets,
		},
		[]string{
			"model", "model_group", "api_provider", "api_base",
		},
	)

	// TimeToFirstToken tracks TTFT for streaming requests.
	TimeToFirstToken = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "time_to_first_token_seconds",
			Help:      "Time to first token for streaming requests",
			Buckets:   LatencyBuckets,
		},
		[]string{
			"model", "model_group", "api_provider", "api_base",
		},
	)

	// OverheadLatency tracks LLMux processing overhead.
	OverheadLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "overhead_latency_seconds",
			Help:      "LLMux processing overhead latency",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"route"},
	)

	// LatencyPerOutputToken tracks latency per output token.
	LatencyPerOutputToken = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "latency_per_output_token_seconds",
			Help:      "Latency per output token",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5},
		},
		[]string{
			"model", "model_group", "api_provider",
		},
	)
)

// =============================================================================
// Token Metrics
// =============================================================================

var (
	// TotalTokens counts total tokens used.
	TotalTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "total_tokens",
			Help:      "Total tokens used",
		},
		[]string{
			"model", "model_group", "api_provider",
		},
	)

	// InputTokens counts input tokens.
	InputTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "input_tokens",
			Help:      "Total input tokens",
		},
		[]string{
			"model", "model_group", "api_provider",
		},
	)

	// OutputTokens counts output tokens.
	OutputTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "output_tokens",
			Help:      "Total output tokens",
		},
		[]string{
			"model", "model_group", "api_provider",
		},
	)
)

// =============================================================================
// Cost Metrics
// =============================================================================

var (
	// TotalSpend tracks total spend.
	TotalSpend = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "spend_total",
			Help:      "Total spend in USD",
		},
		[]string{
			"model", "model_group", "api_provider",
		},
	)
)
