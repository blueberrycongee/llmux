// Package watsonx provides the IBM watsonx.ai provider for LLMux library mode.
package watsonx

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "watsonx"
	DefaultBaseURL = "https://us-south.ml.cloud.ibm.com/ml/v1"
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"ibm/", "meta-llama/", "mistralai/"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
