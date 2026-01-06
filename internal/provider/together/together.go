// Package together implements the Together AI provider adapter.
// Together AI provides access to 100+ open-source models with fast inference.
// API Reference: https://docs.together.ai/reference
package together

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "together"

	// DefaultBaseURL is the default Together AI API endpoint.
	DefaultBaseURL = "https://api.together.xyz/v1"
)

// DefaultModels lists commonly available Together AI models.
var DefaultModels = []string{
	"meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
	"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
	"meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo",
	"mistralai/Mixtral-8x7B-Instruct-v0.1",
	"mistralai/Mistral-7B-Instruct-v0.2",
	"Qwen/Qwen2.5-72B-Instruct-Turbo",
	"Qwen/Qwen2.5-7B-Instruct-Turbo",
	"codellama/CodeLlama-34b-Instruct-hf",
	"deepseek-ai/deepseek-coder-33b-instruct",
}

// New creates a new Together AI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"meta-llama/",
			"mistralai/",
			"Qwen/",
			"codellama/",
			"deepseek-ai/",
			"togethercomputer/",
			"NousResearch/",
		},
	}
	return openailike.New(cfg, info)
}
