// Package replicate provides the Replicate provider for LLMux library mode.
package replicate

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "replicate"
	DefaultBaseURL = "https://api.replicate.com/v1"
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"meta/", "mistralai/"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
