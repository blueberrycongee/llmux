// Package baichuan implements the Baichuan AI provider adapter.
// Baichuan provides Chinese-optimized large language models.
// API Reference: https://platform.baichuan-ai.com/docs
package baichuan

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "baichuan"

	// DefaultBaseURL is the default Baichuan API endpoint.
	DefaultBaseURL = "https://api.baichuan-ai.com/v1"
)

// DefaultModels lists available Baichuan models.
var DefaultModels = []string{
	"Baichuan4",
	"Baichuan3-Turbo",
	"Baichuan3-Turbo-128k",
	"Baichuan2-Turbo",
	"Baichuan2-Turbo-192k",
}

// New creates a new Baichuan provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"Baichuan",
		},
	}
	return openailike.New(cfg, info)
}
