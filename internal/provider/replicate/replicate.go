// Package replicate implements the Replicate provider adapter.
// Replicate provides access to open-source models via their API.
// API Reference: https://replicate.com/docs/reference/http
package replicate

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "replicate"

	// DefaultBaseURL is the default Replicate API endpoint.
	DefaultBaseURL = "https://api.replicate.com/v1"
)

// DefaultModels lists commonly available Replicate models.
var DefaultModels = []string{
	"meta/meta-llama-3-70b-instruct",
	"meta/meta-llama-3-8b-instruct",
	"mistralai/mixtral-8x7b-instruct-v0.1",
	"mistralai/mistral-7b-instruct-v0.2",
}

// New creates a new Replicate provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ChatEndpoint:      "/models/meta/meta-llama-3-70b-instruct/predictions",
		ModelPrefixes: []string{
			"meta/",
			"mistralai/",
		},
	}
	return openailike.New(cfg, info)
}
