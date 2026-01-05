// Package observability provides OpenTelemetry tracing and logging utilities.
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TracerName is the name of the tracer used by LLMux.
	TracerName = "llmux"
)

// TracingConfig contains configuration for OpenTelemetry tracing.
type TracingConfig struct {
	Enabled     bool
	Endpoint    string  // OTLP endpoint (e.g., "localhost:4317")
	ServiceName string  // Service name for traces
	SampleRate  float64 // Sampling rate (0.0 to 1.0)
	Insecure    bool    // Use insecure connection (no TLS)
}

// DefaultTracingConfig returns sensible defaults.
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:     false,
		Endpoint:    "localhost:4317",
		ServiceName: "llmux",
		SampleRate:  1.0,
		Insecure:    true,
	}
}

// TracerProvider wraps the OpenTelemetry tracer provider.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// InitTracing initializes OpenTelemetry tracing.
func InitTracing(ctx context.Context, cfg TracingConfig) (*TracerProvider, error) {
	if !cfg.Enabled {
		// Return a no-op tracer when disabled
		return &TracerProvider{
			tracer: otel.Tracer(TracerName),
		}, nil
	}

	// Create OTLP exporter
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("0.1.0"),
			attribute.String("gen_ai.system", "llmux"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider and propagator
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{
		provider: provider,
		tracer:   provider.Tracer(TracerName),
	}, nil
}

// Tracer returns the tracer instance.
func (tp *TracerProvider) Tracer() trace.Tracer {
	return tp.tracer
}

// Shutdown gracefully shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider != nil {
		return tp.provider.Shutdown(ctx)
	}
	return nil
}

// LLMSpanAttributes contains common attributes for LLM request spans.
type LLMSpanAttributes struct {
	Provider    string
	Model       string
	Stream      bool
	MaxTokens   int
	Temperature float64
}

// StartLLMSpan starts a new span for an LLM request with standard attributes.
func StartLLMSpan(ctx context.Context, tracer trace.Tracer, operation string, attrs LLMSpanAttributes) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", attrs.Provider),
			attribute.String("gen_ai.request.model", attrs.Model),
			attribute.Bool("gen_ai.request.stream", attrs.Stream),
		),
	)

	if attrs.MaxTokens > 0 {
		span.SetAttributes(attribute.Int("gen_ai.request.max_tokens", attrs.MaxTokens))
	}
	if attrs.Temperature > 0 {
		span.SetAttributes(attribute.Float64("gen_ai.request.temperature", attrs.Temperature))
	}

	return ctx, span
}

// RecordLLMResponse records response attributes on a span.
func RecordLLMResponse(span trace.Span, inputTokens, outputTokens int, finishReason string) {
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", inputTokens),
		attribute.Int("gen_ai.usage.output_tokens", outputTokens),
		attribute.String("gen_ai.response.finish_reason", finishReason),
	)
}

// RecordError records an error on a span.
func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}

// SpanFromContext extracts the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithTimeout creates a context with timeout and propagates trace context.
func ContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}
