// Package lmstudio implements the LM Studio provider adapter.
// LM Studio provides local LLM inference with an OpenAI-compatible API.
// API Reference: https://lmstudio.ai/docs/api
package lmstudio

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "lmstudio"

	// DefaultBaseURL is the default LM Studio API endpoint (local).
	DefaultBaseURL = "http://localhost:1234/v1"
)

// DefaultModels is empty as models depend on user's local setup.
var DefaultModels = []string{}

// New creates a new LM Studio provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		APIKeyHeader:      "",         // LM Studio doesn't require authentication
		ModelPrefixes:     []string{}, // Models depend on local setup
	}
	return openailike.New(cfg, info)
}
