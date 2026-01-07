// Package databricks provides the Databricks provider for LLMux library mode.
package databricks

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "databricks"
	DefaultBaseURL = "" // Requires workspace URL
)

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ChatEndpoint:      "/serving-endpoints/{endpoint}/invocations",
	ModelPrefixes:     []string{"databricks-"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
