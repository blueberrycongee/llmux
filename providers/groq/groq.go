// Package groq provides the Groq provider for LLMux library mode.
// Groq provides ultra-fast inference for open-source LLMs.
// API Reference: https://console.groq.com/docs/api-reference
package groq

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "groq"

	// DefaultBaseURL is the default Groq API endpoint.
	DefaultBaseURL = "https://api.groq.com/openai/v1"
)

// DefaultModels lists the commonly available Groq models.
var DefaultModels = []string{
	"llama-3.3-70b-versatile",
	"llama-3.1-70b-versatile",
	"llama-3.1-8b-instant",
	"llama3-70b-8192",
	"llama3-8b-8192",
	"mixtral-8x7b-32768",
	"gemma2-9b-it",
	"gemma-7b-it",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"llama", "mixtral", "gemma"},
}

// Provider wraps the OpenAI-like provider for Groq.
type Provider struct {
	*openailike.Provider
}

// New creates a new Groq provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
