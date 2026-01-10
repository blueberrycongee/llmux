package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// AuthContextKey is the context key for AuthContext.
	AuthContextKey contextKey = "auth"
)

// Middleware provides HTTP middleware for API key authentication.
type Middleware struct {
	store     Store
	logger    *slog.Logger
	skipPaths map[string]bool
	enabled   bool
}

// MiddlewareConfig contains configuration for the auth middleware.
type MiddlewareConfig struct {
	Store     Store
	Logger    *slog.Logger
	SkipPaths []string // Paths to skip authentication (e.g., /health, /metrics)
	Enabled   bool
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(cfg *MiddlewareConfig) *Middleware {
	skipPaths := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPaths[path] = true
	}

	return &Middleware{
		store:     cfg.Store,
		logger:    cfg.Logger,
		skipPaths: skipPaths,
		enabled:   cfg.Enabled,
	}
}

// Authenticate returns an HTTP middleware that validates API keys.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if disabled
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for certain paths
		if m.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		apiKey, err := ParseAuthHeader(authHeader)
		if err != nil {
			m.writeUnauthorized(w, "missing or invalid authorization header")
			return
		}

		// Hash the key and look it up
		keyHash := HashKey(apiKey)
		key, err := m.store.GetAPIKeyByHash(r.Context(), keyHash)
		if err != nil {
			m.logger.Error("failed to lookup api key", "error", err)
			m.writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if key == nil {
			m.writeUnauthorized(w, "invalid api key")
			return
		}

		// Validate key status
		if !key.IsActive {
			m.writeUnauthorized(w, "api key is inactive")
			return
		}

		if key.IsExpired() {
			m.writeUnauthorized(w, "api key has expired")
			return
		}

		if key.IsOverBudget() {
			m.writeError(w, http.StatusPaymentRequired, "api key budget exceeded")
			return
		}

		// Load team if associated
		var team *Team
		if key.TeamID != nil {
			team, err = m.store.GetTeam(r.Context(), *key.TeamID)
			if err != nil {
				m.logger.Error("failed to lookup team", "error", err, "team_id", *key.TeamID)
			}
			if team != nil && team.IsOverBudget() {
				m.writeError(w, http.StatusPaymentRequired, "team budget exceeded")
				return
			}
		}

		// Update last used timestamp (async to not block request)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := m.store.UpdateAPIKeyLastUsed(ctx, key.ID, time.Now()); err != nil {
				m.logger.Warn("failed to update last_used_at", "error", err, "key_id", key.ID)
			}
		}()

		// Create auth context
		authCtx := &AuthContext{
			APIKey: key,
			Team:   team,
		}

		// Add auth context to request context
		ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAuthContext retrieves the AuthContext from the request context.
func GetAuthContext(ctx context.Context) *AuthContext {
	if auth, ok := ctx.Value(AuthContextKey).(*AuthContext); ok {
		return auth
	}
	return nil
}

func (m *Middleware) writeUnauthorized(w http.ResponseWriter, message string) {
	m.writeError(w, http.StatusUnauthorized, message)
}

func (m *Middleware) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"message":"` + message + `","type":"authentication_error"}}`))
}

// ModelAccessMiddleware checks if the authenticated key can access the requested model.
// This should be called after Authenticate middleware.
func (m *Middleware) ModelAccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Model validation is done in the handler after parsing the request body
		// This middleware is a placeholder for future enhancements
		// TODO: [Real Data Fetching] - Implement real model access control logic here.
		// Should fetch allowed models for the API Key/User from Store and validate against request.
		next.ServeHTTP(w, r)
	})
}
