// Package anyscale implements the Anyscale provider adapter.
// Anyscale provides serverless endpoints for open-source models using Ray.
// API Reference: https://docs.anyscale.com/
package anyscale

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "anyscale"

	// DefaultBaseURL is the default Anyscale API endpoint.
	DefaultBaseURL = "https://api.endpoints.anyscale.com/v1"
)

// DefaultModels lists available Anyscale models.
var DefaultModels = []string{
	"meta-llama/Meta-Llama-3-70B-Instruct",
	"meta-llama/Meta-Llama-3-8B-Instruct",
	"mistralai/Mixtral-8x7B-Instruct-v0.1",
	"mistralai/Mistral-7B-Instruct-v0.1",
	"codellama/CodeLlama-70b-Instruct-hf",
}

// New creates a new Anyscale provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"meta-llama/",
			"mistralai/",
			"codellama/",
		},
	}
	return openailike.New(cfg, info)
}
