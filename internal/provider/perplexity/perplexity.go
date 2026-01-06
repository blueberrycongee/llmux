// Package perplexity implements the Perplexity AI provider adapter.
// Perplexity provides AI-powered search and conversational models.
// API Reference: https://docs.perplexity.ai/reference
package perplexity

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "perplexity"

	// DefaultBaseURL is the default Perplexity API endpoint.
	DefaultBaseURL = "https://api.perplexity.ai"
)

// DefaultModels lists available Perplexity models.
var DefaultModels = []string{
	"llama-3.1-sonar-small-128k-online",
	"llama-3.1-sonar-large-128k-online",
	"llama-3.1-sonar-huge-128k-online",
	"llama-3.1-sonar-small-128k-chat",
	"llama-3.1-sonar-large-128k-chat",
}

// New creates a new Perplexity provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"llama-3.1-sonar",
			"pplx-",
			"sonar-",
		},
	}
	return openailike.New(cfg, info)
}
