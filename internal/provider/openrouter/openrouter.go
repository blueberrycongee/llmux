// Package openrouter implements the OpenRouter provider adapter.
// OpenRouter is an LLM aggregator that provides unified access to 100+ models.
// API Reference: https://openrouter.ai/docs
package openrouter

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "openrouter"

	// DefaultBaseURL is the default OpenRouter API endpoint.
	DefaultBaseURL = "https://openrouter.ai/api/v1"
)

// DefaultModels lists some commonly used OpenRouter models.
var DefaultModels = []string{
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"anthropic/claude-3.5-sonnet",
	"anthropic/claude-3-opus",
	"google/gemini-pro-1.5",
	"meta-llama/llama-3.1-70b-instruct",
	"mistralai/mistral-large",
	"qwen/qwen-2.5-72b-instruct",
}

// New creates a new OpenRouter provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ExtraHeaders: map[string]string{
			"HTTP-Referer": "https://github.com/blueberrycongee/llmux",
			"X-Title":      "LLMux",
		},
		ModelPrefixes: []string{
			"openai/",
			"anthropic/",
			"google/",
			"meta-llama/",
			"mistralai/",
			"qwen/",
			"cohere/",
			"perplexity/",
		},
	}
	return openailike.New(cfg, info)
}
