// Package observability provides a Langfuse callback implementation for LLM tracing.
// Langfuse is an open-source LLM observability platform.
package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LangfuseConfig contains configuration for Langfuse integration.
type LangfuseConfig struct {
	PublicKey     string        // LANGFUSE_PUBLIC_KEY
	SecretKey     string        // LANGFUSE_SECRET_KEY
	Host          string        // LANGFUSE_HOST (default: https://cloud.langfuse.com)
	FlushInterval time.Duration // Flush interval for batching (default: 1s)
	BatchSize     int           // Max batch size before flush (default: 100)
	Debug         bool          // Enable debug logging
	MaskInput     bool          // Mask input content
	MaskOutput    bool          // Mask output content
}

// DefaultLangfuseConfig returns default configuration from environment.
func DefaultLangfuseConfig() LangfuseConfig {
	host := os.Getenv("LANGFUSE_HOST")
	if host == "" {
		host = "https://cloud.langfuse.com"
	}
	return LangfuseConfig{
		PublicKey:     os.Getenv("LANGFUSE_PUBLIC_KEY"),
		SecretKey:     os.Getenv("LANGFUSE_SECRET_KEY"),
		Host:          host,
		FlushInterval: time.Second,
		BatchSize:     100,
		Debug:         os.Getenv("LANGFUSE_DEBUG") == "true",
		MaskInput:     false,
		MaskOutput:    false,
	}
}

// LangfuseCallback implements Callback for Langfuse tracing.
type LangfuseCallback struct {
	config     LangfuseConfig
	client     *http.Client
	eventQueue []langfuseEvent
	mu         sync.Mutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// langfuseEvent represents an event to be sent to Langfuse.
type langfuseEvent struct {
	Type string      `json:"type"`
	Body interface{} `json:"body"`
}

// langfuseTrace represents a Langfuse trace.
type langfuseTrace struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	UserID    string                 `json:"userId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// langfuseGeneration represents a Langfuse generation (LLM call).
type langfuseGeneration struct {
	ID                  string                 `json:"id"`
	TraceID             string                 `json:"traceId"`
	Name                string                 `json:"name,omitempty"`
	Model               string                 `json:"model,omitempty"`
	ModelParameters     map[string]interface{} `json:"modelParameters,omitempty"`
	Input               interface{}            `json:"input,omitempty"`
	Output              interface{}            `json:"output,omitempty"`
	Usage               *langfuseUsage         `json:"usage,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Level               string                 `json:"level,omitempty"` // DEFAULT, DEBUG, WARNING, ERROR
	StatusMessage       string                 `json:"statusMessage,omitempty"`
	StartTime           time.Time              `json:"startTime"`
	EndTime             time.Time              `json:"endTime,omitempty"`
	CompletionStartTime *time.Time             `json:"completionStartTime,omitempty"`
}

// langfuseUsage represents token usage.
type langfuseUsage struct {
	PromptTokens     int     `json:"promptTokens,omitempty"`
	CompletionTokens int     `json:"completionTokens,omitempty"`
	TotalTokens      int     `json:"totalTokens,omitempty"`
	InputCost        float64 `json:"inputCost,omitempty"`
	OutputCost       float64 `json:"outputCost,omitempty"`
	TotalCost        float64 `json:"totalCost,omitempty"`
}

// langfuseBatchRequest represents a batch ingestion request.
type langfuseBatchRequest struct {
	Batch    []langfuseEvent `json:"batch"`
	Metadata struct {
		SDKIntegration string `json:"sdk_integration"`
		SDKVersion     string `json:"sdk_version"`
	} `json:"metadata"`
}

// NewLangfuseCallback creates a new Langfuse callback.
func NewLangfuseCallback(cfg LangfuseConfig) (*LangfuseCallback, error) {
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("langfuse: public_key and secret_key are required")
	}

	cb := &LangfuseCallback{
		config:     cfg,
		client:     &http.Client{Timeout: 30 * time.Second},
		eventQueue: make([]langfuseEvent, 0, cfg.BatchSize),
		stopCh:     make(chan struct{}),
	}

	// Start background flush goroutine
	cb.wg.Add(1)
	go cb.flushLoop()

	return cb, nil
}

