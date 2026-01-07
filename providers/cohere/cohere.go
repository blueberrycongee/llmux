// Package cohere provides the Cohere provider for LLMux library mode.
package cohere

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "cohere"
	DefaultBaseURL = "https://api.cohere.ai/v2"
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ChatEndpoint:      "/chat",
	ModelPrefixes:     []string{"command-"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
