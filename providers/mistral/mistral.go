// Package mistral provides the Mistral AI provider for LLMux library mode.
// Mistral AI provides high-performance inference for Mistral models.
// API Reference: https://docs.mistral.ai/api/
package mistral

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "mistral"

	// DefaultBaseURL is the default Mistral AI API endpoint.
	DefaultBaseURL = "https://api.mistral.ai/v1"
)

// DefaultModels lists the available Mistral models.
var DefaultModels = []string{
	"mistral-large-latest",
	"mistral-medium-latest",
	"mistral-small-latest",
	"open-mistral-7b",
	"open-mixtral-8x7b",
	"open-mixtral-8x22b",
	"codestral-latest",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"mistral-", "open-mistral", "open-mixtral", "codestral"},
}

// Provider wraps the OpenAI-like provider for Mistral AI.
type Provider struct {
	*openailike.Provider
}

// New creates a new Mistral AI provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
