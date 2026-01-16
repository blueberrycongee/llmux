// Package observability provides an OpenTelemetry callback implementation.
package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Gen AI semantic convention attributes.
// See: https://opentelemetry.io/docs/specs/semconv/gen-ai/
const (
	// Gen AI attributes
	GenAISystem               = "gen_ai.system"
	GenAIRequestModel         = "gen_ai.request.model"
	GenAIRequestMaxTokens     = "gen_ai.request.max_tokens" // #nosec G101 -- attribute key, not a credential.
	GenAIRequestTemperature   = "gen_ai.request.temperature"
	GenAIRequestTopP          = "gen_ai.request.top_p"
	GenAIRequestStream        = "gen_ai.request.stream"
	GenAIOperationName        = "gen_ai.operation.name"
	GenAIUsageInputTokens     = "gen_ai.usage.input_tokens"  // #nosec G101 -- attribute key, not a credential.
	GenAIUsageOutputTokens    = "gen_ai.usage.output_tokens" // #nosec G101 -- attribute key, not a credential.
	GenAIResponseFinishReason = "gen_ai.response.finish_reason"
	GenAIResponseID           = "gen_ai.response.id"
	GenAIFramework            = "gen_ai.framework"

	// LLMux specific attributes
	LLMuxRequestID    = "llmux.request_id"
	LLMuxTeam         = "llmux.team"
	LLMuxTeamAlias    = "llmux.team_alias"
	LLMuxUser         = "llmux.user"
	LLMuxEndUser      = "llmux.end_user"
	LLMuxAPIKeyAlias  = "llmux.api_key_alias" // #nosec G101 -- attribute key, not a credential.
	LLMuxModelGroup   = "llmux.model_group"
	LLMuxDeploymentID = "llmux.deployment_id"
	LLMuxCacheHit     = "llmux.cache_hit"
	LLMuxResponseCost = "llmux.response_cost"
	LLMuxTTFTMs       = "llmux.ttft_ms"
)

// OTelCallback implements Callback for OpenTelemetry tracing.
type OTelCallback struct {
	tracer          trace.Tracer
	includeMessages bool // Whether to include request/response messages in spans
}

// OTelCallbackConfig contains configuration for OTelCallback.
type OTelCallbackConfig struct {
	Tracer          trace.Tracer
	IncludeMessages bool // Include raw messages (may contain sensitive data)
}

// NewOTelCallback creates a new OpenTelemetry callback.
func NewOTelCallback(cfg OTelCallbackConfig) *OTelCallback {
	return &OTelCallback{
		tracer:          cfg.Tracer,
		includeMessages: cfg.IncludeMessages,
	}
}

// Name returns the callback name.
func (o *OTelCallback) Name() string {
	return "opentelemetry"
}

// LogPreAPICall starts a span for the LLM request.
func (o *OTelCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	// Span is typically started by the handler, this is for additional attributes
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.String(GenAISystem, payload.APIProvider),
		attribute.String(GenAIRequestModel, payload.Model),
		attribute.String(GenAIOperationName, string(payload.CallType)),
		attribute.String(GenAIFramework, "llmux"),
		attribute.String(LLMuxRequestID, payload.RequestID),
	}

	// Add optional attributes
	if payload.Team != nil {
		attrs = append(attrs, attribute.String(LLMuxTeam, *payload.Team))
	}
	if payload.TeamAlias != nil {
		attrs = append(attrs, attribute.String(LLMuxTeamAlias, *payload.TeamAlias))
	}
	if payload.User != nil {
		attrs = append(attrs, attribute.String(LLMuxUser, *payload.User))
	}
	if payload.EndUser != nil {
		attrs = append(attrs, attribute.String(LLMuxEndUser, *payload.EndUser))
	}
	if payload.APIKeyAlias != nil {
		attrs = append(attrs, attribute.String(LLMuxAPIKeyAlias, *payload.APIKeyAlias))
	}
	if payload.ModelGroup != nil {
		attrs = append(attrs, attribute.String(LLMuxModelGroup, *payload.ModelGroup))
	}
	if payload.ModelID != nil {
		attrs = append(attrs, attribute.String(LLMuxDeploymentID, *payload.ModelID))
	}

	// Model parameters
	if params := payload.ModelParameters; params != nil {
		if maxTokens, ok := params["max_tokens"].(int); ok {
			attrs = append(attrs, attribute.Int(GenAIRequestMaxTokens, maxTokens))
		}
		if temp, ok := params["temperature"].(float64); ok {
			attrs = append(attrs, attribute.Float64(GenAIRequestTemperature, temp))
		}
		if topP, ok := params["top_p"].(float64); ok {
			attrs = append(attrs, attribute.Float64(GenAIRequestTopP, topP))
		}
		if stream, ok := params["stream"].(bool); ok {
			attrs = append(attrs, attribute.Bool(GenAIRequestStream, stream))
		}
	}

	span.SetAttributes(attrs...)
	return nil
}

