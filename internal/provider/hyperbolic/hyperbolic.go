// Package hyperbolic implements the Hyperbolic provider adapter.
// Hyperbolic provides GPU cloud and inference services.
// API Reference: https://docs.hyperbolic.xyz/
package hyperbolic

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "hyperbolic"

	// DefaultBaseURL is the default Hyperbolic API endpoint.
	DefaultBaseURL = "https://api.hyperbolic.xyz/v1"
)

// DefaultModels lists available Hyperbolic models.
var DefaultModels = []string{
	"meta-llama/Meta-Llama-3.1-70B-Instruct",
	"meta-llama/Meta-Llama-3.1-8B-Instruct",
	"Qwen/Qwen2.5-72B-Instruct",
}

// New creates a new Hyperbolic provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"meta-llama/",
			"Qwen/",
			"mistralai/",
		},
	}
	return openailike.New(cfg, info)
}
