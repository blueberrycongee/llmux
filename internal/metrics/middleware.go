// Package metrics provides Prometheus metrics collection for the LLM gateway.
// It tracks request counts, latencies, token usage, and error rates.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total requests by provider, model, and status (legacy).
	// For new code, use ProxyTotalRequests from prometheus.go instead.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llmux",
			Name:      "requests_total",
			Help:      "Total number of LLM requests (legacy)",
		},
		[]string{"provider", "model", "status"},
	)

	// RequestLatency tracks request latency distribution (legacy).
	// For new code, use RequestTotalLatency from prometheus.go instead.
	RequestLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "llmux",
			Name:      "request_latency_seconds",
			Help:      "Request latency in seconds (legacy)",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model"},
	)

	// TokenUsage tracks token consumption by type (legacy).
	// For new code, use InputTokens/OutputTokens from prometheus.go instead.
	TokenUsage = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llmux",
			Name:      "token_usage_total",
			Help:      "Total token usage (legacy)",
		},
		[]string{"provider", "model", "type"}, // type: input, output
	)

	// UpstreamErrors counts errors by type.
	UpstreamErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llmux",
			Name:      "upstream_errors_total",
			Help:      "Total upstream errors by type",
		},
		[]string{"provider", "error_type"},
	)

	// CircuitBreakerState tracks circuit breaker status.
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "llmux",
			Name:      "circuit_breaker_state",
			Help:      "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"provider", "deployment_id"},
	)
)

// RecordRequest records metrics for a completed request.
func RecordRequest(provider, model string, statusCode int, latency time.Duration) {
	status := strconv.Itoa(statusCode)
	RequestsTotal.WithLabelValues(provider, model, status).Inc()
	RequestLatency.WithLabelValues(provider, model).Observe(latency.Seconds())
}

// RecordTokens records token usage metrics.
func RecordTokens(provider, model string, inputTokens, outputTokens int) {
	if inputTokens > 0 {
		TokenUsage.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		TokenUsage.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
	}
}

// RecordError records an upstream error.
func RecordError(provider, errorType string) {
	UpstreamErrors.WithLabelValues(provider, errorType).Inc()
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware returns an HTTP middleware that records request metrics.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		// Record basic HTTP metrics
		// Provider and model specific metrics are recorded in the handler
		latency := time.Since(start)
		RequestLatency.WithLabelValues("gateway", "all").Observe(latency.Seconds())
	})
}
