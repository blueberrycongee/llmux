// Package observability provides a Prometheus callback implementation.
package observability

import (
	"context"

	"github.com/blueberrycongee/llmux/internal/metrics"
)

// PrometheusCallback implements Callback for Prometheus metrics.
type PrometheusCallback struct {
	collector *metrics.Collector
}

// NewPrometheusCallback creates a new Prometheus callback.
func NewPrometheusCallback() *PrometheusCallback {
	return &PrometheusCallback{
		collector: metrics.NewCollector(),
	}
}

// Name returns the callback name.
func (p *PrometheusCallback) Name() string {
	return "prometheus"
}

// LogPreAPICall records pre-request metrics.
func (p *PrometheusCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	// Increment active requests
	deploymentID := ""
	if payload.ModelID != nil {
		deploymentID = *payload.ModelID
	}
	p.collector.RecordActiveRequest(deploymentID, payload.Model, payload.APIProvider, 1)
	return nil
}

// LogPostAPICall records post-request metrics.
func (p *PrometheusCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	// Decrement active requests
	deploymentID := ""
	if payload.ModelID != nil {
		deploymentID = *payload.ModelID
	}
	p.collector.RecordActiveRequest(deploymentID, payload.Model, payload.APIProvider, -1)
	return nil
}

// LogStreamEvent records streaming metrics.
func (p *PrometheusCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	// Stream events are handled in LogSuccessEvent with TTFT
	return nil
}

// LogSuccessEvent records success metrics.
func (p *PrometheusCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	m := p.payloadToMetrics(payload)
	m.Success = true
	p.collector.RecordRequest(m)
	return nil
}

// LogFailureEvent records failure metrics.
func (p *PrometheusCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	m := p.payloadToMetrics(payload)
	m.Success = false
	if payload.ExceptionClass != nil {
		m.Labels.ExceptionClass = *payload.ExceptionClass
	}
	p.collector.RecordRequest(m)
	return nil
}

// LogFallbackEvent records fallback metrics.
func (p *PrometheusCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	exceptionStatus := ""
	exceptionClass := ""
	if err != nil {
		exceptionClass = "error"
	}
	p.collector.RecordFallback(originalModel, fallbackModel, "", exceptionStatus, exceptionClass, success)
	return nil
}

// Shutdown gracefully shuts down the callback.
func (p *PrometheusCallback) Shutdown(ctx context.Context) error {
	return nil
}

// payloadToMetrics converts StandardLoggingPayload to RequestMetrics.
func (p *PrometheusCallback) payloadToMetrics(payload *StandardLoggingPayload) *metrics.RequestMetrics {
	labels := metrics.Labels{
		RequestedModel: payload.RequestedModel,
		Model:          payload.Model,
		APIProvider:    payload.APIProvider,
		APIBase:        payload.APIBase,
		StatusCode:     200,
	}

	// Optional fields
	if payload.EndUser != nil {
		labels.EndUser = *payload.EndUser
	}
	if payload.User != nil {
		labels.User = *payload.User
	}
	if payload.HashedAPIKey != nil {
		labels.HashedAPIKey = *payload.HashedAPIKey
	}
	if payload.APIKeyAlias != nil {
		labels.APIKeyAlias = *payload.APIKeyAlias
	}
	if payload.Team != nil {
		labels.Team = *payload.Team
	}
	if payload.TeamAlias != nil {
		labels.TeamAlias = *payload.TeamAlias
	}
	if payload.ModelGroup != nil {
		labels.ModelGroup = *payload.ModelGroup
	}
	if payload.ModelID != nil {
		labels.DeploymentID = *payload.ModelID
	}

	// Error status
	if payload.Status == RequestStatusFailure {
		labels.StatusCode = 500
		if payload.ErrorStr != nil {
			labels.ExceptionStatus = "500"
		}
	}

	m := &metrics.RequestMetrics{
		Labels:       labels,
		StartTime:    payload.StartTime,
		EndTime:      payload.EndTime,
		InputTokens:  payload.PromptTokens,
		OutputTokens: payload.CompletionTokens,
		TotalTokens:  payload.TotalTokens,
		Cost:         payload.ResponseCost,
		Success:      payload.Status == RequestStatusSuccess,
	}

	// TTFT
	if payload.CompletionStartTime != nil {
		m.TTFT = payload.CompletionStartTime.Sub(payload.StartTime)
		m.Streaming = true
	}

	// Upstream time (total - overhead)
	m.UpstreamTime = payload.EndTime.Sub(payload.StartTime)

	// Cache hit
	if payload.CacheHit != nil {
		m.CacheHit = *payload.CacheHit
	}

	return m
}
