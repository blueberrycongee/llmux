// Package observability provides request ID generation and propagation.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader is the HTTP header name for request IDs.
const RequestIDHeader = "X-Request-ID"

// requestIDKey is the context key for request IDs.
type requestIDKey struct{}

// GenerateRequestID generates a new unique request ID.
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a simple timestamp-based ID if crypto/rand fails
		return "req-fallback"
	}
	return hex.EncodeToString(b)
}

// ContextWithRequestID adds a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// RequestIDMiddleware adds request ID to incoming requests.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing request ID in header
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = GenerateRequestID()
		}

		// Add to response header
		w.Header().Set(RequestIDHeader, requestID)

		// Add to context
		ctx := ContextWithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetOrCreateRequestID gets existing request ID or creates a new one.
func GetOrCreateRequestID(ctx context.Context) (context.Context, string) {
	if id := RequestIDFromContext(ctx); id != "" {
		return ctx, id
	}
	id := GenerateRequestID()
	return ContextWithRequestID(ctx, id), id
}
