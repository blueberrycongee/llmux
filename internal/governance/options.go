package governance

import (
	"log/slog"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// Option configures the governance engine.
type Option func(*Engine)

// WithStore sets the auth store for governance checks and accounting.
func WithStore(store auth.Store) Option {
	return func(e *Engine) {
		e.store = store
	}
}

// WithRateLimiter sets the tenant rate limiter for governance checks.
func WithRateLimiter(limiter *auth.TenantRateLimiter) Option {
	return func(e *Engine) {
		e.rateLimiter = limiter
	}
}

// WithAuditLogger sets the audit logger for governance events.
func WithAuditLogger(logger *auth.AuditLogger) Option {
	return func(e *Engine) {
		e.auditLogger = logger
	}
}

// WithLogger sets the logger for governance diagnostics.
func WithLogger(logger *slog.Logger) Option {
	return func(e *Engine) {
		e.logger = logger
	}
}

// WithIdempotencyStore sets the idempotency store for accounting writes.
func WithIdempotencyStore(store IdempotencyStore) Option {
	return func(e *Engine) {
		e.idempotency = store
	}
}
