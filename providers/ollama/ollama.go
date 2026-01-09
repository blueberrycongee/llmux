// Package ollama provides the Ollama provider for LLMux library mode.
package ollama

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "ollama"
	DefaultBaseURL = "http://localhost:11434/v1"
)

// DefaultModels lists common local Ollama models.
var DefaultModels = []string{
	"llama3.2",
	"llama3.1",
	"mistral",
	"mixtral",
	"codellama",
	"qwen2.5",
	"phi3",
	"gemma2",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"llama", "mistral", "qwen", "gemma", "phi"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
