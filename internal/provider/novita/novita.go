// Package novita implements the Novita AI provider adapter.
// Novita AI provides affordable LLM and image generation APIs.
// API Reference: https://novita.ai/docs/
package novita

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "novita"

	// DefaultBaseURL is the default Novita AI API endpoint.
	DefaultBaseURL = "https://api.novita.ai/v3/openai"
)

// DefaultModels lists available Novita AI models.
var DefaultModels = []string{
	"meta-llama/llama-3.1-8b-instruct",
	"meta-llama/llama-3.1-70b-instruct",
	"mistralai/mistral-7b-instruct",
	"microsoft/wizardlm-2-8x22b",
}

// New creates a new Novita AI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"meta-llama/",
			"mistralai/",
			"microsoft/",
		},
	}
	return openailike.New(cfg, info)
}
