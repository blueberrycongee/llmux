// Package fireworks implements the Fireworks AI provider adapter.
// Fireworks AI provides fast and affordable inference for open-source models.
// API Reference: https://docs.fireworks.ai/api-reference
package fireworks

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "fireworks"

	// DefaultBaseURL is the default Fireworks AI API endpoint.
	DefaultBaseURL = "https://api.fireworks.ai/inference/v1"
)

// DefaultModels lists commonly available Fireworks AI models.
var DefaultModels = []string{
	"accounts/fireworks/models/llama-v3p1-70b-instruct",
	"accounts/fireworks/models/llama-v3p1-8b-instruct",
	"accounts/fireworks/models/llama-v3p1-405b-instruct",
	"accounts/fireworks/models/mixtral-8x7b-instruct",
	"accounts/fireworks/models/qwen2p5-72b-instruct",
	"accounts/fireworks/models/firefunction-v2",
	"accounts/fireworks/models/fw-function-call-34b-v0",
}

// New creates a new Fireworks AI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"accounts/fireworks/",
			"fw-",
		},
	}
	return openailike.New(cfg, info)
}
