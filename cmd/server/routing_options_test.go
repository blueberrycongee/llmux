package main

import (
	"testing"

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
		fallbackEnabled bool
	}{
		{
			name:            "uses retry and fallback from config",
			retryCount:      1,
			fallbackEnabled: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Routing: config.RoutingConfig{
					RetryCount:      tt.retryCount,
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
			if clientCfg.RetryBackoff != routingRetryBackoff {
				t.Fatalf("expected retry backoff %s, got %s", routingRetryBackoff, clientCfg.RetryBackoff)
			}
		})
	}
}
