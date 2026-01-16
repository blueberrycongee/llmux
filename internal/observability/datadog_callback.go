// Package observability provides a Datadog callback implementation for LLM observability.
// This follows LiteLLM's DataDog integration pattern for logging to Datadog's API.
//
// Reference: https://docs.datadoghq.com/api/latest/logs/
package observability

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
)

// DatadogStatus represents the log status level.
type DatadogStatus string

const (
	DatadogStatusInfo    DatadogStatus = "info"
	DatadogStatusWarning DatadogStatus = "warning"
	DatadogStatusError   DatadogStatus = "error"
)

// DatadogPayload represents a single log entry for Datadog.
type DatadogPayload struct {
	DDSource  string        `json:"ddsource"`
	DDTags    string        `json:"ddtags"`
	Hostname  string        `json:"hostname"`
	Message   string        `json:"message"`
	Service   string        `json:"service"`
	Status    DatadogStatus `json:"status"`
	Timestamp int64         `json:"timestamp,omitempty"`
	TraceID   string        `json:"dd.trace_id,omitempty"`
	SpanID    string        `json:"dd.span_id,omitempty"`
}

// DatadogConfig contains configuration for Datadog integration.
type DatadogConfig struct {
	// APIKey is the Datadog API key (required for direct API, optional for agent)
	APIKey string `yaml:"api_key" json:"api_key"`
	// Site is the Datadog site (e.g., "us5.datadoghq.com")
	Site string `yaml:"site" json:"site"`
	// AgentHost is the hostname of the Datadog agent (optional, uses agent instead of direct API)
	AgentHost string `yaml:"agent_host" json:"agent_host"`
	// AgentPort is the port of the Datadog agent (default: 10518)
	AgentPort string `yaml:"agent_port" json:"agent_port"`
	// Service is the service name for logs
	Service string `yaml:"service" json:"service"`
	// Source is the source name for logs
	Source string `yaml:"source" json:"source"`
	// Hostname is the hostname to report
	Hostname string `yaml:"hostname" json:"hostname"`
	// Tags are additional tags to add to all logs
	Tags []string `yaml:"tags" json:"tags"`
	// BatchSize is the maximum number of logs to batch before sending
	BatchSize int `yaml:"batch_size" json:"batch_size"`
	// FlushInterval is the interval to flush logs
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	// TurnOffMessageLogging disables logging of request/response messages
	TurnOffMessageLogging bool `yaml:"turn_off_message_logging" json:"turn_off_message_logging"`
}

// DefaultDatadogConfig returns configuration from environment variables.
func DefaultDatadogConfig() DatadogConfig {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	cfg := DatadogConfig{
		APIKey:                os.Getenv("DD_API_KEY"),
		Site:                  os.Getenv("DD_SITE"),
		AgentHost:             os.Getenv("LLMUX_DD_AGENT_HOST"),
		AgentPort:             os.Getenv("LLMUX_DD_AGENT_PORT"),
		Service:               os.Getenv("DD_SERVICE"),
		Source:                "llmux",
		Hostname:              hostname,
		BatchSize:             100,
		FlushInterval:         5 * time.Second,
		TurnOffMessageLogging: envBool("LLMUX_DD_TURN_OFF_MESSAGE_LOGGING", true),
	}

	if cfg.AgentPort == "" {
		cfg.AgentPort = "10518"
	}
	if cfg.Service == "" {
		cfg.Service = "llmux"
	}

	// Parse tags from environment
	if tags := os.Getenv("DD_TAGS"); tags != "" {
		cfg.Tags = strings.Split(tags, ",")
	}

	return cfg
}

