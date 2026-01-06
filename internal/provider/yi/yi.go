// Package yi implements the 01.AI (Yi) provider adapter.
// 01.AI provides the Yi series of large language models.
// API Reference: https://platform.01.ai/docs
package yi

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "yi"

	// DefaultBaseURL is the default Yi API endpoint.
	DefaultBaseURL = "https://api.01.ai/v1"
)

// DefaultModels lists available Yi models.
var DefaultModels = []string{
	"yi-lightning",
	"yi-large",
	"yi-medium",
	"yi-spark",
	"yi-large-turbo",
	"yi-large-rag",
	"yi-vision",
}

// New creates a new Yi provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"yi-",
		},
	}
	return openailike.New(cfg, info)
}
