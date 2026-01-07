// Package minimax provides the MiniMax provider for LLMux library mode.
package minimax

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "minimax"
	DefaultBaseURL = "https://api.minimax.chat/v1"
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"abab"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
