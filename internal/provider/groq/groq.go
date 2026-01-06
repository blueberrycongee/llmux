// Package groq implements the Groq provider adapter.
// Groq provides ultra-fast inference for open-source LLMs like Llama, Mixtral, and Gemma.
// API Reference: https://console.groq.com/docs/api-reference
package groq

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "groq"

	// DefaultBaseURL is the default Groq API endpoint.
	DefaultBaseURL = "https://api.groq.com/openai/v1"
)

// DefaultModels lists the commonly available Groq models.
var DefaultModels = []string{
	"llama-3.3-70b-versatile",
	"llama-3.1-70b-versatile",
	"llama-3.1-8b-instant",
	"llama3-70b-8192",
	"llama3-8b-8192",
	"mixtral-8x7b-32768",
	"gemma2-9b-it",
	"gemma-7b-it",
}

// New creates a new Groq provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"llama",
			"mixtral",
			"gemma",
		},
	}
	return openailike.New(cfg, info)
}
