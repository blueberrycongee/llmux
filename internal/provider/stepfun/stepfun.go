// Package stepfun implements the StepFun provider adapter.
// StepFun (阶跃星辰) provides the Step series of large language models.
// API Reference: https://platform.stepfun.com/docs/api
package stepfun

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "stepfun"

	// DefaultBaseURL is the default StepFun API endpoint.
	DefaultBaseURL = "https://api.stepfun.com/v1"
)

// DefaultModels lists available StepFun models.
var DefaultModels = []string{
	"step-2-16k",
	"step-1-8k",
	"step-1-32k",
	"step-1-128k",
	"step-1-256k",
	"step-1v-8k",
	"step-1v-32k",
}

// New creates a new StepFun provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"step-",
		},
	}
	return openailike.New(cfg, info)
}
