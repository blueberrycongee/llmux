package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/observability"
)

func buildMiddlewareStack(cfg *config.Config, authStore auth.Store, logger *slog.Logger, syncer *auth.UserTeamSyncer) (func(http.Handler) http.Handler, error) {
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
		})
		logger.Info("API key authentication middleware enabled")
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
		if authMiddleware != nil {
			handler = authMiddleware.Authenticate(handler)
		}
		if oidcMiddleware != nil {
			handler = oidcMiddleware(handler)
		}
		handler = metrics.Middleware(handler)
		handler = observability.RequestIDMiddleware(handler)
		handler = corsMiddleware(cfg.CORS, handler)
		return handler
	}, nil
}
