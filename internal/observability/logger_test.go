package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.Slog() == nil {
		t.Error("expected non-nil underlying logger")
	}
	if logger.redactor == nil {
		t.Error("expected non-nil redactor")
	}
}

func TestLogger_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, nil)
	ctx := ContextWithRequestID(context.Background(), "test-req-123")

	loggerWithID := logger.WithRequestID(ctx)
	loggerWithID.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-req-123") {
		t.Errorf("expected request ID in output, got %s", output)
	}
}

func TestLogger_WithRequestID_Empty(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, nil)
	ctx := context.Background() // No request ID

	loggerWithID := logger.WithRequestID(ctx)

	// Should return same logger instance
	if loggerWithID != logger {
		t.Error("expected same logger when no request ID")
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, nil)
	loggerWithFields := logger.WithFields("provider", "openai", "model", "gpt-4")
	loggerWithFields.Info("test")

	output := buf.String()
	if !strings.Contains(output, "openai") {
		t.Errorf("expected provider in output, got %s", output)
	}
	if !strings.Contains(output, "gpt-4") {
		t.Errorf("expected model in output, got %s", output)
	}
}

func TestLogger_RedactedInfo(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())
	logger.RedactedInfo("API key is sk-1234567890abcdefghijklmnop")

	output := buf.String()
	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected API key to be redacted, got %s", output)
	}
	if !strings.Contains(output, "[REDACTED_OPENAI_KEY]") {
		t.Errorf("expected redaction marker, got %s", output)
	}
}

func TestLogger_RedactedError(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())
	logger.RedactedError("failed with key sk-1234567890abcdefghijklmnop")

	output := buf.String()
	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected API key to be redacted in error")
	}
}

func TestLogger_RedactedDebug(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelDebug,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())
	logger.RedactedDebug("debug: email test@example.com")

	output := buf.String()
	if strings.Contains(output, "test@example.com") {
		t.Errorf("expected email to be redacted")
	}
}

func TestLogger_RedactedWarn(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelWarn,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())
	logger.RedactedWarn("warning: phone +1-555-123-4567")

	output := buf.String()
	if strings.Contains(output, "555-123-4567") {
		t.Errorf("expected phone to be redacted")
	}
}

func TestLogger_RedactArgs(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())
	logger.RedactedInfo("request", "key", "sk-1234567890abcdefghijklmnop")

	output := buf.String()
	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected key arg to be redacted")
	}
}

func TestLogger_RedactArgs_Error(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, NewRedactor())
	err := errors.New("failed with key sk-1234567890abcdefghijklmnop")
	logger.RedactedError("operation failed", "error", err)

	output := buf.String()
	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected error message to be redacted")
	}
}

func TestLogger_NoRedactor(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, nil) // No redactor
	logger.RedactedInfo("API key is sk-1234567890abcdefghijklmnop")

	output := buf.String()
	// Without redactor, should not redact
	if !strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected no redaction without redactor")
	}
}

func TestLogger_Slog(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	}

	logger := NewLogger(cfg, nil)
	slogger := logger.Slog()

	if slogger == nil {
		t.Error("expected non-nil slog.Logger")
	}
}

func TestLogger_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Level:      slog.LevelInfo,
		Output:     &buf,
		JSONFormat: false, // Text format
	}

	logger := NewLogger(cfg, nil)
	logger.Info("test message")

	output := buf.String()
	if strings.Contains(output, "{") {
		t.Errorf("expected text format, got JSON-like output: %s", output)
	}
}
