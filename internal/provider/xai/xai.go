// Package xai implements the xAI (Grok) provider adapter.
// xAI provides access to Grok models with an OpenAI-compatible API.
// API Reference: https://docs.x.ai/api
package xai

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "xai"

	// DefaultBaseURL is the default xAI API endpoint.
	DefaultBaseURL = "https://api.x.ai/v1"
)

// DefaultModels lists available xAI models.
var DefaultModels = []string{
	"grok-beta",
	"grok-2",
	"grok-2-mini",
}

// New creates a new xAI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"grok-",
		},
	}
	return openailike.New(cfg, info)
}