// Name returns the callback name.
func (l *LangfuseCallback) Name() string {
	return "langfuse"
}

// LogPreAPICall creates a trace for the request.
func (l *LangfuseCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	traceID := l.getOrCreateTraceID(payload)

	trace := langfuseTrace{
		ID:        traceID,
		Name:      fmt.Sprintf("llmux-%s", payload.CallType),
		Timestamp: payload.StartTime,
		Tags:      l.buildTags(payload),
	}

	// Set user ID
	if payload.EndUser != nil {
		trace.UserID = *payload.EndUser
	} else if payload.User != nil {
		trace.UserID = *payload.User
	}

	// Set input (with optional masking)
	if !l.config.MaskInput {
		trace.Input = payload.Messages
	} else {
		trace.Input = "redacted-by-llmux"
	}

	// Build metadata
	trace.Metadata = l.buildMetadata(payload)

	l.enqueue(langfuseEvent{Type: "trace-create", Body: trace})
	return nil
}

// LogPostAPICall updates the trace with response.
func (l *LangfuseCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	// Generation is logged in success/failure events
	return nil
}

// LogStreamEvent records streaming events.
func (l *LangfuseCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	// Streaming events are aggregated in the final generation
	return nil
}

// LogSuccessEvent logs a successful generation.
func (l *LangfuseCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	return l.logGeneration(ctx, payload, "DEFAULT", "")
}

// LogFailureEvent logs a failed generation.
func (l *LangfuseCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	statusMsg := ""
	if err != nil {
		statusMsg = err.Error()
	} else if payload.ErrorStr != nil {
		statusMsg = *payload.ErrorStr
	}
	return l.logGeneration(ctx, payload, "ERROR", statusMsg)
}

// LogFallbackEvent logs a fallback event as a span.
func (l *LangfuseCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	// Fallback events are logged as trace updates
	return nil
}

// Shutdown flushes remaining events and stops the callback.
func (l *LangfuseCallback) Shutdown(ctx context.Context) error {
	close(l.stopCh)
	l.wg.Wait()
	return l.flush()
}

// logGeneration logs a generation event to Langfuse.
func (l *LangfuseCallback) logGeneration(ctx context.Context, payload *StandardLoggingPayload, level, statusMsg string) error {
	traceID := l.getOrCreateTraceID(payload)
	generationID := uuid.New().String()

	gen := langfuseGeneration{
		ID:        generationID,
		TraceID:   traceID,
		Name:      fmt.Sprintf("%s-%s", payload.CallType, payload.Model),
		Model:     payload.Model,
		StartTime: payload.StartTime,
		EndTime:   payload.EndTime,
		Level:     level,
	}

	// Model parameters
	if payload.ModelParameters != nil {
		gen.ModelParameters = payload.ModelParameters
	}

	// Input (with optional masking)
	if !l.config.MaskInput {
		gen.Input = map[string]interface{}{
			"messages": payload.Messages,
		}
	} else {
		gen.Input = "redacted-by-llmux"
	}

	// Output (with optional masking)
	if !l.config.MaskOutput {
		gen.Output = payload.Response
	} else {
		gen.Output = "redacted-by-llmux"
	}

	// Usage
	gen.Usage = &langfuseUsage{
		PromptTokens:     payload.PromptTokens,
		CompletionTokens: payload.CompletionTokens,
		TotalTokens:      payload.TotalTokens,
		TotalCost:        payload.ResponseCost,
	}

	// TTFT for streaming
	if payload.CompletionStartTime != nil {
		gen.CompletionStartTime = payload.CompletionStartTime
	}

	// Status message for errors
	if statusMsg != "" {
		gen.StatusMessage = statusMsg
	}

	// Metadata
	gen.Metadata = l.buildMetadata(payload)
	gen.Metadata["response_cost"] = payload.ResponseCost

	l.enqueue(langfuseEvent{Type: "generation-create", Body: gen})

	// Update trace output
	traceUpdate := map[string]interface{}{
		"id": traceID,
	}
	if !l.config.MaskOutput {
		traceUpdate["output"] = payload.Response
	} else {
		traceUpdate["output"] = "redacted-by-llmux"
	}
	l.enqueue(langfuseEvent{Type: "trace-create", Body: traceUpdate})

	return nil
}

