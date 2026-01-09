package openailike

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSupportEmbedding_WhenSupportsEmbeddingTrue tests SupportEmbedding returns true when configured.
func TestSupportEmbedding_WhenSupportsEmbeddingTrue(t *testing.T) {
	info := Info{
		Name:              "test-provider",
		DefaultBaseURL:    "https://api.test.com",
		SupportsEmbedding: true,
	}

	provider := New(info)

	assert.True(t, provider.SupportEmbedding(), "should support embedding when SupportsEmbedding=true")
}

// TestSupportEmbedding_WhenSupportsEmbeddingFalse tests SupportEmbedding returns false when configured.
func TestSupportEmbedding_WhenSupportsEmbeddingFalse(t *testing.T) {
	info := Info{
		Name:              "test-provider",
		DefaultBaseURL:    "https://api.test.com",
		SupportsEmbedding: false,
	}

	provider := New(info)

	assert.False(t, provider.SupportEmbedding(), "should not support embedding when SupportsEmbedding=false")
}

// TestSupportEmbedding_DefaultValue tests SupportEmbedding returns false when not configured.
func TestSupportEmbedding_DefaultValue(t *testing.T) {
	info := Info{
		Name:           "test-provider",
		DefaultBaseURL: "https://api.test.com",
		// SupportsEmbedding not set - should default to false
	}

	provider := New(info)

	assert.False(t, provider.SupportEmbedding(), "should default to false when SupportsEmbedding not set")
}
