// Package observability provides structured logging with redaction support.
package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger with redaction and request ID support.
type Logger struct {
	logger   *slog.Logger
	redactor *Redactor
}

// LoggerConfig contains configuration for the logger.
type LoggerConfig struct {
	Level      slog.Level
	Output     io.Writer
	AddSource  bool
	JSONFormat bool
}

// NewLogger creates a new logger with redaction support.
func NewLogger(cfg LoggerConfig, redactor *Redactor) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.JSONFormat {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	return &Logger{
		logger:   slog.New(handler),
		redactor: redactor,
	}
}

// WithRequestID returns a logger with the request ID from context.
func (l *Logger) WithRequestID(ctx context.Context) *Logger {
	requestID := RequestIDFromContext(ctx)
	if requestID == "" {
		return l
	}
	return &Logger{
		logger:   l.logger.With("request_id", requestID),
		redactor: l.redactor,
	}
}

// WithFields returns a logger with additional fields.
func (l *Logger) WithFields(args ...any) *Logger {
	return &Logger{
		logger:   l.logger.With(args...),
		redactor: l.redactor,
	}
}

// RedactedInfo logs at INFO level with redacted message.
func (l *Logger) RedactedInfo(msg string, args ...any) {
	if l.redactor != nil {
		msg = l.redactor.Redact(msg)
		args = l.redactArgs(args)
	}
	l.logger.Info(msg, args...)
}

// RedactedError logs at ERROR level with redacted message.
func (l *Logger) RedactedError(msg string, args ...any) {
	if l.redactor != nil {
		msg = l.redactor.Redact(msg)
		args = l.redactArgs(args)
	}
	l.logger.Error(msg, args...)
}

// RedactedDebug logs at DEBUG level with redacted message.
func (l *Logger) RedactedDebug(msg string, args ...any) {
	if l.redactor != nil {
		msg = l.redactor.Redact(msg)
		args = l.redactArgs(args)
	}
	l.logger.Debug(msg, args...)
}

// RedactedWarn logs at WARN level with redacted message.
func (l *Logger) RedactedWarn(msg string, args ...any) {
	if l.redactor != nil {
		msg = l.redactor.Redact(msg)
		args = l.redactArgs(args)
	}
	l.logger.Warn(msg, args...)
}

func (l *Logger) redactArgs(args []any) []any {
	if l.redactor == nil {
		return args
	}

	result := make([]any, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			result[i] = l.redactor.Redact(v)
		case error:
			result[i] = l.redactor.Redact(v.Error())
		default:
			result[i] = arg
		}
	}
	return result
}

// Slog returns the underlying slog.Logger for compatibility.
func (l *Logger) Slog() *slog.Logger {
	return l.logger
}

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// With returns a logger with additional fields.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:   l.logger.With(args...),
		redactor: l.redactor,
	}
}
