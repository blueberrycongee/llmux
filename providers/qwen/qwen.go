// Package qwen provides the Alibaba Qwen (通义千问) provider for LLMux library mode.
package qwen

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "qwen"
	DefaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
)

// DefaultModels lists available Qwen models.
var DefaultModels = []string{
	"qwen-turbo",
	"qwen-plus",
	"qwen-max",
	"qwen-max-longcontext",
	"qwen-vl-plus",
	"qwen-vl-max",
	"qwen2.5-72b-instruct",
	"qwen2.5-32b-instruct",
	"qwen2.5-14b-instruct",
	"qwen2.5-7b-instruct",
	"qwen2.5-coder-32b-instruct",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"qwen-"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
