// Package mistral implements the Mistral AI provider adapter.
// Mistral AI provides state-of-the-art open-weight and commercial models.
// API Reference: https://docs.mistral.ai/api/
package mistral

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "mistral"

	// DefaultBaseURL is the default Mistral AI API endpoint.
	DefaultBaseURL = "https://api.mistral.ai/v1"
)

// DefaultModels lists available Mistral AI models.
var DefaultModels = []string{
	"mistral-large-latest",
	"mistral-medium-latest",
	"mistral-small-latest",
	"open-mistral-7b",
	"open-mixtral-8x7b",
	"open-mixtral-8x22b",
	"codestral-latest",
	"mistral-embed",
}

// New creates a new Mistral AI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"mistral-",
			"open-mistral-",
			"open-mixtral-",
			"codestral-",
		},
	}
	return openailike.New(cfg, info)
}