// LogPostAPICall adds response attributes to the span.
func (o *OTelCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.Int(GenAIUsageInputTokens, payload.PromptTokens),
		attribute.Int(GenAIUsageOutputTokens, payload.CompletionTokens),
		attribute.Float64(LLMuxResponseCost, payload.ResponseCost),
	}

	// TTFT for streaming
	if payload.CompletionStartTime != nil {
		ttftMs := payload.CompletionStartTime.Sub(payload.StartTime).Milliseconds()
		attrs = append(attrs, attribute.Int64(LLMuxTTFTMs, ttftMs))
	}

	// Cache hit
	if payload.CacheHit != nil {
		attrs = append(attrs, attribute.Bool(LLMuxCacheHit, *payload.CacheHit))
	}

	span.SetAttributes(attrs...)
	return nil
}

// LogStreamEvent records streaming chunk events.
func (o *OTelCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	// Add event for first token
	if payload.CompletionStartTime != nil {
		span.AddEvent("first_token_received", trace.WithAttributes(
			attribute.Int64(LLMuxTTFTMs, payload.CompletionStartTime.Sub(payload.StartTime).Milliseconds()),
		))
	}

	return nil
}

// LogSuccessEvent marks the span as successful.
func (o *OTelCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	span.SetStatus(codes.Ok, "")

	// Add response ID if available
	if payload.ID != "" {
		span.SetAttributes(attribute.String(GenAIResponseID, payload.ID))
	}

	return nil
}

// LogFailureEvent marks the span as failed.
func (o *OTelCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)

	if payload.ExceptionClass != nil {
		span.SetAttributes(attribute.String("exception.type", *payload.ExceptionClass))
	}

	return nil
}

// LogFallbackEvent records a fallback event.
func (o *OTelCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	var eventName string
	if success {
		eventName = "fallback_successful"
	} else {
		eventName = "fallback_failed"
	}

	attrs := []attribute.KeyValue{
		attribute.String("original_model", originalModel),
		attribute.String("fallback_model", fallbackModel),
		attribute.Bool("success", success),
	}

	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
	}

	span.AddEvent(eventName, trace.WithAttributes(attrs...))
	return nil
}

// Shutdown gracefully shuts down the callback.
func (o *OTelCallback) Shutdown(ctx context.Context) error {
	return nil
}

// StartLLMSpanWithPayload starts a new span using StandardLoggingPayload.
func StartLLMSpanWithPayload(ctx context.Context, tracer trace.Tracer, payload *StandardLoggingPayload) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("%s %s", payload.CallType, payload.Model)

	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(GenAISystem, payload.APIProvider),
			attribute.String(GenAIRequestModel, payload.Model),
			attribute.String(GenAIOperationName, string(payload.CallType)),
			attribute.String(GenAIFramework, "llmux"),
		),
	)

	return ctx, span
}
