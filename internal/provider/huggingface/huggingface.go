// Package huggingface implements the Hugging Face Inference provider adapter.
// Hugging Face provides serverless inference for hosted models.
// API Reference: https://huggingface.co/docs/api-inference
package huggingface

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "huggingface"

	// DefaultBaseURL is the default Hugging Face API endpoint.
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

// New creates a new Hugging Face provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"meta-llama/",
			"mistralai/",
			"microsoft/",
			"Qwen/",
			"google/",
			"bigcode/",
		},
	}
	return openailike.New(cfg, info)
}
