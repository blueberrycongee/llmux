package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestInitTracing_Disabled(t *testing.T) {
	cfg := TracingConfig{
		Enabled: false,
	}

	tp, err := InitTracing(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracing failed: %v", err)
	}
	defer tp.Shutdown(context.Background())

	if tp.Tracer() == nil {
		t.Error("expected non-nil tracer even when disabled")
	}
}

func TestDefaultTracingConfig(t *testing.T) {
	cfg := DefaultTracingConfig()

	if cfg.Enabled {
		t.Error("expected Enabled to be false by default")
	}
	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("expected endpoint localhost:4317, got %s", cfg.Endpoint)
	}
	if cfg.ServiceName != "llmux" {
		t.Errorf("expected service name llmux, got %s", cfg.ServiceName)
	}
	if cfg.SampleRate != 1.0 {
		t.Errorf("expected sample rate 1.0, got %f", cfg.SampleRate)
	}
}

func TestStartLLMSpan(t *testing.T) {
	cfg := TracingConfig{Enabled: false}
	tp, _ := InitTracing(context.Background(), cfg)
	defer tp.Shutdown(context.Background())

	attrs := LLMSpanAttributes{
		Provider:    "openai",
		Model:       "gpt-4",
		Stream:      true,
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	ctx, span := StartLLMSpan(context.Background(), tp.Tracer(), "chat_completion", attrs)
	defer span.End()

	if ctx == nil {
		t.Error("expected non-nil context")
	}
	if span == nil {
		t.Error("expected non-nil span")
	}
}

func TestRecordLLMResponse(t *testing.T) {
	cfg := TracingConfig{Enabled: false}
	tp, _ := InitTracing(context.Background(), cfg)
	defer tp.Shutdown(context.Background())

	_, span := tp.Tracer().Start(context.Background(), "test")
	defer span.End()

	// Should not panic
	RecordLLMResponse(span, 100, 50, "stop")
}

func TestRecordError(t *testing.T) {
	cfg := TracingConfig{Enabled: false}
	tp, _ := InitTracing(context.Background(), cfg)
	defer tp.Shutdown(context.Background())

	_, span := tp.Tracer().Start(context.Background(), "test")
	defer span.End()

	// Should not panic
	RecordError(span, context.DeadlineExceeded)
}

func TestSpanFromContext(t *testing.T) {
	cfg := TracingConfig{Enabled: false}
	tp, _ := InitTracing(context.Background(), cfg)
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer().Start(context.Background(), "test")
	defer span.End()

	extracted := SpanFromContext(ctx)
	if extracted.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Error("extracted span should match original")
	}
}

func TestTracerProvider_Shutdown(t *testing.T) {
	// Test shutdown with nil provider (disabled tracing)
	tp := &TracerProvider{
		tracer: noop.NewTracerProvider().Tracer("test"),
	}

	err := tp.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown should not error with nil provider: %v", err)
	}
}
