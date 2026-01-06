// Package observability provides a callback system for observability integrations.
// This follows LiteLLM's CustomLogger pattern for extensible logging and tracing.
package observability

import (
	"context"
	"log/slog"
	"time"
)

// CallType represents the type of LLM API call.
type CallType string

const (
	CallTypeCompletion     CallType = "completion"
	CallTypeChatCompletion CallType = "chat_completion"
	CallTypeEmbedding      CallType = "embedding"
	CallTypeImageGen       CallType = "image_generation"
	CallTypeAudioTranscr   CallType = "audio_transcription"
	CallTypeModeration     CallType = "moderation"
)

// RequestStatus represents the status of a request.
type RequestStatus string

const (
	RequestStatusSuccess RequestStatus = "success"
	RequestStatusFailure RequestStatus = "failure"
)

// StandardLoggingPayload is the unified logging data structure.
// This aligns with LiteLLM's StandardLoggingPayload for consistency.
type StandardLoggingPayload struct {
	// Identifiers
	ID        string `json:"id"`
	RequestID string `json:"request_id"`

	// Call info
	CallType CallType      `json:"call_type"`
	Status   RequestStatus `json:"status"`

	// Model info
	RequestedModel string  `json:"requested_model"`
	Model          string  `json:"model"`
	ModelID        *string `json:"model_id,omitempty"`
	ModelGroup     *string `json:"model_group,omitempty"`

	// Provider info
	APIBase     string `json:"api_base"`
	APIProvider string `json:"api_provider"`

	// Token usage
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`

	// Cost
	ResponseCost                 float64        `json:"response_cost"`
	ResponseCostFailureDebugInfo map[string]any `json:"response_cost_failure_debug_info,omitempty"`
	SavedCacheCost               float64        `json:"saved_cache_cost,omitempty"`

	// Timing
	StartTime           time.Time  `json:"startTime"`
	EndTime             time.Time  `json:"endTime"`
	CompletionStartTime *time.Time `json:"completionStartTime,omitempty"` // TTFT

	// Auth context
	EndUser      *string `json:"end_user,omitempty"`
	User         *string `json:"user,omitempty"`
	UserEmail    *string `json:"user_email,omitempty"`
	HashedAPIKey *string `json:"hashed_api_key,omitempty"`
	APIKeyAlias  *string `json:"api_key_alias,omitempty"`
	Team         *string `json:"team,omitempty"`
	TeamAlias    *string `json:"team_alias,omitempty"`
	Organization *string `json:"organization,omitempty"`

	// Request details
	Messages        any            `json:"messages,omitempty"`
	Response        any            `json:"response,omitempty"`
	ModelParameters map[string]any `json:"model_parameters,omitempty"`
	HiddenParams    map[string]any `json:"hidden_params,omitempty"`

	// Error info
	ErrorStr       *string `json:"error_str,omitempty"`
	ExceptionClass *string `json:"exception_class,omitempty"`

	// Cache
	CacheHit *bool   `json:"cache_hit,omitempty"`
	CacheKey *string `json:"cache_key,omitempty"`

	// Metadata
	RequestTags        []string       `json:"request_tags,omitempty"`
	RequesterIPAddress *string        `json:"requester_ip_address,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// Callback defines the interface for observability callbacks.
// Implementations can log to various backends (Prometheus, OTEL, Langfuse, etc.)
type Callback interface {
	// Name returns the callback name for identification.
	Name() string

	// LogPreAPICall is called before making an LLM API call.
	LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error

	// LogPostAPICall is called after receiving a response (success or failure).
	LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error

	// LogStreamEvent is called for each streaming chunk.
	LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error

	// LogSuccessEvent is called when a request completes successfully.
	LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error

	// LogFailureEvent is called when a request fails.
	LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error

	// LogFallbackEvent is called when a fallback occurs.
	LogFallbackEvent(ctx context.Context, originalModel string, fallbackModel string, err error, success bool) error

	// Shutdown gracefully shuts down the callback.
	Shutdown(ctx context.Context) error
}

// AsyncCallback extends Callback with async methods.
type AsyncCallback interface {
	Callback

	// AsyncLogSuccessEvent is the async version of LogSuccessEvent.
	AsyncLogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error

	// AsyncLogFailureEvent is the async version of LogFailureEvent.
	AsyncLogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error
}

// CallbackManager manages multiple callbacks.
type CallbackManager struct {
	callbacks []Callback
	logger    *Logger
}

// NewCallbackManager creates a new callback manager.
func NewCallbackManager() *CallbackManager {
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     nil, // Will use default
		JSONFormat: true,
	}
	return &CallbackManager{
		callbacks: make([]Callback, 0),
		logger:    NewLogger(cfg, nil),
	}
}

// Register adds a callback to the manager.
func (m *CallbackManager) Register(cb Callback) {
	m.callbacks = append(m.callbacks, cb)
}

// Unregister removes a callback by name.
func (m *CallbackManager) Unregister(name string) {
	for i, cb := range m.callbacks {
		if cb.Name() == name {
			m.callbacks = append(m.callbacks[:i], m.callbacks[i+1:]...)
			return
		}
	}
}

// LogPreAPICall calls all registered callbacks.
func (m *CallbackManager) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) {
	for _, cb := range m.callbacks {
		if err := cb.LogPreAPICall(ctx, payload); err != nil {
			m.logger.Error("callback LogPreAPICall failed", "error", err)
		}
	}
}

// LogPostAPICall calls all registered callbacks.
func (m *CallbackManager) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) {
	for _, cb := range m.callbacks {
		if err := cb.LogPostAPICall(ctx, payload); err != nil {
			m.logger.Error("callback LogPostAPICall failed", "error", err)
		}
	}
}

// LogStreamEvent calls all registered callbacks.
func (m *CallbackManager) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) {
	for _, cb := range m.callbacks {
		if err := cb.LogStreamEvent(ctx, payload, chunk); err != nil {
			m.logger.Error("callback LogStreamEvent failed", "error", err)
		}
	}
}

// LogSuccessEvent calls all registered callbacks.
func (m *CallbackManager) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) {
	for _, cb := range m.callbacks {
		if err := cb.LogSuccessEvent(ctx, payload); err != nil {
			m.logger.Error("callback LogSuccessEvent failed", "error", err)
		}
	}
}

// LogFailureEvent calls all registered callbacks.
func (m *CallbackManager) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) {
	for _, cb := range m.callbacks {
		if logErr := cb.LogFailureEvent(ctx, payload, err); logErr != nil {
			m.logger.Error("callback LogFailureEvent failed", "error", logErr)
		}
	}
}

// LogFallbackEvent calls all registered callbacks.
func (m *CallbackManager) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) {
	for _, cb := range m.callbacks {
		if logErr := cb.LogFallbackEvent(ctx, originalModel, fallbackModel, err, success); logErr != nil {
			m.logger.Error("callback LogFallbackEvent failed", "error", logErr)
		}
	}
}

// Shutdown gracefully shuts down all callbacks.
func (m *CallbackManager) Shutdown(ctx context.Context) error {
	for _, cb := range m.callbacks {
		if err := cb.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}
