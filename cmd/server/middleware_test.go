package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/blueberrycongee/llmux/internal/config"
)

func TestBuildMiddlewareStack_FailOpenAllowsOnBackendError(t *testing.T) {
	status := runRateLimitRequestWithRedisFailure(t, true)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
}

func TestBuildMiddlewareStack_FailCloseDeniesOnBackendError(t *testing.T) {
	status := runRateLimitRequestWithRedisFailure(t, false)
	if status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func runRateLimitRequestWithRedisFailure(t *testing.T, failOpen bool) int {
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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	middleware, err := buildMiddlewareStack(cfg, nil, logger, nil)
	if err != nil {
		t.Fatalf("buildMiddlewareStack error: %v", err)
	}

	redisServer.Close()
	redisServer = nil

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "http://localhost/v1/chat/completions", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	return rr.Code
}
