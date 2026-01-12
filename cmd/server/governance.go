package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/governance"
	"github.com/blueberrycongee/llmux/internal/resilience"
)

func buildGovernanceEngine(cfg *config.Config, authStore auth.Store, auditLogger *auth.AuditLogger, logger *slog.Logger) *governance.Engine {
	if cfg == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	var rateLimiter *auth.TenantRateLimiter
	if cfg.RateLimit.Enabled {
		rateLimiter = buildTenantRateLimiter(cfg, logger)
	}

	var idempotency governance.IdempotencyStore
	if cfg.Governance.IdempotencyWindow > 0 {
		if cfg.Deployment.Mode == "distributed" && (cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0) {
			redisClient, _, err := newRedisUniversalClient(cfg.Cache.Redis)
			if err != nil {
				logger.Warn("distributed idempotency unavailable, falling back to memory", "error", err)
			} else {
				idempotency = governance.NewRedisIdempotencyStore(redisClient, "llmux:idempotency:")
			}
		}
		if idempotency == nil {
			idempotency = governance.NewMemoryIdempotencyStore()
		}
	}

	return governance.NewEngine(mapGovernanceConfig(cfg.Governance),
		governance.WithStore(authStore),
		governance.WithRateLimiter(rateLimiter),
		governance.WithAuditLogger(auditLogger),
		governance.WithIdempotencyStore(idempotency),
		governance.WithLogger(logger),
	)
}

func mapGovernanceConfig(cfg config.GovernanceConfig) governance.Config {
	return governance.Config{
		Enabled:           cfg.Enabled,
		AsyncAccounting:   cfg.AsyncAccounting,
		IdempotencyWindow: cfg.IdempotencyWindow,
		AuditEnabled:      cfg.AuditEnabled,
	}
}

func buildTenantRateLimiter(cfg *config.Config, logger *slog.Logger) *auth.TenantRateLimiter {
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

	rateLimiter := auth.NewTenantRateLimiter(&auth.TenantRateLimiterConfig{
		DefaultRPM:        defaultRPM,
		DefaultBurst:      defaultBurst,
		UseDefaultBurst:   useDefaultBurst,
		CleanupTTL:        10 * time.Minute,
		FailOpen:          cfg.RateLimit.FailOpen,
		Logger:            logger,
		TrustedProxyCIDRs: cfg.RateLimit.TrustedProxyCIDRs,
	})

	if cfg.RateLimit.Distributed && (cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0) {
		redisClient, isCluster, err := newRedisUniversalClient(cfg.Cache.Redis)
		if err != nil {
			logger.Warn("distributed rate limiting unavailable, using local limiter", "error", err)
		} else {
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

	logger.Info("gateway governance rate limiting enabled",
		"default_rpm", cfg.RateLimit.RequestsPerMinute,
		"default_burst", defaultBurst,
		"distributed", cfg.RateLimit.Distributed,
	)

	return rateLimiter
}
