// Package moonshot implements the Moonshot AI (Kimi) provider adapter.
// Moonshot AI provides the Kimi models with long context support.
// API Reference: https://platform.moonshot.cn/docs/api
package moonshot

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "moonshot"

	// DefaultBaseURL is the default Moonshot API endpoint.
	DefaultBaseURL = "https://api.moonshot.cn/v1"
)

// DefaultModels lists available Moonshot models.
var DefaultModels = []string{
	"moonshot-v1-8k",
	"moonshot-v1-32k",
	"moonshot-v1-128k",
}

// New creates a new Moonshot provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"moonshot-",
		},
	}
	return openailike.New(cfg, info)
}
