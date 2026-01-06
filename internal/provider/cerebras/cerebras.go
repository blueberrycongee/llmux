// Package cerebras implements the Cerebras provider adapter.
// Cerebras provides ultra-fast inference on their Wafer Scale Engine.
// API Reference: https://inference-docs.cerebras.ai/api-reference
package cerebras

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "cerebras"

	// DefaultBaseURL is the default Cerebras API endpoint.
	DefaultBaseURL = "https://api.cerebras.ai/v1"
)

// DefaultModels lists available Cerebras models.
var DefaultModels = []string{
	"llama3.1-8b",
	"llama3.1-70b",
}

// New creates a new Cerebras provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"llama3",
		},
	}
	return openailike.New(cfg, info)
}
