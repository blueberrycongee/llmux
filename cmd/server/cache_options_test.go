package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/caches/memory"
	"github.com/blueberrycongee/llmux/internal/config"
)

func TestBuildClientOptions_CacheDisabled(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Enabled: false,
		},
	}

	opts := buildClientOptions(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	clientCfg := applyOptions(opts)

	if clientCfg.CacheEnabled {
		t.Fatalf("expected cache disabled, got enabled")
	}
	if clientCfg.Cache != nil {
		t.Fatalf("expected cache to be nil when disabled")
	}
}

func TestBuildClientOptions_CacheEnabledLocal(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Enabled: true,
			Type:    "local",
			TTL:     time.Minute,
			Memory: config.MemoryCacheConfig{
				MaxSize:         10,
				DefaultTTL:      time.Minute,
				MaxItemSize:     1024,
				CleanupInterval: time.Second,
			},
		},
	}

	opts := buildClientOptions(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	clientCfg := applyOptions(opts)

	if !clientCfg.CacheEnabled {
		t.Fatalf("expected cache enabled, got disabled")
	}
	if clientCfg.Cache == nil {
		t.Fatalf("expected cache instance when enabled")
	}
	if _, ok := clientCfg.Cache.(*memory.Cache); !ok {
		t.Fatalf("expected memory cache, got %T", clientCfg.Cache)
	}
	if clientCfg.CacheTTL != cfg.Cache.TTL {
		t.Fatalf("expected cache TTL %s, got %s", cfg.Cache.TTL, clientCfg.CacheTTL)
	}
	if clientCfg.CacheTypeLabel != "local" {
		t.Fatalf("expected cache type label local, got %s", clientCfg.CacheTypeLabel)
	}
}
