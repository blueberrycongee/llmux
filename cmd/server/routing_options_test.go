package main

import (
	"testing"
	"time"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/config"
)

func applyOptions(opts []llmux.Option) llmux.ClientConfig {
	cfg := llmux.ClientConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func TestBuildRoutingOptions(t *testing.T) {
	tests := []struct {
		name            string
		retryCount      int
		retryBackoff    time.Duration
		retryMaxBackoff time.Duration
		retryJitter     float64
		fallbackEnabled bool
	}{
		{
			name:            "uses retry and fallback from config",
			retryCount:      1,
			retryBackoff:    200 * time.Millisecond,
			retryMaxBackoff: 2 * time.Second,
			retryJitter:     0.3,
			fallbackEnabled: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Routing: config.RoutingConfig{
					RetryCount:      tt.retryCount,
					RetryBackoff:    tt.retryBackoff,
					RetryMaxBackoff: tt.retryMaxBackoff,
					RetryJitter:     tt.retryJitter,
					FallbackEnabled: tt.fallbackEnabled,
				},
			}

			clientCfg := applyOptions(buildRoutingOptions(cfg))

			if clientCfg.RetryCount != tt.retryCount {
				t.Fatalf("expected retry count %d, got %d", tt.retryCount, clientCfg.RetryCount)
			}
			if clientCfg.FallbackEnabled != tt.fallbackEnabled {
				t.Fatalf("expected fallback enabled %t, got %t", tt.fallbackEnabled, clientCfg.FallbackEnabled)
			}
			if clientCfg.RetryBackoff != tt.retryBackoff {
				t.Fatalf("expected retry backoff %s, got %s", tt.retryBackoff, clientCfg.RetryBackoff)
			}
			if clientCfg.RetryMaxBackoff != tt.retryMaxBackoff {
				t.Fatalf("expected retry max backoff %s, got %s", tt.retryMaxBackoff, clientCfg.RetryMaxBackoff)
			}
			if clientCfg.RetryJitter != tt.retryJitter {
				t.Fatalf("expected retry jitter %f, got %f", tt.retryJitter, clientCfg.RetryJitter)
			}
		})
	}
}
