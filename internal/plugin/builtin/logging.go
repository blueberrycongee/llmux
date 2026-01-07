package builtin

import (
	"log/slog"
	"time"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// LoggingPlugin provides comprehensive request/response logging.
// It logs request details in PreHook and response/error details in PostHook.
type LoggingPlugin struct {
	logger   *slog.Logger
	priority int

	// LogRequestBody controls whether to log the full request body
	LogRequestBody bool

	// LogResponseBody controls whether to log the full response body
	LogResponseBody bool
}

// LoggingOption configures the LoggingPlugin.
type LoggingOption func(*LoggingPlugin)

// WithLogRequestBody enables logging of request body.
func WithLogRequestBody(enabled bool) LoggingOption {
	return func(p *LoggingPlugin) {
		p.LogRequestBody = enabled
	}
}

// WithLogResponseBody enables logging of response body.
func WithLogResponseBody(enabled bool) LoggingOption {
	return func(p *LoggingPlugin) {
		p.LogResponseBody = enabled
	}
}

// WithLoggingPriority sets the plugin priority.
func WithLoggingPriority(priority int) LoggingOption {
	return func(p *LoggingPlugin) {
		p.priority = priority
	}
}

// NewLoggingPlugin creates a new logging plugin.
// Default priority is 1000 (very low, runs last in PreHook, first in PostHook).
func NewLoggingPlugin(logger *slog.Logger, opts ...LoggingOption) *LoggingPlugin {
	if logger == nil {
		logger = slog.Default()
	}

	p := &LoggingPlugin{
		logger:   logger,
		priority: 1000, // Low priority - log after other plugins modify request
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *LoggingPlugin) Name() string  { return "logging" }
func (p *LoggingPlugin) Priority() int { return p.priority }

func (p *LoggingPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
	attrs := []any{
		"request_id", ctx.RequestID,
		"model", req.Model,
		"stream", req.Stream,
		"message_count", len(req.Messages),
	}

	if ctx.Provider != "" {
		attrs = append(attrs, "provider", ctx.Provider)
	}

	if req.MaxTokens > 0 {
		attrs = append(attrs, "max_tokens", req.MaxTokens)
	}

	if req.Temperature != nil {
		attrs = append(attrs, "temperature", *req.Temperature)
	}

	if len(req.Tools) > 0 {
		attrs = append(attrs, "tools_count", len(req.Tools))
	}

	if p.LogRequestBody && len(req.Messages) > 0 {
		// Log last message content (usually the user's query)
		lastMsg := req.Messages[len(req.Messages)-1]
		attrs = append(attrs, "last_message_role", lastMsg.Role)
		if len(lastMsg.Content) > 0 && len(lastMsg.Content) < 500 {
			attrs = append(attrs, "last_message_content", string(lastMsg.Content))
		}
	}

	p.logger.Info("chat completion request started", attrs...)

	// Store start time in context for latency calculation
	ctx.Set("logging_start_time", time.Now())

	return req, nil, nil
}

func (p *LoggingPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	// Calculate latency
	var latency time.Duration
	if startTime, ok := ctx.Get("logging_start_time"); ok {
		if t, ok := startTime.(time.Time); ok {
			latency = time.Since(t)
		}
	}

	attrs := []any{
		"request_id", ctx.RequestID,
		"model", ctx.Model,
		"latency_ms", latency.Milliseconds(),
	}

	if ctx.Provider != "" {
		attrs = append(attrs, "provider", ctx.Provider)
	}

	if err != nil {
		attrs = append(attrs, "error", err.Error())
		p.logger.Error("chat completion request failed", attrs...)
	} else if resp != nil {
		attrs = append(attrs, "response_id", resp.ID)

		if resp.Usage != nil {
			attrs = append(attrs,
				"prompt_tokens", resp.Usage.PromptTokens,
				"completion_tokens", resp.Usage.CompletionTokens,
				"total_tokens", resp.Usage.TotalTokens,
			)
		}

		if len(resp.Choices) > 0 {
			attrs = append(attrs,
				"choices_count", len(resp.Choices),
				"finish_reason", resp.Choices[0].FinishReason,
			)

			if p.LogResponseBody {
				content := resp.Choices[0].Message.Content
				if len(content) > 0 && len(content) < 500 {
					attrs = append(attrs, "response_content", string(content))
				}
			}
		}

		p.logger.Info("chat completion request completed", attrs...)
	}

	// Check for cache hit
	if cacheHit, ok := ctx.Get("cache_hit"); ok && cacheHit.(bool) {
		p.logger.Debug("response served from cache", "request_id", ctx.RequestID)
	}

	return resp, err, nil
}

func (p *LoggingPlugin) Cleanup() error {
	return nil
}

// Ensure LoggingPlugin implements Plugin interface
var _ plugin.Plugin = (*LoggingPlugin)(nil)
