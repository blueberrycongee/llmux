// Package metrics provides a unified metrics collector for LLMux.
package metrics

import (
	"strconv"
	"time"
)

// Labels contains all possible label values for metrics.
type Labels struct {
	// Request context
	EndUser      string
	User         string
	HashedAPIKey string
	APIKeyAlias  string
	Team         string
	TeamAlias    string

	// Model info
	RequestedModel string
	Model          string
	ModelGroup     string
	ModelID        string

	// Provider info
	APIProvider  string
	APIBase      string
	DeploymentID string

	// Error info
	StatusCode      int
	ExceptionStatus string
	ExceptionClass  string

	// Routing info
	Route         string
	FallbackModel string
	Tag           string
}

// RequestMetrics contains metrics for a single request.
type RequestMetrics struct {
	Labels Labels

	// Timing
	StartTime    time.Time
	EndTime      time.Time
	TTFT         time.Duration // Time to first token
	OverheadTime time.Duration // LLMux processing overhead
	UpstreamTime time.Duration // Actual LLM API time

	// Tokens
	InputTokens  int
	OutputTokens int
	TotalTokens  int

	// Cost
	Cost float64

	// Status
	Success   bool
	CacheHit  bool
	Streaming bool
}

// Collector provides methods to record metrics.
type Collector struct{}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{}
}

// RecordRequest records all metrics for a completed request.
func (c *Collector) RecordRequest(m *RequestMetrics) {
	labels := m.Labels
	statusCode := strconv.Itoa(labels.StatusCode)

	// Total requests
	ProxyTotalRequests.WithLabelValues(
		labels.Model, labels.ModelGroup, labels.APIProvider, statusCode,
	).Inc()

	// Failed requests
	if !m.Success {
		ProxyFailedRequests.WithLabelValues(
			labels.Model, labels.ModelGroup, labels.APIProvider, labels.ExceptionStatus, labels.ExceptionClass,
		).Inc()
	}

	// Total latency
	totalLatency := m.EndTime.Sub(m.StartTime).Seconds()
	RequestTotalLatency.WithLabelValues(
		labels.Model, labels.ModelGroup, labels.APIProvider,
	).Observe(totalLatency)

	// LLM API latency
	if m.UpstreamTime > 0 {
		LLMAPILatency.WithLabelValues(
			labels.Model, labels.ModelGroup, labels.APIProvider, labels.APIBase,
		).Observe(m.UpstreamTime.Seconds())
	}

	// TTFT for streaming
	if m.Streaming && m.TTFT > 0 {
		TimeToFirstToken.WithLabelValues(
			labels.Model, labels.ModelGroup, labels.APIProvider, labels.APIBase,
		).Observe(m.TTFT.Seconds())
	}

	// Overhead latency
	if m.OverheadTime > 0 {
		OverheadLatency.WithLabelValues(labels.Route).Observe(m.OverheadTime.Seconds())
	}

	// Latency per output token
	if m.OutputTokens > 0 && m.UpstreamTime > 0 {
		latencyPerToken := m.UpstreamTime.Seconds() / float64(m.OutputTokens)
		LatencyPerOutputToken.WithLabelValues(
			labels.Model, labels.ModelGroup, labels.APIProvider,
		).Observe(latencyPerToken)
	}

	// Token metrics
	tokenLabels := []string{labels.Model, labels.ModelGroup, labels.APIProvider}

	if m.TotalTokens > 0 {
		TotalTokens.WithLabelValues(tokenLabels...).Add(float64(m.TotalTokens))
	}
	if m.InputTokens > 0 {
		InputTokens.WithLabelValues(tokenLabels...).Add(float64(m.InputTokens))
	}
	if m.OutputTokens > 0 {
		OutputTokens.WithLabelValues(tokenLabels...).Add(float64(m.OutputTokens))
	}

	// Cost
	if m.Cost > 0 {
		TotalSpend.WithLabelValues(tokenLabels...).Add(m.Cost)
	}

	// Deployment metrics
	if labels.DeploymentID != "" {
		deploymentLabels := []string{
			labels.DeploymentID, labels.Model, labels.ModelGroup,
			labels.APIProvider, labels.APIBase,
		}

		DeploymentTotalRequests.WithLabelValues(deploymentLabels...).Inc()

		if m.Success {
			DeploymentSuccessResponses.WithLabelValues(deploymentLabels...).Inc()
			DeploymentState.WithLabelValues(deploymentLabels...).Set(DeploymentStateHealthy)
		} else {
			DeploymentFailureResponses.WithLabelValues(
				labels.DeploymentID, labels.Model, labels.ModelGroup,
				labels.APIProvider, labels.APIBase, labels.ExceptionStatus,
			).Inc()
		}

		// Latency per output token for deployment
		if m.OutputTokens > 0 && m.UpstreamTime > 0 {
			latencyPerToken := m.UpstreamTime.Seconds() / float64(m.OutputTokens)
			DeploymentLatencyPerOutputToken.WithLabelValues(
				labels.DeploymentID, labels.Model, labels.ModelGroup, labels.APIProvider,
			).Observe(latencyPerToken)
		}
	}
}

