// Package watsonx implements the IBM watsonx.ai provider adapter.
// IBM watsonx.ai provides enterprise AI with Granite and other models.
// API Reference: https://cloud.ibm.com/apidocs/watsonx-ai
package watsonx

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "watsonx"

	// DefaultBaseURL is the default watsonx.ai API endpoint.
	DefaultBaseURL = "https://us-south.ml.cloud.ibm.com/ml/v1"
)

// DefaultModels lists available watsonx.ai models.
var DefaultModels = []string{
	"ibm/granite-13b-chat-v2",
	"ibm/granite-34b-code-instruct",
	"meta-llama/llama-3-70b-instruct",
	"meta-llama/llama-3-8b-instruct",
	"mistralai/mixtral-8x7b-instruct-v01",
}

// New creates a new watsonx.ai provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"ibm/",
			"meta-llama/",
			"mistralai/",
		},
	}
	return openailike.New(cfg, info)
}
