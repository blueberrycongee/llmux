// Package deepinfra provides the DeepInfra provider for LLMux library mode.
package deepinfra

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "deepinfra"
	DefaultBaseURL = "https://api.deepinfra.com/v1/openai"
)

// DefaultModels lists commonly available DeepInfra models.
var DefaultModels = []string{
	"meta-llama/Meta-Llama-3.1-70B-Instruct",
	"meta-llama/Meta-Llama-3.1-8B-Instruct",
	"mistralai/Mixtral-8x7B-Instruct-v0.1",
	"mistralai/Mistral-7B-Instruct-v0.3",
	"Qwen/Qwen2.5-72B-Instruct",
	"microsoft/WizardLM-2-8x22B",
	"01-ai/Yi-34B-Chat",
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
