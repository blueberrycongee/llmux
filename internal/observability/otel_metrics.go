// Package observability provides OpenTelemetry Metrics integration.
// This implements the gen_ai semantic conventions for metrics.
//
// Reference: https://opentelemetry.io/docs/specs/semconv/gen-ai/
package observability

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// OTelMetricsConfig contains configuration for OpenTelemetry Metrics.
type OTelMetricsConfig struct {
	Enabled      bool
	Endpoint     string
	ExporterType ExporterType
	ServiceName  string
	Insecure     bool
	Headers      map[string]string
	// ExportInterval is the interval between metric exports
	ExportInterval time.Duration
}

// DefaultOTelMetricsConfig returns sensible defaults.
func DefaultOTelMetricsConfig() OTelMetricsConfig {
	return OTelMetricsConfig{
		Enabled:        os.Getenv("LLMUX_OTEL_METRICS_ENABLED") == "true",
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"),
		ExporterType:   ExporterGRPC,
		ServiceName:    "llmux",
		Insecure:       true,
		Headers:        make(map[string]string),
		ExportInterval: 60 * time.Second,
	}
}

// OTelMetricsProvider wraps the OpenTelemetry meter provider.
type OTelMetricsProvider struct {
	provider *sdkmetric.MeterProvider
	meter    metric.Meter

	// Gen AI metrics
	operationDuration  metric.Float64Histogram
	tokenUsage         metric.Int64Counter
	tokenCost          metric.Float64Counter
	timeToFirstToken   metric.Float64Histogram
	timePerOutputToken metric.Float64Histogram
	requestCount       metric.Int64Counter
	errorCount         metric.Int64Counter
}

// InitOTelMetrics initializes OpenTelemetry Metrics.
func InitOTelMetrics(ctx context.Context, cfg OTelMetricsConfig) (*OTelMetricsProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Create exporter
	var exporter sdkmetric.Exporter
	var err error

	switch cfg.ExporterType {
	case ExporterHTTP:
		exporter, err = createHTTPMetricExporter(ctx, cfg)
	default:
		exporter, err = createGRPCMetricExporter(ctx, cfg)
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

	// Create meter provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.ExportInterval)),
		),
	)

	otel.SetMeterProvider(provider)
	meter := provider.Meter("llmux")

	// Create metrics
	omp := &OTelMetricsProvider{
		provider: provider,
		meter:    meter,
	}

	if err := omp.initMetrics(); err != nil {
		return nil, err
	}

	return omp, nil
}

