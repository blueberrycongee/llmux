// Package minimax implements the MiniMax provider adapter.
// MiniMax provides large language models and multimodal capabilities.
// API Reference: https://www.minimaxi.com/document
package minimax

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "minimax"

	// DefaultBaseURL is the default MiniMax API endpoint.
	DefaultBaseURL = "https://api.minimax.chat/v1"
)

// DefaultModels lists available MiniMax models.
var DefaultModels = []string{
	"abab6.5-chat",
	"abab6.5s-chat",
	"abab5.5-chat",
	"abab5.5s-chat",
}

// New creates a new MiniMax provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"abab",
		},
	}
	return openailike.New(cfg, info)
}
