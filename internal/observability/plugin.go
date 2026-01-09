package observability

import (
	"context"
	"time"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// ObservabilityPlugin integrates Redactor and OTelMetricsProvider into the plugin system.
type ObservabilityPlugin struct {
	redactor *Redactor
	metrics  *OTelMetricsProvider
}

// NewObservabilityPlugin creates a new observability plugin.
func NewObservabilityPlugin(redactor *Redactor, metrics *OTelMetricsProvider) *ObservabilityPlugin {
	return &ObservabilityPlugin{
		redactor: redactor,
		metrics:  metrics,
	}
}

// Name returns the plugin name.
func (p *ObservabilityPlugin) Name() string {
	return "observability"
}

// Priority returns the plugin priority.
// It should run early in PreHook and late in PostHook to capture accurate timings.
func (p *ObservabilityPlugin) Priority() int {
	return -1000 // High priority (low number)
}

// PreHook prepares the context for observability.
func (p *ObservabilityPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
	// No modification to request, just ensuring context has what we need
	// The context already has StartTime and RequestID
	return req, nil, nil
}

// PostHook records metrics and logs the request/response.
func (p *ObservabilityPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	if p.metrics == nil {
		return resp, nil, nil
	}

	// Construct payload for metrics
	payload := &StandardLoggingPayload{
		RequestID:      ctx.RequestID,
		CallType:       CallTypeChatCompletion,
		RequestedModel: ctx.Model,
		Model:          ctx.Model, // May be updated if we have deployment info
		StartTime:      ctx.StartTime,
		EndTime:        time.Now(),
	}

	if ctx.Deployment != nil {
		payload.APIProvider = ctx.Deployment.ProviderName
		payload.Model = ctx.Deployment.ModelName
	}

	if err != nil {
		payload.Status = RequestStatusFailure
		errStr := err.Error()
		payload.ErrorStr = &errStr
	} else {
		payload.Status = RequestStatusSuccess
		if resp != nil && resp.Usage != nil {
			payload.PromptTokens = resp.Usage.PromptTokens
			payload.CompletionTokens = resp.Usage.CompletionTokens
			payload.TotalTokens = resp.Usage.TotalTokens
		}
	}

	// Record metrics
	p.metrics.RecordRequest(ctx.Context, payload)

	return resp, nil, nil
}

// Cleanup cleans up resources.
func (p *ObservabilityPlugin) Cleanup() error {
	if p.metrics != nil {
		return p.metrics.Shutdown(context.Background())
	}
	return nil
}
