// Package observability provides Datadog LLM Observability integration.
// This implements logging to Datadog's LLM Observability service.
//
// API Reference: https://docs.datadoghq.com/llm_observability/setup/api/
package observability

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/google/uuid"
)

// DDLLMObsSpanKind represents the type of LLM operation.
type DDLLMObsSpanKind string

const (
	DDSpanKindLLM       DDLLMObsSpanKind = "llm"
	DDSpanKindTool      DDLLMObsSpanKind = "tool"
	DDSpanKindTask      DDLLMObsSpanKind = "task"
	DDSpanKindEmbedding DDLLMObsSpanKind = "embedding"
	DDSpanKindRetrieval DDLLMObsSpanKind = "retrieval"
)

// DDLLMObsInputMeta represents input metadata.
type DDLLMObsInputMeta struct {
	Messages []map[string]any `json:"messages,omitempty"`
}

// DDLLMObsOutputMeta represents output metadata.
type DDLLMObsOutputMeta struct {
	Messages []map[string]any `json:"messages,omitempty"`
}

// DDLLMObsError represents error information.
type DDLLMObsError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Stack   string `json:"stack,omitempty"`
}

// DDLLMObsMeta represents span metadata.
type DDLLMObsMeta struct {
	Kind     DDLLMObsSpanKind   `json:"kind"`
	Input    DDLLMObsInputMeta  `json:"input,omitempty"`
	Output   DDLLMObsOutputMeta `json:"output,omitempty"`
	Metadata map[string]any     `json:"metadata,omitempty"`
	Error    *DDLLMObsError     `json:"error,omitempty"`
}

// DDLLMObsMetrics represents LLM metrics.
type DDLLMObsMetrics struct {
	InputTokens      float64 `json:"input_tokens,omitempty"`
	OutputTokens     float64 `json:"output_tokens,omitempty"`
	TotalTokens      float64 `json:"total_tokens,omitempty"`
	TotalCost        float64 `json:"total_cost,omitempty"`
	TimeToFirstToken float64 `json:"time_to_first_token,omitempty"`
}

// DDLLMObsSpan represents a single LLM observation span.
type DDLLMObsSpan struct {
	ParentID string          `json:"parent_id"`
	TraceID  string          `json:"trace_id"`
	SpanID   string          `json:"span_id"`
	Name     string          `json:"name"`
	Meta     DDLLMObsMeta    `json:"meta"`
	StartNs  int64           `json:"start_ns"`
	Duration int64           `json:"duration"`
	Metrics  DDLLMObsMetrics `json:"metrics"`
	Status   string          `json:"status"`
	Tags     []string        `json:"tags,omitempty"`
	APMID    string          `json:"apm_id,omitempty"`
}

// DDLLMObsSpanAttributes represents the span attributes wrapper.
type DDLLMObsSpanAttributes struct {
	MLApp string         `json:"ml_app"`
	Tags  []string       `json:"tags,omitempty"`
	Spans []DDLLMObsSpan `json:"spans"`
}

// DDLLMObsIntakePayload represents the intake payload.
type DDLLMObsIntakePayload struct {
	Type       string                 `json:"type"`
	Attributes DDLLMObsSpanAttributes `json:"attributes"`
}

// DDLLMObsConfig contains configuration for Datadog LLM Observability.
type DDLLMObsConfig struct {
	// APIKey is the Datadog API key
	APIKey string `yaml:"api_key" json:"api_key"`
	// Site is the Datadog site (e.g., "us5.datadoghq.com")
	Site string `yaml:"site" json:"site"`
	// MLApp is the ML application name
	MLApp string `yaml:"ml_app" json:"ml_app"`
	// Tags are additional tags to add to all spans
	Tags []string `yaml:"tags" json:"tags"`
	// BatchSize is the maximum number of spans to batch
	BatchSize int `yaml:"batch_size" json:"batch_size"`
	// FlushInterval is the interval to flush spans
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	// TurnOffMessageLogging disables logging of request/response messages
	TurnOffMessageLogging bool `yaml:"turn_off_message_logging" json:"turn_off_message_logging"`
}

// DefaultDDLLMObsConfig returns configuration from environment variables.
func DefaultDDLLMObsConfig() DDLLMObsConfig {
	cfg := DDLLMObsConfig{
		APIKey:                os.Getenv("DD_API_KEY"),
		Site:                  os.Getenv("DD_SITE"),
		MLApp:                 os.Getenv("DD_LLMOBS_ML_APP"),
		BatchSize:             100,
		FlushInterval:         5 * time.Second,
		TurnOffMessageLogging: envBool("LLMUX_DD_LLMOBS_TURN_OFF_MESSAGE_LOGGING", true),
	}

	if cfg.MLApp == "" {
		cfg.MLApp = "llmux"
	}

	// Parse tags from environment
	if tags := os.Getenv("DD_TAGS"); tags != "" {
		cfg.Tags = strings.Split(tags, ",")
	}

	return cfg
}

