// Package deepseek implements the DeepSeek provider adapter.
// DeepSeek provides high-performance inference for their DeepSeek models.
// API Reference: https://platform.deepseek.com/api-docs
package deepseek

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "deepseek"

	// DefaultBaseURL is the default DeepSeek API endpoint.
	DefaultBaseURL = "https://api.deepseek.com"
)

// DefaultModels lists the available DeepSeek models.
var DefaultModels = []string{
	"deepseek-chat",
	"deepseek-coder",
	"deepseek-reasoner",
}

// New creates a new DeepSeek provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"deepseek",
		},
	}
	return openailike.New(cfg, info)
}
