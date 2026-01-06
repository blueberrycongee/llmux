// Package zhipu implements the Zhipu AI (ChatGLM) provider adapter.
// Zhipu AI provides the GLM series models with Chinese language support.
// API Reference: https://open.bigmodel.cn/dev/api
package zhipu

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "zhipu"

	// DefaultBaseURL is the default Zhipu AI API endpoint.
	DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
)

// DefaultModels lists available Zhipu AI models.
var DefaultModels = []string{
	"glm-4",
	"glm-4-plus",
	"glm-4v",
	"glm-4-air",
	"glm-4-airx",
	"glm-4-flash",
	"glm-4-long",
	"cogview-3",
}

// New creates a new Zhipu AI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"glm-",
			"cogview-",
		},
	}
	return openailike.New(cfg, info)
}
