// Package lambda implements the Lambda Labs provider adapter.
// Lambda Labs provides GPU cloud and inference services.
// API Reference: https://docs.lambdalabs.com/
package lambda

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "lambda"

	// DefaultBaseURL is the default Lambda Labs API endpoint.
	DefaultBaseURL = "https://api.lambdalabs.com/v1"
)

// DefaultModels lists available Lambda Labs models.
var DefaultModels = []string{
	"llama3.1-70b-instruct-fp8",
	"llama3.1-8b-instruct-fp8",
	"hermes-3-llama-3.1-405b-fp8",
}

// New creates a new Lambda Labs provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"llama",
			"hermes",
		},
	}
	return openailike.New(cfg, info)
}