// getOrCreateTraceID gets trace ID from metadata or creates a new one.
func (l *LangfuseCallback) getOrCreateTraceID(payload *StandardLoggingPayload) string {
	// Check metadata for existing trace_id
	if payload.Metadata != nil {
		if traceID, ok := payload.Metadata["trace_id"].(string); ok && traceID != "" {
			return traceID
		}
	}
	// Use request ID as trace ID
	if payload.RequestID != "" {
		return payload.RequestID
	}
	return uuid.New().String()
}

// buildTags builds tags for Langfuse from payload.
func (l *LangfuseCallback) buildTags(payload *StandardLoggingPayload) []string {
	tags := make([]string, 0)

	// Add model tag
	tags = append(tags, fmt.Sprintf("model:%s", payload.Model))

	// Add provider tag
	if payload.APIProvider != "" {
		tags = append(tags, fmt.Sprintf("provider:%s", payload.APIProvider))
	}

	// Add team tag
	if payload.Team != nil {
		tags = append(tags, fmt.Sprintf("team:%s", *payload.Team))
	}

	// Add custom tags from metadata
	if payload.RequestTags != nil {
		tags = append(tags, payload.RequestTags...)
	}

	return tags
}

// buildMetadata builds metadata map for Langfuse.
func (l *LangfuseCallback) buildMetadata(payload *StandardLoggingPayload) map[string]interface{} {
	meta := make(map[string]interface{})

	meta["api_provider"] = payload.APIProvider
	meta["api_base"] = payload.APIBase
	meta["requested_model"] = payload.RequestedModel

	if payload.Team != nil {
		meta["team"] = *payload.Team
	}
	if payload.TeamAlias != nil {
		meta["team_alias"] = *payload.TeamAlias
	}
	if payload.User != nil {
		meta["user"] = *payload.User
	}
	if payload.APIKeyAlias != nil {
		meta["api_key_alias"] = *payload.APIKeyAlias
	}
	if payload.ModelGroup != nil {
		meta["model_group"] = *payload.ModelGroup
	}
	if payload.ModelID != nil {
		meta["model_id"] = *payload.ModelID
	}
	if payload.CacheHit != nil {
		meta["cache_hit"] = *payload.CacheHit
	}

	// Merge custom metadata
	if payload.Metadata != nil {
		for k, v := range payload.Metadata {
			// Skip internal keys
			if k == "trace_id" || k == "session_id" {
				continue
			}
			meta[k] = v
		}
	}

	return meta
}

// enqueue adds an event to the queue.
func (l *LangfuseCallback) enqueue(event langfuseEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.eventQueue = append(l.eventQueue, event)

	// Flush if batch size reached
	if len(l.eventQueue) >= l.config.BatchSize {
		go l.flush()
	}
}

// flushLoop periodically flushes events.
func (l *LangfuseCallback) flushLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.flush()
		case <-l.stopCh:
			return
		}
	}
}

// flush sends queued events to Langfuse.
func (l *LangfuseCallback) flush() error {
	l.mu.Lock()
	if len(l.eventQueue) == 0 {
		l.mu.Unlock()
		return nil
	}

	events := l.eventQueue
	l.eventQueue = make([]langfuseEvent, 0, l.config.BatchSize)
	l.mu.Unlock()

	// Build batch request
	req := langfuseBatchRequest{
		Batch: events,
	}
	req.Metadata.SDKIntegration = "llmux"
	req.Metadata.SDKVersion = "0.1.0"

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("langfuse: failed to marshal batch: %w", err)
	}

	// Send to Langfuse
	url := fmt.Sprintf("%s/api/public/ingestion", l.config.Host)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("langfuse: failed to create request: %w", err)
	}

	httpReq.SetBasicAuth(l.config.PublicKey, l.config.SecretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("langfuse: failed to send batch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("langfuse: batch ingestion failed with status %d", resp.StatusCode)
	}

	return nil
}
