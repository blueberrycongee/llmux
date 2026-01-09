// Package volcengine provides the ByteDance Volcengine (火山引擎) provider for LLMux library mode.
package volcengine

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "volcengine"
	DefaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
)

// DefaultModels lists available Volcengine models.
var DefaultModels = []string{
	"doubao-pro-32k",
	"doubao-pro-128k",
	"doubao-lite-32k",
	"doubao-lite-128k",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"doubao-", "skylark-"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
