// Package deepinfra implements the DeepInfra provider adapter.
// DeepInfra provides scalable inference for open-source models.
// API Reference: https://deepinfra.com/docs
package deepinfra

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "deepinfra"

	// DefaultBaseURL is the default DeepInfra API endpoint.
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

// New creates a new DeepInfra provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"meta-llama/",
			"mistralai/",
			"Qwen/",
			"microsoft/",
			"01-ai/",
			"bigcode/",
			"google/",
		},
	}
	return openailike.New(cfg, info)
}
