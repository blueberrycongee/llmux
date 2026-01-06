// Package siliconflow implements the SiliconFlow provider adapter.
// SiliconFlow provides affordable access to various open-source models.
// API Reference: https://docs.siliconflow.cn/
package siliconflow

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "siliconflow"

	// DefaultBaseURL is the default SiliconFlow API endpoint.
	DefaultBaseURL = "https://api.siliconflow.cn/v1"
)

// DefaultModels lists commonly available SiliconFlow models.
var DefaultModels = []string{
	"Qwen/Qwen2.5-72B-Instruct",
	"Qwen/Qwen2.5-32B-Instruct",
	"Qwen/Qwen2.5-Coder-32B-Instruct",
	"deepseek-ai/DeepSeek-V3",
	"deepseek-ai/DeepSeek-R1",
	"Pro/Qwen/Qwen2.5-7B-Instruct",
	"meta-llama/Meta-Llama-3.1-70B-Instruct",
}

// New creates a new SiliconFlow provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"Qwen/",
			"deepseek-ai/",
			"Pro/",
			"meta-llama/",
			"THUDM/",
			"01-ai/",
		},
	}
	return openailike.New(cfg, info)
}