// DDLLMObsCallback implements Callback for Datadog LLM Observability.
type DDLLMObsCallback struct {
	config    DDLLMObsConfig
	intakeURL string
	client    *http.Client
	spanQueue []DDLLMObsSpan
	mu        sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewDDLLMObsCallback creates a new Datadog LLM Observability callback.
func NewDDLLMObsCallback(cfg DDLLMObsConfig) (*DDLLMObsCallback, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("DD_API_KEY is required")
	}
	if cfg.Site == "" {
		return nil, fmt.Errorf("DD_SITE is required")
	}

	cb := &DDLLMObsCallback{
		config:    cfg,
		intakeURL: fmt.Sprintf("https://api.%s/api/intake/llm-obs/v1/trace/spans", cfg.Site),
		client:    &http.Client{Timeout: 30 * time.Second},
		spanQueue: make([]DDLLMObsSpan, 0, cfg.BatchSize),
		stopCh:    make(chan struct{}),
	}

	// Start background flush goroutine
	cb.wg.Add(1)
	go cb.periodicFlush()

	return cb, nil
}

// Name returns the callback name.
func (d *DDLLMObsCallback) Name() string {
	return "datadog_llm_obs"
}

// LogPreAPICall logs pre-request events.
func (d *DDLLMObsCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogPostAPICall logs post-request events.
func (d *DDLLMObsCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogStreamEvent logs streaming events.
func (d *DDLLMObsCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	return nil
}

// LogSuccessEvent logs successful requests.
func (d *DDLLMObsCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	span := d.createSpan(payload, "ok")
	d.enqueue(span)
	return nil
}

// LogFailureEvent logs failed requests.
func (d *DDLLMObsCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	span := d.createSpan(payload, "error")
	d.enqueue(span)
	return nil
}

// LogFallbackEvent logs fallback events.
func (d *DDLLMObsCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	status := "ok"
	if !success {
		status = "error"
	}

	span := DDLLMObsSpan{
		ParentID: "undefined",
		TraceID:  uuid.New().String(),
		SpanID:   uuid.New().String(),
		Name:     "llmux_fallback",
		Meta: DDLLMObsMeta{
			Kind: DDSpanKindTask,
			Metadata: map[string]any{
				"original_model": originalModel,
				"fallback_model": fallbackModel,
				"success":        success,
			},
		},
		StartNs:  time.Now().UnixNano(),
		Duration: 0,
		Status:   status,
		Tags:     d.buildTags(nil),
	}

	if err != nil {
		span.Meta.Error = &DDLLMObsError{
			Message: err.Error(),
		}
	}

	d.enqueue(span)
	return nil
}

// Shutdown gracefully shuts down the callback.
func (d *DDLLMObsCallback) Shutdown(ctx context.Context) error {
	close(d.stopCh)

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	return d.flush()
}

// createSpan creates a DDLLMObsSpan from StandardLoggingPayload.
func (d *DDLLMObsCallback) createSpan(payload *StandardLoggingPayload, status string) DDLLMObsSpan {
	// Determine span kind based on call type
	kind := d.getSpanKind(payload.CallType)

	// Build input messages
	var inputMessages []map[string]any
	if !d.config.TurnOffMessageLogging && payload.Messages != nil {
		inputMessages = d.convertMessages(payload.Messages)
	}

	// Build output messages
	var outputMessages []map[string]any
	if !d.config.TurnOffMessageLogging && payload.Response != nil {
		outputMessages = d.extractResponseMessages(payload.Response)
	}

	// Build metadata
	metadata := d.buildMetadata(payload)

	// Build error info if failed
	var errorInfo *DDLLMObsError
	if payload.Status == RequestStatusFailure && payload.ErrorStr != nil {
		errorInfo = &DDLLMObsError{
			Message: *payload.ErrorStr,
		}
		if payload.ExceptionClass != nil {
			errorInfo.Type = *payload.ExceptionClass
		}
	}

	// Calculate TTFT
	var ttft float64
	if payload.CompletionStartTime != nil {
		ttft = payload.CompletionStartTime.Sub(payload.StartTime).Seconds()
	}

	// Get trace/span IDs from metadata or generate new ones
	traceID := payload.RequestID
	if traceID == "" {
		traceID = uuid.New().String()
	}
	spanID := uuid.New().String()

	// Get parent ID from metadata
	parentID := "undefined"
	if payload.Metadata != nil {
		if pid, ok := payload.Metadata["parent_id"].(string); ok && pid != "" {
			parentID = pid
		}
	}

	return DDLLMObsSpan{
		ParentID: parentID,
		TraceID:  traceID,
		SpanID:   spanID,
		Name:     fmt.Sprintf("llmux_%s", payload.CallType),
		Meta: DDLLMObsMeta{
			Kind:     kind,
			Input:    DDLLMObsInputMeta{Messages: inputMessages},
			Output:   DDLLMObsOutputMeta{Messages: outputMessages},
			Metadata: metadata,
			Error:    errorInfo,
		},
		StartNs:  payload.StartTime.UnixNano(),
		Duration: payload.EndTime.Sub(payload.StartTime).Nanoseconds(),
		Metrics: DDLLMObsMetrics{
			InputTokens:      float64(payload.PromptTokens),
			OutputTokens:     float64(payload.CompletionTokens),
			TotalTokens:      float64(payload.TotalTokens),
			TotalCost:        payload.ResponseCost,
			TimeToFirstToken: ttft,
		},
		Status: status,
		Tags:   d.buildTags(payload),
	}
}

// getSpanKind maps CallType to DDLLMObsSpanKind.
func (d *DDLLMObsCallback) getSpanKind(callType CallType) DDLLMObsSpanKind {
	switch callType {
	case CallTypeEmbedding:
		return DDSpanKindEmbedding
	case CallTypeCompletion, CallTypeChatCompletion:
		return DDSpanKindLLM
	case CallTypeImageGen, CallTypeAudioTranscr, CallTypeModeration:
		return DDSpanKindTask
	default:
		return DDSpanKindLLM
	}
}

// convertMessages converts messages to the expected format.
func (d *DDLLMObsCallback) convertMessages(messages any) []map[string]any {
	switch v := messages.(type) {
	case []map[string]any:
		return v
	case []any:
		result := make([]map[string]any, 0, len(v))
		for _, msg := range v {
			if m, ok := msg.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result
	default:
		return nil
	}
}

// extractResponseMessages extracts messages from response.
func (d *DDLLMObsCallback) extractResponseMessages(response any) []map[string]any {
	respMap, ok := response.(map[string]any)
	if !ok {
		return nil
	}

	choices, ok := respMap["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return nil
	}

	message, ok := choice["message"].(map[string]any)
	if !ok {
		return nil
	}

	return []map[string]any{message}
}

// buildMetadata builds span metadata.
func (d *DDLLMObsCallback) buildMetadata(payload *StandardLoggingPayload) map[string]any {
	metadata := map[string]any{
		"model_name":     payload.Model,
		"model_provider": payload.APIProvider,
		"id":             payload.ID,
		"request_id":     payload.RequestID,
	}

	if payload.CacheHit != nil {
		metadata["cache_hit"] = *payload.CacheHit
	}
	if payload.ModelGroup != nil {
		metadata["model_group"] = *payload.ModelGroup
	}

	// Add latency metrics
	latencyMetrics := map[string]any{}
	if payload.CompletionStartTime != nil {
		ttftMs := payload.CompletionStartTime.Sub(payload.StartTime).Milliseconds()
		latencyMetrics["time_to_first_token_ms"] = ttftMs
	}
	if len(latencyMetrics) > 0 {
		metadata["latency_metrics"] = latencyMetrics
	}

	// Add spend metrics
	spendMetrics := map[string]any{
		"response_cost": payload.ResponseCost,
	}
	metadata["spend_metrics"] = spendMetrics

	return metadata
}

// buildTags builds tags for the span.
func (d *DDLLMObsCallback) buildTags(payload *StandardLoggingPayload) []string {
	tags := make([]string, 0, len(d.config.Tags)+5)
	tags = append(tags, d.config.Tags...)

	if payload != nil {
		tags = append(tags,
			fmt.Sprintf("model:%s", payload.Model),
			fmt.Sprintf("provider:%s", payload.APIProvider),
		)

		if payload.Team != nil {
			tags = append(tags, fmt.Sprintf("team:%s", *payload.Team))
		}
		if payload.User != nil {
			tags = append(tags, fmt.Sprintf("user:%s", *payload.User))
		}
	}

	return tags
}

// enqueue adds a span to the queue.
func (d *DDLLMObsCallback) enqueue(span DDLLMObsSpan) {
	d.mu.Lock()
	d.spanQueue = append(d.spanQueue, span)
	shouldFlush := len(d.spanQueue) >= d.config.BatchSize
	d.mu.Unlock()

	if shouldFlush {
		go func() {
			_ = d.flush()
		}()
	}
}

// periodicFlush periodically flushes the span queue.
func (d *DDLLMObsCallback) periodicFlush() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := d.flush(); err != nil {
				// Log error but continue flushing
				_ = err
			}
		case <-d.stopCh:
			return
		}
	}
}

// flush sends all queued spans to Datadog.
func (d *DDLLMObsCallback) flush() error {
	d.mu.Lock()
	if len(d.spanQueue) == 0 {
		d.mu.Unlock()
		return nil
	}
	spans := d.spanQueue
	d.spanQueue = make([]DDLLMObsSpan, 0, d.config.BatchSize)
	d.mu.Unlock()

	return d.sendBatch(spans)
}

// sendBatch sends a batch of spans to Datadog LLM Observability.
func (d *DDLLMObsCallback) sendBatch(spans []DDLLMObsSpan) error {
	payload := map[string]any{
		"data": DDLLMObsIntakePayload{
			Type: "span",
			Attributes: DDLLMObsSpanAttributes{
				MLApp: d.config.MLApp,
				Tags:  d.config.Tags,
				Spans: spans,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal spans: %w", err)
	}

	// Create request with context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.intakeURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", d.config.APIKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send spans: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
