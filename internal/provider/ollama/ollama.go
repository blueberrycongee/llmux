// Package ollama implements the Ollama provider adapter.
// Ollama provides local LLM inference with an OpenAI-compatible API.
// API Reference: https://ollama.ai/docs/api
package ollama

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "ollama"

	// DefaultBaseURL is the default Ollama API endpoint (local).
	DefaultBaseURL = "http://localhost:11434/v1"
)

// DefaultModels lists common local Ollama models.
var DefaultModels = []string{
	"llama3.2",
	"llama3.1",
	"mistral",
	"mixtral",
	"codellama",
	"qwen2.5",
	"phi3",
	"gemma2",
}

// New creates a new Ollama provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		APIKeyHeader:      "", // Ollama doesn't require authentication by default
		ModelPrefixes: []string{
			"llama",
			"mistral",
			"mixtral",
			"codellama",
			"qwen",
			"phi",
			"gemma",
		},
	}
	return openailike.New(cfg, info)
}
