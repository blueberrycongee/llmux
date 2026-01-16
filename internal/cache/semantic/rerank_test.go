package semantic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/blueberrycongee/llmux/internal/cache/semantic/vector"
)

func TestCalculateStringSimilarity(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected float64
	}{
		{"hello world", "hello world", 1.0},
		{"hello world", "HELLO WORLD", 1.0},
		{"hello world", "world hello", 1.0},
		{"hello world", "hello", 0.5},
		{"apple banana", "apple orange", 1.0 / 3.0},
		{"", "test", 0.0},
		{"test", "", 0.0},
	}

	for _, tt := range tests {
		score := CalculateStringSimilarity(tt.s1, tt.s2)
		assert.InDelta(t, tt.expected, score, 0.001, "Failed for %s and %s", tt.s1, tt.s2)
	}
}

func TestCacheReranking(t *testing.T) {
	ctx := context.Background()
	testVector := []float64{0.1, 0.2, 0.3}

	t.Run("should select better match using reranking", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		prompt := "What is the capital of France?"

		// Vector store returns two results, both with high vector similarity
		// But result 2 is a much better string match
		storeResults := []vector.SearchResult{
			{
				ID:    "id1",
				Score: 0.99, // Very high vector similarity
				Payload: vector.Payload{
					Prompt:   "Tell me about the capital of Germany",
					Response: "Berlin",
				},
			},
			{
				ID:    "id2",
				Score: 0.98, // Slightly lower but still high
				Payload: vector.Payload{
					Prompt:   "What is the capital city of France?",
					Response: "Paris",
				},
			},
		}

		embedder.On("Embed", ctx, prompt).Return(testVector, nil)
		store.On("Search", ctx, testVector, mock.MatchedBy(func(opts vector.SearchOptions) bool {
			return opts.TopK == 5
		})).Return(storeResults, nil)

		cfg := DefaultConfig()
		cfg.EnableReranking = true
		cfg.SimilarityThreshold = 0.95
		cfg.RerankingThreshold = 0.5

		cache, _ := New(embedder, store, cfg)

		result, err := cache.Get(ctx, prompt)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Should pick the one with better string similarity ("Paris"), not the one with higher vector score ("Berlin")
		assert.Equal(t, "Paris", result.Response)
		assert.Equal(t, "What is the capital city of France?", result.CachedPrompt)
	})

	t.Run("should return nil if no candidate passes reranking threshold", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		prompt := "What is the capital of France?"
		storeResults := []vector.SearchResult{
			{
				ID:    "id1",
				Score: 0.99,
				Payload: vector.Payload{
					Prompt:   "Completely different topic but high vector score",
					Response: "Some response",
				},
			},
		}

		embedder.On("Embed", ctx, prompt).Return(testVector, nil)
		store.On("Search", ctx, testVector, mock.Anything).Return(storeResults, nil)

		cfg := DefaultConfig()
		cfg.EnableReranking = true
		cfg.RerankingThreshold = 0.8 // Strict reranking

		cache, _ := New(embedder, store, cfg)

		result, err := cache.Get(ctx, prompt)
		assert.NoError(t, err)
		assert.Nil(t, result) // Fails reranking
	})

	t.Run("should work without reranking (baseline)", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		prompt := "What is the capital of France?"
		storeResults := []vector.SearchResult{
			{
				ID:    "id1",
				Score: 0.99,
				Payload: vector.Payload{
					Prompt:   "Tell me about the capital of Germany",
					Response: "Berlin",
				},
			},
		}

		embedder.On("Embed", ctx, prompt).Return(testVector, nil)
		store.On("Search", ctx, testVector, mock.MatchedBy(func(opts vector.SearchOptions) bool {
			return opts.TopK == 1
		})).Return(storeResults, nil)

		cfg := DefaultConfig()
		cfg.EnableReranking = false // Disable reranking
		cfg.SimilarityThreshold = 0.95

		cache, _ := New(embedder, store, cfg)

		result, err := cache.Get(ctx, prompt)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Berlin", result.Response) // Picks top vector match regardless of string content
	})
}
