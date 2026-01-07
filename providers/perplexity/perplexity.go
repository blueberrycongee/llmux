// Package perplexity provides the Perplexity AI provider for LLMux library mode.
// Perplexity AI provides search-augmented LLM inference.
// API Reference: https://docs.perplexity.ai/reference
package perplexity

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "perplexity"

	// DefaultBaseURL is the default Perplexity API endpoint.
	DefaultBaseURL = "https://api.perplexity.ai"
)

// DefaultModels lists the available Perplexity models.
var DefaultModels = []string{
	"llama-3.1-sonar-small-128k-online",
	"llama-3.1-sonar-large-128k-online",
	"llama-3.1-sonar-huge-128k-online",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"llama-3.1-sonar", "pplx-"},
}

// Provider wraps the OpenAI-like provider for Perplexity.
type Provider struct {
	*openailike.Provider
}

// New creates a new Perplexity provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
