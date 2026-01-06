// Package vllm implements the vLLM provider adapter.
// vLLM provides high-throughput LLM serving with an OpenAI-compatible API.
// API Reference: https://docs.vllm.ai/
package vllm

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "vllm"

	// DefaultBaseURL is the default vLLM API endpoint.
	DefaultBaseURL = "http://localhost:8000/v1"
)

// DefaultModels is empty as models depend on deployment.
var DefaultModels = []string{}

// New creates a new vLLM provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		APIKeyHeader:      "", // vLLM may not require authentication
		ModelPrefixes:     []string{}, // Models depend on deployment
	}
	return openailike.New(cfg, info)
}
