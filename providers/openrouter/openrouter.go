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

// DefaultModels lists some commonly used OpenRouter models.
var DefaultModels = []string{
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"anthropic/claude-3.5-sonnet",
	"anthropic/claude-3-opus",
	"google/gemini-pro-1.5",
	"meta-llama/llama-3.1-70b-instruct",
	"mistralai/mistral-large",
	"qwen/qwen-2.5-72b-instruct",
}

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