// DatadogCallback implements Callback for Datadog logging.
type DatadogCallback struct {
	config    DatadogConfig
	intakeURL string
	client    *http.Client
	logQueue  []DatadogPayload
	mu        sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewDatadogCallback creates a new Datadog callback.
func NewDatadogCallback(cfg DatadogConfig) (*DatadogCallback, error) {
	cb := &DatadogCallback{
		config:   cfg,
		client:   &http.Client{Timeout: 30 * time.Second},
		logQueue: make([]DatadogPayload, 0, cfg.BatchSize),
		stopCh:   make(chan struct{}),
	}

	// Configure intake URL
	if cfg.AgentHost != "" {
		// Use Datadog Agent
		cb.intakeURL = fmt.Sprintf("http://%s:%s/api/v2/logs", cfg.AgentHost, cfg.AgentPort)
	} else {
		// Use direct API
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("DD_API_KEY is required when not using Datadog Agent")
		}
		if cfg.Site == "" {
			return nil, fmt.Errorf("DD_SITE is required when not using Datadog Agent")
		}
		cb.intakeURL = fmt.Sprintf("https://http-intake.logs.%s/api/v2/logs", cfg.Site)
	}

	// Start background flush goroutine
	cb.wg.Add(1)
	go cb.periodicFlush()

	return cb, nil
}

// Name returns the callback name.
func (d *DatadogCallback) Name() string {
	return "datadog"
}

// LogPreAPICall logs pre-request events.
func (d *DatadogCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	// Pre-call logging is optional for Datadog
	return nil
}

// LogPostAPICall logs post-request events.
func (d *DatadogCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	// Post-call logging is handled in success/failure events
	return nil
}

// LogStreamEvent logs streaming events.
func (d *DatadogCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	// Stream events are not logged individually to avoid spam
	return nil
}

// LogSuccessEvent logs successful requests.
func (d *DatadogCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	ddPayload := d.createPayload(payload, DatadogStatusInfo)
	d.enqueue(ddPayload)
	return nil
}

// LogFailureEvent logs failed requests.
func (d *DatadogCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	ddPayload := d.createPayload(payload, DatadogStatusError)
	d.enqueue(ddPayload)
	return nil
}

// LogFallbackEvent logs fallback events.
func (d *DatadogCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	status := DatadogStatusInfo
	if !success {
		status = DatadogStatusWarning
	}

	message := map[string]any{
		"event":          "fallback",
		"original_model": originalModel,
		"fallback_model": fallbackModel,
		"success":        success,
	}
	if err != nil {
		message["error"] = err.Error()
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		msgBytes = []byte(`{"error":"failed to marshal message"}`)
	}
	ddPayload := DatadogPayload{
		DDSource:  d.config.Source,
		DDTags:    d.buildTags(nil),
		Hostname:  d.config.Hostname,
		Message:   string(msgBytes),
		Service:   d.config.Service,
		Status:    status,
		Timestamp: time.Now().UnixMilli(),
	}

	d.enqueue(ddPayload)
	return nil
}

// Shutdown gracefully shuts down the callback.
func (d *DatadogCallback) Shutdown(ctx context.Context) error {
	close(d.stopCh)

	// Wait for flush goroutine with timeout
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

	// Final flush
	return d.flush()
}

// createPayload creates a Datadog payload from StandardLoggingPayload.
func (d *DatadogCallback) createPayload(payload *StandardLoggingPayload, status DatadogStatus) DatadogPayload {
	// Create message content
	message := d.buildMessage(payload)
	msgBytes, err := json.Marshal(message)
	if err != nil {
		msgBytes = []byte(`{"error":"failed to marshal message"}`)
	}

	return DatadogPayload{
		DDSource:  d.config.Source,
		DDTags:    d.buildTags(payload),
		Hostname:  d.config.Hostname,
		Message:   string(msgBytes),
		Service:   d.config.Service,
		Status:    status,
		Timestamp: payload.EndTime.UnixMilli(),
	}
}

