// Package deepseek provides the DeepSeek provider for LLMux library mode.
// DeepSeek provides high-performance inference for their DeepSeek models.
// API Reference: https://platform.deepseek.com/api-docs
package deepseek

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "deepseek"

	// DefaultBaseURL is the default DeepSeek API endpoint.
	DefaultBaseURL = "https://api.deepseek.com"
)

// DefaultModels lists the available DeepSeek models.
var DefaultModels = []string{
	"deepseek-chat",
	"deepseek-coder",
	"deepseek-reasoner",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	SupportsEmbedding: false, // DeepSeek primarily supports chat
	ModelPrefixes:     []string{"deepseek"},
}

// Provider wraps the OpenAI-like provider for DeepSeek.
type Provider struct {
	*openailike.Provider
}

// New creates a new DeepSeek provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
