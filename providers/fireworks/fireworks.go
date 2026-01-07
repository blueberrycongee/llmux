// Package fireworks provides the Fireworks AI provider for LLMux library mode.
// Fireworks AI provides fast inference for open-source models.
// API Reference: https://docs.fireworks.ai/api-reference
package fireworks

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "fireworks"

	// DefaultBaseURL is the default Fireworks AI API endpoint.
	DefaultBaseURL = "https://api.fireworks.ai/inference/v1"
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"accounts/fireworks/models/"},
}

// Provider wraps the OpenAI-like provider for Fireworks AI.
type Provider struct {
	*openailike.Provider
}

// New creates a new Fireworks AI provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
