package openailike_test

import (
	"testing"

	"github.com/blueberrycongee/llmux/providers/deepseek"
	"github.com/blueberrycongee/llmux/providers/fireworks"
	"github.com/blueberrycongee/llmux/providers/groq"
	"github.com/blueberrycongee/llmux/providers/ollama"
	"github.com/blueberrycongee/llmux/providers/openrouter"
	"github.com/blueberrycongee/llmux/providers/together"
	"github.com/blueberrycongee/llmux/providers/vllm"
	"github.com/stretchr/testify/assert"
)

// TestProviderEmbeddingSupport verifies that providers correctly declare embedding support.
// This test validates the fix for P2: Missing configuration for embedding support.
func TestProviderEmbeddingSupport(t *testing.T) {
	testCases := []struct {
		name            string
		createProvider  func() interface{ SupportEmbedding() bool }
		expectedSupport bool
		reason          string
	}{
		{
			name: "groq",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return groq.New()
			},
			expectedSupport: false,
			reason:          "Groq does not support embeddings API",
		},
		{
			name: "deepseek",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return deepseek.New()
			},
			expectedSupport: false,
			reason:          "DeepSeek primarily supports chat",
		},
		{
			name: "vllm",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return vllm.New()
			},
			expectedSupport: false,
			reason:          "vLLM is primarily for chat/completion inference",
		},
		{
			name: "openrouter",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return openrouter.New()
			},
			expectedSupport: false,
			reason:          "OpenRouter is a router/aggregator",
		},
		{
			name: "ollama",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return ollama.New()
			},
			expectedSupport: true,
			reason:          "Ollama supports embeddings via local models",
		},
		{
			name: "together",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return together.New()
			},
			expectedSupport: true,
			reason:          "Together AI supports embeddings",
		},
		{
			name: "fireworks",
			createProvider: func() interface{ SupportEmbedding() bool } {
				return fireworks.New()
			},
			expectedSupport: true,
			reason:          "Fireworks AI supports embeddings",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := tc.createProvider()
			actual := provider.SupportEmbedding()
			assert.Equal(t, tc.expectedSupport, actual,
				"Provider %s should %ssupport embeddings: %s",
				tc.name,
				map[bool]string{true: "", false: "NOT "}[tc.expectedSupport],
				tc.reason,
			)
		})
	}
}

// TestEmbeddingSupportPreventsIncorrectCalls demonstrates how the fix prevents runtime errors.
func TestEmbeddingSupportPreventsIncorrectCalls(t *testing.T) {
	// Before the fix: Groq would return true for SupportEmbedding()
	// After the fix: Groq correctly returns false
	groqProvider := groq.New()

	if groqProvider.SupportEmbedding() {
		t.Fatal("Groq should not support embeddings, but SupportEmbedding() returned true")
	}

	// This allows client code to fail fast with a clear error message
	// instead of getting a 404/405 from the Groq API
	t.Log("✓ Client can now check SupportEmbedding() and fail fast with clear error")
}