// buildMessage builds the log message from payload.
func (d *DatadogCallback) buildMessage(payload *StandardLoggingPayload) map[string]any {
	msg := map[string]any{
		"id":                payload.ID,
		"request_id":        payload.RequestID,
		"call_type":         payload.CallType,
		"status":            payload.Status,
		"model":             payload.Model,
		"requested_model":   payload.RequestedModel,
		"api_provider":      payload.APIProvider,
		"api_base":          payload.APIBase,
		"prompt_tokens":     payload.PromptTokens,
		"completion_tokens": payload.CompletionTokens,
		"total_tokens":      payload.TotalTokens,
		"response_cost":     payload.ResponseCost,
		"start_time":        payload.StartTime.Format(time.RFC3339Nano),
		"end_time":          payload.EndTime.Format(time.RFC3339Nano),
		"duration_ms":       payload.EndTime.Sub(payload.StartTime).Milliseconds(),
	}

	// Add optional fields
	if payload.Team != nil {
		msg["team"] = *payload.Team
	}
	if payload.User != nil {
		msg["user"] = *payload.User
	}
	if payload.EndUser != nil {
		msg["end_user"] = *payload.EndUser
	}
	if payload.HashedAPIKey != nil {
		msg["hashed_api_key"] = *payload.HashedAPIKey
	}
	if payload.APIKeyAlias != nil {
		msg["api_key_alias"] = *payload.APIKeyAlias
	}
	if payload.ModelGroup != nil {
		msg["model_group"] = *payload.ModelGroup
	}
	if payload.ModelID != nil {
		msg["model_id"] = *payload.ModelID
	}
	if payload.CacheHit != nil {
		msg["cache_hit"] = *payload.CacheHit
	}
	if payload.ErrorStr != nil {
		msg["error"] = *payload.ErrorStr
	}
	if payload.ExceptionClass != nil {
		msg["exception_class"] = *payload.ExceptionClass
	}

	// TTFT for streaming
	if payload.CompletionStartTime != nil {
		ttftMs := payload.CompletionStartTime.Sub(payload.StartTime).Milliseconds()
		msg["time_to_first_token_ms"] = ttftMs
	}

	// Include messages if not disabled
	if !d.config.TurnOffMessageLogging {
		if payload.Messages != nil {
			msg["messages"] = payload.Messages
		}
		if payload.Response != nil {
			msg["response"] = payload.Response
		}
	}

	// Metadata
	if payload.Metadata != nil {
		msg["metadata"] = payload.Metadata
	}

	return msg
}

// buildTags builds the ddtags string.
func (d *DatadogCallback) buildTags(payload *StandardLoggingPayload) string {
	tags := make([]string, 0, len(d.config.Tags)+10)
	tags = append(tags, d.config.Tags...)

	if payload != nil {
		tags = append(tags,
			fmt.Sprintf("model:%s", payload.Model),
			fmt.Sprintf("provider:%s", payload.APIProvider),
			fmt.Sprintf("status:%s", payload.Status),
		)

		if payload.Team != nil {
			tags = append(tags, fmt.Sprintf("team:%s", *payload.Team))
		}
		if payload.User != nil {
			tags = append(tags, fmt.Sprintf("user:%s", *payload.User))
		}
		if payload.ModelGroup != nil {
			tags = append(tags, fmt.Sprintf("model_group:%s", *payload.ModelGroup))
		}
	}

	return strings.Join(tags, ",")
}

// enqueue adds a payload to the queue.
func (d *DatadogCallback) enqueue(payload DatadogPayload) {
	d.mu.Lock()
	d.logQueue = append(d.logQueue, payload)
	shouldFlush := len(d.logQueue) >= d.config.BatchSize
	d.mu.Unlock()

	if shouldFlush {
		go func() {
			_ = d.flush()
		}()
	}
}

// periodicFlush periodically flushes the log queue.
func (d *DatadogCallback) periodicFlush() {
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

// flush sends all queued logs to Datadog.
func (d *DatadogCallback) flush() error {
	d.mu.Lock()
	if len(d.logQueue) == 0 {
		d.mu.Unlock()
		return nil
	}
	logs := d.logQueue
	d.logQueue = make([]DatadogPayload, 0, d.config.BatchSize)
	d.mu.Unlock()

	return d.sendBatch(logs)
}

// sendBatch sends a batch of logs to Datadog.
func (d *DatadogCallback) sendBatch(logs []DatadogPayload) error {
	// Serialize to JSON
	data, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	// Compress with gzip (recommended by Datadog)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, writeErr := gz.Write(data); writeErr != nil {
		return fmt.Errorf("failed to compress logs: %w", writeErr)
	}
	if closeErr := gz.Close(); closeErr != nil {
		return fmt.Errorf("failed to close gzip writer: %w", closeErr)
	}

	// Create request with context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.intakeURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if d.config.APIKey != "" {
		req.Header.Set("DD-API-KEY", d.config.APIKey)
	}

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
