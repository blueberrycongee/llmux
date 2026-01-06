// Package snowflake implements the Snowflake Cortex provider adapter.
// Snowflake Cortex provides LLM inference within the Snowflake platform.
// API Reference: https://docs.snowflake.com/en/user-guide/snowflake-cortex/llm-functions
package snowflake

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "snowflake"

	// DefaultBaseURL is a placeholder; actual URL depends on account.
	DefaultBaseURL = "https://your-account.snowflakecomputing.com/api/v2"
)

// DefaultModels lists available Snowflake Cortex models.
var DefaultModels = []string{
	"snowflake-arctic",
	"llama3-8b",
	"llama3-70b",
	"mistral-large",
	"reka-flash",
}

// New creates a new Snowflake Cortex provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"snowflake-",
			"llama3",
			"mistral",
			"reka",
		},
	}
	return openailike.New(cfg, info)
}
