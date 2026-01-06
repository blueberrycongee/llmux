// Package hunyuan implements the Tencent Hunyuan provider adapter.
// Tencent Cloud provides the Hunyuan series of large language models.
// API Reference: https://cloud.tencent.com/document/product/1729
package hunyuan

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "hunyuan"

	// DefaultBaseURL is the default Hunyuan API endpoint.
	DefaultBaseURL = "https://api.hunyuan.cloud.tencent.com/v1"
)

// DefaultModels lists available Hunyuan models.
var DefaultModels = []string{
	"hunyuan-lite",
	"hunyuan-standard",
	"hunyuan-standard-256K",
	"hunyuan-pro",
	"hunyuan-turbo",
	"hunyuan-turbo-latest",
	"hunyuan-vision",
}

// New creates a new Hunyuan provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"hunyuan",
		},
	}
	return openailike.New(cfg, info)
}
