// Package together provides the Together AI provider for LLMux library mode.
// Together AI provides access to 100+ open-source models with fast inference.
// API Reference: https://docs.together.ai/reference
package together

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "together"

	// DefaultBaseURL is the default Together AI API endpoint.
	DefaultBaseURL = "https://api.together.xyz/v1"
)

// DefaultModels lists commonly available Together AI models.
var DefaultModels = []string{
	"meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
	"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
	"mistralai/Mixtral-8x7B-Instruct-v0.1",
	"Qwen/Qwen2.5-72B-Instruct-Turbo",
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	SupportsEmbedding: true, // Together AI supports embeddings (e.g., m2-bert-80M-8k-retrieval)
	ModelPrefixes:     []string{"meta-llama/", "mistralai/", "Qwen/", "codellama/", "deepseek-ai/"},
}

// Provider wraps the OpenAI-like provider for Together AI.
type Provider struct {
	*openailike.Provider
}

// New creates a new Together AI provider with the given options.
func New(opts ...openailike.Option) *Provider {
	return &Provider{
		Provider: openailike.New(providerInfo, opts...),
	}
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	return openailike.NewFromConfig(providerInfo, cfg)
}