// initMetrics initializes all gen_ai metrics.
func (o *OTelMetricsProvider) initMetrics() error {
	var err error

	// gen_ai.client.operation.duration - Duration of GenAI operations
	o.operationDuration, err = o.meter.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithDescription("Duration of GenAI operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// gen_ai.client.token.usage - Token usage count
	o.tokenUsage, err = o.meter.Int64Counter(
		"gen_ai.client.token.usage",
		metric.WithDescription("Number of tokens used"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return err
	}

	// gen_ai.client.token.cost - Token cost
	o.tokenCost, err = o.meter.Float64Counter(
		"gen_ai.client.token.cost",
		metric.WithDescription("Cost of tokens used"),
		metric.WithUnit("{currency}"),
	)
	if err != nil {
		return err
	}

	// gen_ai.client.response.time_to_first_token - TTFT
	o.timeToFirstToken, err = o.meter.Float64Histogram(
		"gen_ai.client.response.time_to_first_token",
		metric.WithDescription("Time to first token"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// gen_ai.client.response.time_per_output_token
	o.timePerOutputToken, err = o.meter.Float64Histogram(
		"gen_ai.client.response.time_per_output_token",
		metric.WithDescription("Time per output token"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Request count
	o.requestCount, err = o.meter.Int64Counter(
		"gen_ai.client.request.count",
		metric.WithDescription("Number of GenAI requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Error count
	o.errorCount, err = o.meter.Int64Counter(
		"gen_ai.client.error.count",
		metric.WithDescription("Number of GenAI errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// RecordRequest records metrics for a request.
func (o *OTelMetricsProvider) RecordRequest(ctx context.Context, payload *StandardLoggingPayload) {
	if o == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", payload.APIProvider),
		attribute.String("gen_ai.request.model", payload.Model),
		attribute.String("gen_ai.operation.name", string(payload.CallType)),
	}

	// Add optional attributes
	if payload.Team != nil {
		attrs = append(attrs, attribute.String("llmux.team", *payload.Team))
	}
	if payload.User != nil {
		attrs = append(attrs, attribute.String("llmux.user", *payload.User))
	}

	// Record operation duration
	duration := payload.EndTime.Sub(payload.StartTime).Seconds()
	o.operationDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

	// Record token usage
	inputAttrs := append([]attribute.KeyValue{}, attrs...)
	inputAttrs = append(inputAttrs, attribute.String("gen_ai.token.type", "input"))
	o.tokenUsage.Add(ctx, int64(payload.PromptTokens), metric.WithAttributes(inputAttrs...))

	outputAttrs := append([]attribute.KeyValue{}, attrs...)
	outputAttrs = append(outputAttrs, attribute.String("gen_ai.token.type", "output"))
	o.tokenUsage.Add(ctx, int64(payload.CompletionTokens), metric.WithAttributes(outputAttrs...))

	// Record cost
	o.tokenCost.Add(ctx, payload.ResponseCost, metric.WithAttributes(attrs...))

	// Record TTFT if available
	if payload.CompletionStartTime != nil {
		ttft := payload.CompletionStartTime.Sub(payload.StartTime).Seconds()
		o.timeToFirstToken.Record(ctx, ttft, metric.WithAttributes(attrs...))

		// Calculate time per output token
		if payload.CompletionTokens > 0 {
			totalOutputTime := payload.EndTime.Sub(*payload.CompletionStartTime).Seconds()
			timePerToken := totalOutputTime / float64(payload.CompletionTokens)
			o.timePerOutputToken.Record(ctx, timePerToken, metric.WithAttributes(attrs...))
		}
	}

	// Record request count
	o.requestCount.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record error if failed
	if payload.Status == RequestStatusFailure {
		errorAttrs := attrs
		if payload.ExceptionClass != nil {
			errorAttrs = append(errorAttrs, attribute.String("error.type", *payload.ExceptionClass))
		}
		o.errorCount.Add(ctx, 1, metric.WithAttributes(errorAttrs...))
	}
}

// Shutdown gracefully shuts down the metrics provider.
func (o *OTelMetricsProvider) Shutdown(ctx context.Context) error {
	if o == nil || o.provider == nil {
		return nil
	}
	return o.provider.Shutdown(ctx)
}

// createGRPCMetricExporter creates an OTLP gRPC metric exporter.
func createGRPCMetricExporter(ctx context.Context, cfg OTelMetricsConfig) (sdkmetric.Exporter, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}
	return otlpmetricgrpc.New(ctx, opts...)
}

// createHTTPMetricExporter creates an OTLP HTTP metric exporter.
func createHTTPMetricExporter(ctx context.Context, cfg OTelMetricsConfig) (sdkmetric.Exporter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}
	return otlpmetrichttp.New(ctx, opts...)
}

// OTelMetricsCallback implements Callback for OpenTelemetry Metrics.
type OTelMetricsCallback struct {
	provider *OTelMetricsProvider
}

// NewOTelMetricsCallback creates a new OpenTelemetry Metrics callback.
func NewOTelMetricsCallback(provider *OTelMetricsProvider) *OTelMetricsCallback {
	return &OTelMetricsCallback{provider: provider}
}

// Name returns the callback name.
func (o *OTelMetricsCallback) Name() string {
	return "otel_metrics"
}

// LogPreAPICall is a no-op for metrics.
func (o *OTelMetricsCallback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogPostAPICall is a no-op for metrics.
func (o *OTelMetricsCallback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogStreamEvent is a no-op for metrics.
func (o *OTelMetricsCallback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	return nil
}

// LogSuccessEvent records success metrics.
func (o *OTelMetricsCallback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	o.provider.RecordRequest(ctx, payload)
	return nil
}

// LogFailureEvent records failure metrics.
func (o *OTelMetricsCallback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	o.provider.RecordRequest(ctx, payload)
	return nil
}

// LogFallbackEvent records fallback metrics.
func (o *OTelMetricsCallback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	// Fallback metrics are handled by Prometheus
	return nil
}

// Shutdown gracefully shuts down the callback.
func (o *OTelMetricsCallback) Shutdown(ctx context.Context) error {
	return o.provider.Shutdown(ctx)
}
