// Package databricks implements the Databricks Foundation Model API provider adapter.
// Databricks provides access to various models through their unified API.
// API Reference: https://docs.databricks.com/en/machine-learning/foundation-models/index.html
package databricks

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "databricks"

	// DefaultBaseURL is a placeholder; actual URL depends on workspace.
	DefaultBaseURL = "https://your-workspace.databricks.com/serving-endpoints"
)

// DefaultModels lists available Databricks models.
var DefaultModels = []string{
	"databricks-dbrx-instruct",
	"databricks-meta-llama-3-1-70b-instruct",
	"databricks-meta-llama-3-1-405b-instruct",
	"databricks-mixtral-8x7b-instruct",
}

// New creates a new Databricks provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"databricks-",
		},
	}
	return openailike.New(cfg, info)
}
