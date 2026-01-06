// Package vertexai implements the Google Vertex AI provider adapter.
// Vertex AI provides enterprise access to Gemini and other models on Google Cloud.
// API Reference: https://cloud.google.com/vertex-ai/docs/generative-ai/model-reference/gemini
package vertexai

import (
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/openailike"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "vertexai"

	// DefaultBaseURL uses placeholder; actual URL depends on project and region.
	DefaultBaseURL = "https://us-central1-aiplatform.googleapis.com/v1"
)

// DefaultModels lists available Vertex AI models.
var DefaultModels = []string{
	"gemini-1.5-pro",
	"gemini-1.5-flash",
	"gemini-1.0-pro",
	"chat-bison",
	"codechat-bison",
}

// New creates a new Vertex AI provider instance.
// Note: Vertex AI typically requires OAuth2/service account authentication.
// This implementation uses a simplified API key approach.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	info := openailike.ProviderInfo{
		Name:              ProviderName,
		DefaultBaseURL:    DefaultBaseURL,
		SupportsStreaming: true,
		ModelPrefixes: []string{
			"gemini-",
			"chat-bison",
			"codechat-bison",
		},
	}
	return openailike.New(cfg, info)
}
