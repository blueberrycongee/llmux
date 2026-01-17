package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/observability"
)

func buildMiddlewareStack(cfg *config.Config, authStore auth.Store, logger *slog.Logger, syncer *auth.UserTeamSyncer, enforcer *auth.CasbinEnforcer) (func(http.Handler) http.Handler, error) {
	if cfg == nil {
		return nil, errNilConfig
	}

	var authMiddleware *auth.Middleware
	if cfg.Auth.Enabled {
		authMiddleware = auth.NewMiddleware(&auth.MiddlewareConfig{
			Store:                  authStore,
			Logger:                 logger,
			SkipPaths:              cfg.Auth.SkipPaths,
			Enabled:                true,
			LastUsedUpdateInterval: cfg.Auth.LastUsedUpdateInterval,
			Enforcer:               enforcer,
		})
		logger.Info("API key authentication middleware enabled", "casbin_enabled", enforcer != nil)
	}

	var oidcMiddleware func(http.Handler) http.Handler
	if cfg.Auth.Enabled && cfg.Auth.OIDC.IssuerURL != "" {
		oidcCfg := auth.OIDCConfig{
			IssuerURL:    cfg.Auth.OIDC.IssuerURL,
			ClientID:     cfg.Auth.OIDC.ClientID,
			ClientSecret: cfg.Auth.OIDC.ClientSecret,
			RoleClaim:    cfg.Auth.OIDC.ClaimMapping.RoleClaim,
			RolesMap:     cfg.Auth.OIDC.ClaimMapping.Roles,
		}
		// Use OIDCMiddlewareWithSync instead of OIDCMiddleware
		// This injects the syncer to enable automatic user-team sync from JWT claims
		middleware, err := auth.OIDCMiddlewareWithSync(oidcCfg, syncer)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize OIDC middleware: %w", err)
		}
		oidcMiddleware = middleware
		logger.Info("OIDC authentication enabled", "issuer", cfg.Auth.OIDC.IssuerURL, "sync_enabled", syncer != nil)
	}

	return func(next http.Handler) http.Handler {
		if next == nil {
			return nil
		}
		handler := next
		handler = managementBodyLimitMiddleware(handler)
		handler = managementAuthzMiddleware(cfg, enforcer)(handler)
		if authMiddleware != nil {
			handler = authMiddleware.ModelAccessMiddleware(handler)
			handler = authMiddleware.Authenticate(handler)
		}
		if oidcMiddleware != nil {
			handler = oidcMiddleware(handler)
		}
		handler = metrics.Middleware(handler)
		handler = observability.RequestIDMiddleware(handler)
		handler = corsMiddleware(cfg.CORS, handler)
		handler = recoveryMiddleware(logger)(handler)
		return handler
	}, nil
}

func detectLocaleFromRequest(r *http.Request) string {
	if r == nil {
		return "i18n"
	}
	if v := strings.TrimSpace(r.Header.Get("X-LLMux-Locale")); v != "" {
		v = strings.ToLower(v)
		if v == "cn" || strings.HasPrefix(v, "zh") {
			return "cn"
		}
		return "i18n"
	}
	// Accept-Language: e.g. "zh-CN,zh;q=0.9,en;q=0.8"
	al := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(al, "zh") {
		return "cn"
	}
	return "i18n"
}

func localizeAuthzMessage(locale, message string) string {
	if locale != "cn" {
		return message
	}
	switch message {
	case "internal server error":
		return "服务器内部错误"
	case "authentication required":
		return "需要认证"
	case "management permission required":
		return "需要管理权限"
	case "invalid request body":
		return "请求体无效"
	case "request body too large":
		return "请求体过大"
	}
	return message
}

func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered", "error", err, "path", r.URL.Path)
					writeAuthzError(w, r, http.StatusInternalServerError, "internal server error", "server_error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

const maxManagementBodyBytes int64 = 1 << 20

const bootstrapTokenHeader = "X-LLMux-Bootstrap-Token" // #nosec G101 -- header name, not a credential

func managementAuthzMiddleware(cfg *config.Config, enforcer *auth.CasbinEnforcer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg == nil || !cfg.Auth.Enabled || r == nil || !isManagementPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			bootstrapToken := strings.TrimSpace(cfg.Auth.BootstrapToken)
			if bootstrapToken != "" && r.Header.Get(bootstrapTokenHeader) == bootstrapToken {
				next.ServeHTTP(w, r)
				return
			}

			authCtx := auth.GetAuthContext(r.Context())
			if authCtx == nil {
			writeAuthzError(w, r, http.StatusUnauthorized, "authentication required", "authentication_error")
			return
		}

			if enforcer != nil {
				var sub string
				if authCtx.APIKey != nil {
					sub = auth.KeySub(authCtx.APIKey.ID)
					_, _ = enforcer.AddRoleForUser(sub, auth.RoleSub(string(authCtx.APIKey.KeyType)))
				} else if authCtx.User != nil {
					sub = auth.UserSub(authCtx.User.ID)
					_, _ = enforcer.AddRoleForUser(sub, auth.RoleSub(string(authCtx.UserRole)))
				}

				if sub != "" {
					allowed, err := enforcer.Enforce(sub, auth.PathObj(r.URL.Path), auth.ActionMethod(r.Method))
					if err == nil && allowed {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			// Fallback to legacy hardcoded logic
			if authCtx.User != nil {
				switch authCtx.UserRole {
				case auth.UserRoleProxyAdmin, auth.UserRoleProxyAdminViewer:
					next.ServeHTTP(w, r)
					return
				default:
					writeAuthzError(w, r, http.StatusForbidden, "management permission required", "permission_error")
					return
				}
			}

			if authCtx.APIKey != nil && authCtx.APIKey.KeyType == auth.KeyTypeManagement {
				next.ServeHTTP(w, r)
				return
			}

		writeAuthzError(w, r, http.StatusForbidden, "management permission required", "permission_error")
		})
	}
}

func isManagementPath(path string) bool {
	if path == "" {
		return false
	}
	managementPrefixes := []string{
		"/key/",
		"/team/",
		"/user/",
		"/organization/",
		"/spend/",
		"/audit/",
		"/global/",
		"/invitation/",
		"/control/",
		"/mcp/",
	}
	for _, prefix := range managementPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeAuthzError(w http.ResponseWriter, r *http.Request, status int, message, typ string) {
	message = localizeAuthzMessage(detectLocaleFromRequest(r), message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"message":"` + message + `","type":"` + typ + `"}}`))
}

func managementBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.URL == nil || !isManagementPath(r.URL.Path) || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxManagementBodyBytes+1))
		_ = r.Body.Close()
		if err != nil {
			writeAuthzError(w, r, http.StatusBadRequest, "invalid request body", "request_error")
			return
		}
		if int64(len(body)) > maxManagementBodyBytes {
			writeAuthzError(w, r, http.StatusRequestEntityTooLarge, "request body too large", "request_error")
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}
