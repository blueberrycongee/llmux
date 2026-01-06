// Package observability provides OpenTelemetry Logs integration.
// This implements semantic logging with trace correlation.
//
// Reference: https://opentelemetry.io/docs/specs/otel/logs/
package observability

import (
	"context"
	"github.com/goccy/go-json"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// OTelLogsConfig contains configuration for OpenTelemetry Logs.
type OTelLogsConfig struct {
	Enabled      bool
	Endpoint     string
	ExporterType ExporterType
	ServiceName  string
	Insecure     bool
	Headers      map[string]string
}

// DefaultOTelLogsConfig returns sensible defaults.
func DefaultOTelLogsConfig() OTelLogsConfig {
	return OTelLogsConfig{
		Enabled:      os.Getenv("LLMUX_OTEL_LOGS_ENABLED") == "true",
		Endpoint:     os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"),
		ExporterType: ExporterGRPC,
		ServiceName:  "llmux",
		Insecure:     true,
		Headers:      make(map[string]string),
	}
}

// OTelLogsProvider wraps the OpenTelemetry logger provider.
type OTelLogsProvider struct {
	provider *sdklog.LoggerProvider
	logger   log.Logger
}

// InitOTelLogs initializes OpenTelemetry Logs.
func InitOTelLogs(ctx context.Context, cfg OTelLogsConfig) (*OTelLogsProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Create exporter
	var exporter sdklog.Exporter
	var err error

	switch cfg.ExporterType {
	case ExporterHTTP:
		exporter, err = createHTTPLogExporter(ctx, cfg)
	default:
		exporter, err = createGRPCLogExporter(ctx, cfg)
	}
	if err != nil {
		return nil, err
	}

	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			attribute.String("gen_ai.system", "llmux"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create logger provider
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	global.SetLoggerProvider(provider)
	logger := provider.Logger("llmux")

	return &OTelLogsProvider{
		provider: provider,
		logger:   logger,
	}, nil
}

// Logger returns the logger instance.
func (o *OTelLogsProvider) Logger() log.Logger {
	return o.logger
}

// Shutdown gracefully shuts down the logger provider.
func (o *OTelLogsProvider) Shutdown(ctx context.Context) error {
	if o == nil || o.provider == nil {
		return nil
	}
	return o.provider.Shutdown(ctx)
}

// createGRPCLogExporter creates an OTLP gRPC log exporter.
func createGRPCLogExporter(ctx context.Context, cfg OTelLogsConfig) (sdklog.Exporter, error) {
	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploggrpc.WithHeaders(cfg.Headers))
	}
	return otlploggrpc.New(ctx, opts...)
}

// createHTTPLogExporter creates an OTLP HTTP log exporter.
func createHTTPLogExporter(ctx context.Context, cfg OTelLogsConfig) (sdklog.Exporter, error) {
	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
	}
	return otlploghttp.New(ctx, opts...)
}

// OTelLogsCallback implements Callback for OpenTelemetry Logs.
type OTelLogsCallback struct {
	provider *OTelLogsProvider
}

// NewOTelLogsCallback creates a new OpenTelemetry Logs callback.
func NewOTelLogsCallback(provider *OTelLogsProvider) *OTelLogsCallback {
	return &OTelLogsCallback{provider: provider}
}

// Name returns the callback name.
func (o *OTelLogsCallback) Name() string {
	return "otel_logs"
}

// LogPreAPICall logs pre-request events.
func (o *OTelLogsCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	o.emitLog(ctx, "llm.request.start", log.SeverityInfo, payload, nil)
	return nil
}

// LogPostAPICall logs post-request events.
func (o *OTelLogsCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogStreamEvent logs streaming events.
func (o *OTelLogsCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	// Don't log every chunk to avoid spam
	return nil
}

// LogSuccessEvent logs successful requests.
func (o *OTelLogsCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	o.emitLog(ctx, "llm.request.success", log.SeverityInfo, payload, nil)
	return nil
}

// LogFailureEvent logs failed requests.
func (o *OTelLogsCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	o.emitLog(ctx, "llm.request.failure", log.SeverityError, payload, err)
	return nil
}

// LogFallbackEvent logs fallback events.
func (o *OTelLogsCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	severity := log.SeverityInfo
	if !success {
		severity = log.SeverityWarn
	}

	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetSeverity(severity)
	record.SetBody(log.StringValue("llm.fallback"))

	record.AddAttributes(
		log.String("original_model", originalModel),
		log.String("fallback_model", fallbackModel),
		log.Bool("success", success),
	)

	if err != nil {
		record.AddAttributes(log.String("error.message", err.Error()))
	}

	o.provider.Logger().Emit(ctx, record)
	return nil
}

// Shutdown gracefully shuts down the callback.
func (o *OTelLogsCallback) Shutdown(ctx context.Context) error {
	return o.provider.Shutdown(ctx)
}

// emitLog emits a log record.
func (o *OTelLogsCallback) emitLog(ctx context.Context, eventName string, severity log.Severity, payload *StandardLoggingPayload, err error) {
	if o.provider == nil {
		return
	}

	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetSeverity(severity)
	record.SetBody(log.StringValue(eventName))

	// Add gen_ai attributes
	record.AddAttributes(
		log.String("gen_ai.system", payload.APIProvider),
		log.String("gen_ai.request.model", payload.Model),
		log.String("gen_ai.operation.name", string(payload.CallType)),
		log.String("llmux.request_id", payload.RequestID),
	)

	// Add token usage
	record.AddAttributes(
		log.Int("gen_ai.usage.input_tokens", payload.PromptTokens),
		log.Int("gen_ai.usage.output_tokens", payload.CompletionTokens),
	)

	// Add cost
	record.AddAttributes(
		log.Float64("llmux.response_cost", payload.ResponseCost),
	)

	// Add duration
	duration := payload.EndTime.Sub(payload.StartTime).Milliseconds()
	record.AddAttributes(log.Int64("llmux.duration_ms", duration))

	// Add TTFT if available
	if payload.CompletionStartTime != nil {
		ttft := payload.CompletionStartTime.Sub(payload.StartTime).Milliseconds()
		record.AddAttributes(log.Int64("llmux.ttft_ms", ttft))
	}

	// Add optional attributes
	if payload.Team != nil {
		record.AddAttributes(log.String("llmux.team", *payload.Team))
	}
	if payload.User != nil {
		record.AddAttributes(log.String("llmux.user", *payload.User))
	}
	if payload.EndUser != nil {
		record.AddAttributes(log.String("llmux.end_user", *payload.EndUser))
	}

	// Add error info
	if err != nil {
		record.AddAttributes(log.String("error.message", err.Error()))
	}
	if payload.ErrorStr != nil {
		record.AddAttributes(log.String("error.message", *payload.ErrorStr))
	}
	if payload.ExceptionClass != nil {
		record.AddAttributes(log.String("error.type", *payload.ExceptionClass))
	}

	// Add trace context if available
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		record.AddAttributes(
			log.String("trace_id", span.SpanContext().TraceID().String()),
			log.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	// Add full payload as JSON for detailed logging
	if payloadJSON, jsonErr := json.Marshal(payload); jsonErr == nil {
		record.AddAttributes(log.String("llmux.payload", string(payloadJSON)))
	}

	o.provider.Logger().Emit(ctx, record)
}
