// Package sambanova provides the SambaNova provider for LLMux library mode.
package sambanova

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "sambanova"
	DefaultBaseURL = "https://api.sambanova.ai/v1"
)

// DefaultModels lists available SambaNova models.
var DefaultModels = []string{
	"Meta-Llama-3.1-8B-Instruct",
	"Meta-Llama-3.1-70B-Instruct",
	"Meta-Llama-3.1-405B-Instruct",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"Meta-Llama-", "Qwen"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
