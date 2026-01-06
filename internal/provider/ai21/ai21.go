// Package ai21 implements the AI21 Labs provider adapter.
// AI21 Labs provides the Jurassic and Jamba series of foundation models.
// API Reference: https://docs.ai21.com/reference
package ai21

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "ai21"

	// DefaultBaseURL is the default AI21 API endpoint.
	DefaultBaseURL = "https://api.ai21.com/studio/v1"
)

// DefaultModels lists available AI21 models.
var DefaultModels = []string{
	"jamba-1.5-large",
	"jamba-1.5-mini",
	"jamba-instruct",
	"j2-ultra",
	"j2-mid",
}

// New creates a new AI21 provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		ChatEndpoint:      "/chat/completions",
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"jamba-",
			"j2-",
		},
	}
	return openailike.New(cfg, info)
}
