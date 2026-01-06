// Package github implements the GitHub Models provider adapter.
// GitHub Models provides free access to various models for developers.
// API Reference: https://docs.github.com/en/github-models
package github

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "github"

	// DefaultBaseURL is the default GitHub Models API endpoint.
	DefaultBaseURL = "https://models.inference.ai.azure.com"
)

// DefaultModels lists available GitHub Models.
var DefaultModels = []string{
	"gpt-4o",
	"gpt-4o-mini",
	"o1-preview",
	"o1-mini",
	"Phi-3-medium-128k-instruct",
	"Llama-3.2-90B-Vision-Instruct",
	"Mistral-large-2407",
}

// New creates a new GitHub Models provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"gpt-",
			"o1-",
			"Phi-",
			"Llama-",
			"Mistral-",
		},
	}
	return openailike.New(cfg, info)
}
