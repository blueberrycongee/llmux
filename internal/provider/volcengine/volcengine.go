// Package volcengine implements the Volcengine (DouBao) provider adapter.
// Volcengine provides ByteDance's Doubao models.
// API Reference: https://www.volcengine.com/docs/82379
package volcengine

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "volcengine"

	// DefaultBaseURL is the default Volcengine API endpoint.
	DefaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
)

// DefaultModels lists available Volcengine models.
var DefaultModels = []string{
	"doubao-pro-32k",
	"doubao-pro-128k",
	"doubao-lite-32k",
	"doubao-lite-128k",
}

// New creates a new Volcengine provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"doubao-",
			"ep-",
		},
	}
	return openailike.New(cfg, info)
}
