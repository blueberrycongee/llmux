// Package sambanova implements the SambaNova provider adapter.
// SambaNova provides ultra-fast inference on their custom RDU hardware.
// API Reference: https://docs.sambanova.ai/
package sambanova

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "sambanova"

	// DefaultBaseURL is the default SambaNova API endpoint.
	DefaultBaseURL = "https://api.sambanova.ai/v1"
)

// DefaultModels lists available SambaNova models.
var DefaultModels = []string{
	"Meta-Llama-3.1-8B-Instruct",
	"Meta-Llama-3.1-70B-Instruct",
	"Meta-Llama-3.1-405B-Instruct",
}

// New creates a new SambaNova provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"Meta-Llama-",
			"Llama-",
		},
	}
	return openailike.New(cfg, info)
}
