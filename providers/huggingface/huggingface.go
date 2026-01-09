// Package huggingface provides the Hugging Face Inference API provider for LLMux library mode.
package huggingface

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "huggingface"
	DefaultBaseURL = "https://api-inference.huggingface.co/v1"
)

// DefaultModels lists commonly available Hugging Face models.
var DefaultModels = []string{
	"meta-llama/Meta-Llama-3.1-70B-Instruct",
	"meta-llama/Meta-Llama-3.1-8B-Instruct",
	"mistralai/Mixtral-8x7B-Instruct-v0.1",
	"microsoft/Phi-3-mini-4k-instruct",
	"Qwen/Qwen2.5-72B-Instruct",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"meta-llama/", "mistralai/", "Qwen/", "microsoft/"},
}

type Provider struct{ *openailike.Provider }

func New(opts ...openailike.Option) *Provider {
	return &Provider{Provider: openailike.New(providerInfo, opts...)}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