// RecordFallback records a fallback attempt.
func (c *Collector) RecordFallback(originalModel, fallbackModel, provider, exceptionStatus, exceptionClass string, success bool) {
	labels := []string{originalModel, fallbackModel, provider, exceptionStatus, exceptionClass}

	if success {
		FallbackSuccessful.WithLabelValues(labels...).Inc()
	} else {
		FallbackFailed.WithLabelValues(labels...).Inc()
	}
}

// RecordDeploymentCooldown records when a deployment enters cooldown.
func (c *Collector) RecordDeploymentCooldown(deploymentID, model, modelGroup, provider, apiBase string) {
	DeploymentCooledDown.WithLabelValues(deploymentID, model, modelGroup, provider, apiBase).Inc()
	DeploymentState.WithLabelValues(deploymentID, model, modelGroup, provider, apiBase).Set(DeploymentStateFailed)
}

// RecordActiveRequest increments/decrements active request count.
func (c *Collector) RecordActiveRequest(deploymentID, model, provider string, delta float64) {
	ActiveRequests.WithLabelValues(deploymentID, model, provider).Add(delta)
}

// UpdateBudgetMetrics updates budget-related gauge metrics.
func (c *Collector) UpdateBudgetMetrics(budgetType string, labels []string, remaining, maxBudget, remainingHours float64) {
	switch budgetType {
	case "team":
		if len(labels) >= 2 {
			TeamRemainingBudget.WithLabelValues(labels[0], labels[1]).Set(remaining)
			TeamMaxBudget.WithLabelValues(labels[0], labels[1]).Set(maxBudget)
			TeamBudgetRemainingHours.WithLabelValues(labels[0], labels[1]).Set(remainingHours)
		}
	case "org":
		if len(labels) >= 2 {
			OrgRemainingBudget.WithLabelValues(labels[0], labels[1]).Set(remaining)
			OrgMaxBudget.WithLabelValues(labels[0], labels[1]).Set(maxBudget)
		}
	case "provider":
		if len(labels) >= 1 {
			ProviderRemainingBudget.WithLabelValues(labels[0]).Set(remaining)
		}
	}
}

// UpdateRateLimitMetrics updates rate limit gauge metrics from provider headers.
func (c *Collector) UpdateRateLimitMetrics(model, provider, apiBase string, remainingRequests, remainingTokens int64) {
	if remainingRequests >= 0 {
		RemainingRequests.WithLabelValues(model, provider, apiBase).Set(float64(remainingRequests))
	}
	if remainingTokens >= 0 {
		RemainingTokens.WithLabelValues(model, provider, apiBase).Set(float64(remainingTokens))
	}
}

// UpdateAPIKeyRateLimits updates API key rate limit metrics.
func (c *Collector) UpdateAPIKeyRateLimits(hashedKey, alias, model string, remainingRequests, remainingTokens int64) {
	// Intentionally no-op: per-API-key Prometheus series are high-cardinality and not safe by default.
}
