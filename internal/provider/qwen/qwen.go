// Package qwen implements the Alibaba Qwen provider adapter.
// Alibaba Cloud provides the Qwen series of large language models.
// API Reference: https://help.aliyun.com/document_detail/2712195.html
package qwen

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "qwen"

	// DefaultBaseURL is the default DashScope API endpoint.
	DefaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
)

// DefaultModels lists available Qwen models.
var DefaultModels = []string{
	"qwen-turbo",
	"qwen-plus",
	"qwen-max",
	"qwen-max-longcontext",
	"qwen-vl-plus",
	"qwen-vl-max",
	"qwen2.5-72b-instruct",
	"qwen2.5-32b-instruct",
	"qwen2.5-14b-instruct",
	"qwen2.5-7b-instruct",
	"qwen2.5-coder-32b-instruct",
}

// New creates a new Qwen provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"qwen",
		},
	}
	return openailike.New(cfg, info)
}
