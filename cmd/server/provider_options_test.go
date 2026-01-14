package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/config"
)

func TestBuildClientOptions_WiresProviderConfigFields(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Name:          "p1",
				Type:          "openai",
				APIKey:        "k",
				BaseURL:       "https://example.com/v1",
				Models:        []string{"gpt-4o"},
				MaxConcurrent: 7,
				Timeout:       3 * time.Second,
				Headers: map[string]string{
					"X-Test": "1",
				},
			},
		},
	}

	opts := buildClientOptions(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	clientCfg := applyOptions(opts)

	if len(clientCfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(clientCfg.Providers))
	}
	got := clientCfg.Providers[0]

	if got.MaxConcurrent != cfg.Providers[0].MaxConcurrent {
		t.Fatalf("expected max_concurrent %d, got %d", cfg.Providers[0].MaxConcurrent, got.MaxConcurrent)
	}
	if got.Timeout != cfg.Providers[0].Timeout {
		t.Fatalf("expected timeout %s, got %s", cfg.Providers[0].Timeout, got.Timeout)
	}
	if got.Headers["X-Test"] != "1" {
		t.Fatalf("expected header to be wired")
	}
}
