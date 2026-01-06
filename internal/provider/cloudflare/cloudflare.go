// Package cloudflare implements the Cloudflare Workers AI provider adapter.
// Cloudflare Workers AI provides serverless AI inference at the edge.
// API Reference: https://developers.cloudflare.com/workers-ai/
package cloudflare

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "cloudflare"

	// DefaultBaseURL uses the Cloudflare AI Gateway / Workers AI endpoint.
	// Users need to replace {account_id} with their actual Cloudflare account ID.
	DefaultBaseURL = "https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/v1"
)

// DefaultModels lists available Cloudflare Workers AI models.
var DefaultModels = []string{
	"@cf/meta/llama-3-8b-instruct",
	"@cf/meta/llama-3.1-8b-instruct",
	"@cf/meta/llama-3.1-70b-instruct",
	"@cf/mistral/mistral-7b-instruct-v0.2",
	"@cf/qwen/qwen1.5-14b-chat-awq",
	"@hf/thebloke/deepseek-coder-6.7b-instruct-awq",
}

// New creates a new Cloudflare Workers AI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"@cf/",
			"@hf/",
		},
	}
	return openailike.New(cfg, info)
}
