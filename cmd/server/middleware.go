package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/resilience"
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
		logger.Info("model access middleware enabled")
	}

	var rateLimiter *auth.TenantRateLimiter
	if cfg.RateLimit.Enabled {
		defaultRPM := int(cfg.RateLimit.RequestsPerMinute)
		defaultBurst := cfg.RateLimit.BurstSize
		useDefaultBurst := false
		if defaultBurst <= 0 {
			defaultBurst = defaultRPM / 6
			if defaultBurst < 1 {
				defaultBurst = 1
			}
		} else {
			useDefaultBurst = true
		}
		rateLimiter = auth.NewTenantRateLimiter(&auth.TenantRateLimiterConfig{
			DefaultRPM:      defaultRPM,
			DefaultBurst:    defaultBurst,
			UseDefaultBurst: useDefaultBurst,
			CleanupTTL:      10 * time.Minute,
			FailOpen:        cfg.RateLimit.FailOpen,
			Logger:          logger,
		})

		// Inject distributed limiter if configured (for multi-instance deployments)
		if cfg.RateLimit.Distributed && (cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0) {
			redisClient, isCluster, err := newRedisUniversalClient(cfg.Cache.Redis)
			if err != nil {
				logger.Warn("distributed rate limiting unavailable, using local limiter", "error", err)
			} else {
				// Test connection before using
				pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := redisClient.Ping(pingCtx).Err(); err != nil {
					logger.Warn("distributed rate limiting unavailable, using local limiter", "error", err)
				} else {
					distributedLimiter := resilience.NewRedisLimiter(redisClient)
					rateLimiter.SetDistributedLimiter(distributedLimiter)
					logger.Info("gateway rate limiting using distributed Redis backend", "cluster", isCluster)
				}
				pingCancel()
			}
		}

		logger.Info("gateway rate limiting enabled",
			"default_rpm", cfg.RateLimit.RequestsPerMinute,
			"default_burst", defaultBurst,
			"distributed", cfg.RateLimit.Distributed,
		)
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
			handler = authMiddleware.ModelAccessMiddleware(handler)
			handler = authMiddleware.Authenticate(handler)
		}
		if rateLimiter != nil {
			handler = rateLimiter.RateLimitMiddleware(handler)
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
