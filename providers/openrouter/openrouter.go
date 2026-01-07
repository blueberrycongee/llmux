// Package openrouter provides the OpenRouter provider for LLMux library mode.
// OpenRouter provides unified access to multiple LLM providers.
// API Reference: https://openrouter.ai/docs
package openrouter

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "openrouter"

	// DefaultBaseURL is the default OpenRouter API endpoint.
	DefaultBaseURL = "https://openrouter.ai/api/v1"
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"openai/", "anthropic/", "google/", "meta-llama/", "mistralai/"},
}

// Provider wraps the OpenAI-like provider for OpenRouter.
type Provider struct {
	*openailike.Provider
}

// New creates a new OpenRouter provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
