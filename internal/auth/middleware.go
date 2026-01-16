package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// AuthContextKey is the context key for AuthContext.
	AuthContextKey contextKey = "auth"
	// maxModelAccessBodyBytes should match the API default max request body size.
	maxModelAccessBodyBytes int64 = 10 * 1024 * 1024
)

// Middleware provides HTTP middleware for API key authentication.
type Middleware struct {
	store                  Store
	logger                 *slog.Logger
	skipPaths              map[string]bool
	enabled                bool
	lastUsedUpdateInterval time.Duration
	enforcer               *CasbinEnforcer
}

// MiddlewareConfig contains configuration for the auth middleware.
type MiddlewareConfig struct {
	Store                  Store
	Logger                 *slog.Logger
	SkipPaths              []string // Paths to skip authentication (e.g., /health, /metrics)
	Enabled                bool
	LastUsedUpdateInterval time.Duration
	Enforcer               *CasbinEnforcer
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(cfg *MiddlewareConfig) *Middleware {
	skipPaths := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPaths[path] = true
	}

	return &Middleware{
		store:                  cfg.Store,
		logger:                 cfg.Logger,
		skipPaths:              skipPaths,
		enabled:                cfg.Enabled,
		lastUsedUpdateInterval: cfg.LastUsedUpdateInterval,
		enforcer:               cfg.Enforcer,
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

		// If another auth mechanism already authenticated this request (e.g. OIDC),
		// do not force API key authentication.
		if GetAuthContext(r.Context()) != nil {
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

		if key.Blocked {
			m.writeUnauthorized(w, "api key is blocked")
			return
		}

		// Load team if associated
		var team *Team
		if key.TeamID != nil {
			team, err = m.store.GetTeam(r.Context(), *key.TeamID)
			if err != nil {
				m.logger.Error("failed to lookup team", "error", err, "team_id", *key.TeamID)
				m.writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if team == nil {
				m.writeUnauthorized(w, "invalid team")
				return
			}
			if team.IsBlocked() {
				m.writeUnauthorized(w, "team is blocked")
				return
			}
		}

		// Enforce permissions via Casbin if available.
		if m.enforcer != nil {
			sub := KeySub(key.ID)
			obj := PathObj(r.URL.Path)
			act := ActionMethod(r.Method)

			// Map key type to role
			_, _ = m.enforcer.AddRoleForUser(sub, RoleSub(string(key.KeyType)))

			allowed, err := m.enforcer.Enforce(sub, obj, act)
			if err != nil {
				m.logger.Error("casbin enforcement failed", "error", err)
				m.writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !allowed {
				m.writePermissionDenied(w, "access denied by policy")
				return
			}
		} else if key.KeyType == KeyTypeReadOnly {
			// Legacy hardcoded key type restrictions.
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				m.writePermissionDenied(w, "read-only key")
				return
			}
			if r.URL.Path != "/v1/models" {
				m.writePermissionDenied(w, "read-only key")
				return
			}
		}

		now := time.Now()
		if m.shouldUpdateLastUsed(key.LastUsedAt, now) {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := m.store.UpdateAPIKeyLastUsed(ctx, key.ID, now); err != nil {
				m.logger.Warn("failed to update last_used_at", "error", err, "key_id", key.ID)
			}
		}

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

func (m *Middleware) shouldUpdateLastUsed(lastUsed *time.Time, now time.Time) bool {
	if m.lastUsedUpdateInterval <= 0 {
		return true
	}
	if lastUsed == nil {
		return true
	}
	if lastUsed.After(now) {
		return false
	}
	return now.Sub(*lastUsed) >= m.lastUsedUpdateInterval
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

func (m *Middleware) writePermissionDenied(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":{"message":"` + message + `","type":"permission_error"}}`))
}

// ModelAccessMiddleware checks if the authenticated key can access the requested model.
// This should be called after Authenticate middleware.
func (m *Middleware) ModelAccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		if m.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		if !isModelAccessRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		authCtx := GetAuthContext(r.Context())
		if authCtx == nil {
			m.writeUnauthorized(w, "authentication required")
			return
		}

		origBody := r.Body
		limitedBody := io.LimitReader(origBody, maxModelAccessBodyBytes+1)
		body, err := io.ReadAll(limitedBody)
		_ = origBody.Close()
		if err != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}
		if int64(len(body)) > maxModelAccessBodyBytes {
			m.writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		model, err := parseModelFromBody(body)
		if err != nil || model == "" {
			next.ServeHTTP(w, r)
			return
		}

		if m.enforcer != nil {
			sub := KeySub(authCtx.APIKey.ID)
			obj := ModelObj(model)
			act := ActionUse()

			// Sync roles for the key
			_, _ = m.enforcer.AddRoleForUser(sub, RoleSub(string(authCtx.APIKey.KeyType)))
			if authCtx.Team != nil {
				_, _ = m.enforcer.AddGroupingPolicy(sub, TeamSub(authCtx.Team.ID))
			}
			if authCtx.User != nil {
				_, _ = m.enforcer.AddGroupingPolicy(sub, UserSub(authCtx.User.ID))
			}

			// Add temporary policies for legacy allowed_models support
			// In a real production system, these should be synced to the database
			// or loaded via a custom adapter.
			if len(authCtx.APIKey.AllowedModels) > 0 {
				for _, am := range authCtx.APIKey.AllowedModels {
					_, _ = m.enforcer.AddPolicy(sub, ModelObj(am), act)
				}
			} else {
				// Empty AllowedModels means all models allowed for this key
				_, _ = m.enforcer.AddPolicy(sub, ModelObj("*"), act)
			}

			allowed, err := m.enforcer.Enforce(sub, obj, act)
			if err != nil {
				m.logger.Error("failed to evaluate model access via casbin", "error", err)
				m.writeError(w, http.StatusInternalServerError, "internal error")
				return
			}

			if !allowed {
				m.writePermissionDenied(w, "model access denied")
				return
			}
		} else {
			access, err := NewModelAccess(r.Context(), m.store, authCtx)
			if err != nil {
				m.logger.Error("failed to evaluate model access", "error", err)
				m.writeError(w, http.StatusInternalServerError, "internal error")
				return
			}

			if !access.Allows(model) {
				m.writePermissionDenied(w, "model access denied")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

type modelRequest struct {
	Model string `json:"model"`
}

func parseModelFromBody(body []byte) (string, error) {
	var req modelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", err
	}
	return req.Model, nil
}

func isModelAccessRequest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/v1/chat/completions", "/v1/completions", "/v1/embeddings", "/embeddings":
		return true
	default:
		return false
	}
}
