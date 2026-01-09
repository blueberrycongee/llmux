// Package siliconflow provides the SiliconFlow (硅基流动) provider for LLMux library mode.
package siliconflow

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "siliconflow"
	DefaultBaseURL = "https://api.siliconflow.cn/v1"
)

// DefaultModels lists commonly available SiliconFlow models.
var DefaultModels = []string{
	"Qwen/Qwen2.5-72B-Instruct",
	"Qwen/Qwen2.5-32B-Instruct",
	"Qwen/Qwen2.5-Coder-32B-Instruct",
	"deepseek-ai/DeepSeek-V3",
	"deepseek-ai/DeepSeek-R1",
	"Pro/Qwen/Qwen2.5-7B-Instruct",
	"meta-llama/Meta-Llama-3.1-70B-Instruct",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"Qwen/", "deepseek-ai/", "THUDM/", "01-ai/"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
