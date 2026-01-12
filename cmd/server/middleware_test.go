package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/blueberrycongee/llmux/internal/config"
)

func TestBuildTenantRateLimiter_FailOpenAllowsOnBackendError(t *testing.T) {
	allowed := runRateLimitCheckWithRedisFailure(t, true)
	if !allowed {
		t.Fatal("expected request to be allowed")
	}
}

func TestBuildTenantRateLimiter_FailCloseDeniesOnBackendError(t *testing.T) {
	allowed := runRateLimitCheckWithRedisFailure(t, false)
	if allowed {
		t.Fatal("expected request to be denied")
	}
}

func runRateLimitCheckWithRedisFailure(t *testing.T, failOpen bool) bool {
	t.Helper()

	redisServer := miniredis.RunT(t)
	defer func() {
		if redisServer != nil {
			redisServer.Close()
		}
	}()

	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
			BurstSize:         10,
			Distributed:       true,
			FailOpen:          failOpen,
		},
		Cache: config.CacheConfig{
			Redis: config.RedisCacheConfig{
				Addr:         redisServer.Addr(),
				DialTimeout:  50 * time.Millisecond,
				ReadTimeout:  50 * time.Millisecond,
				WriteTimeout: 50 * time.Millisecond,
			},
		},
	}

	limiter := buildTenantRateLimiter(cfg, slogDiscard())
	redisServer.Close()
	redisServer = nil

	allowed, _ := limiter.Check(context.Background(), "tenant", 10, 1)
	return allowed
}

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
