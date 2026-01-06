// Package nvidia implements the NVIDIA NIM provider adapter.
// NVIDIA NIM provides optimized inference for various models.
// API Reference: https://docs.api.nvidia.com/
package nvidia

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "nvidia"

	// DefaultBaseURL is the default NVIDIA NIM API endpoint.
	DefaultBaseURL = "https://integrate.api.nvidia.com/v1"
)

// DefaultModels lists available NVIDIA NIM models.
var DefaultModels = []string{
	"nvidia/llama-3.1-nemotron-70b-instruct",
	"meta/llama-3.1-70b-instruct",
	"meta/llama-3.1-8b-instruct",
	"mistralai/mixtral-8x7b-instruct-v0.1",
	"google/gemma-7b",
	"google/codegemma-7b",
}

// New creates a new NVIDIA NIM provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"nvidia/",
			"meta/",
			"mistralai/",
			"google/",
		},
	}
	return openailike.New(cfg, info)
}
