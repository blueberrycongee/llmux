package semantic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "text-embedding-ada-002", cfg.EmbeddingModel)
	assert.Equal(t, "openai", cfg.EmbeddingProvider)
	assert.Equal(t, "qdrant", cfg.VectorStore)
	assert.Equal(t, 1536, cfg.VectorDimension)
	assert.Equal(t, 0.95, cfg.SimilarityThreshold)
	assert.Equal(t, time.Hour, cfg.DefaultTTL)
	assert.Equal(t, "llmux_semantic_cache", cfg.QdrantCollection)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "should pass with valid qdrant config",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "qdrant",
				VectorDimension:     1536,
				SimilarityThreshold: 0.95,
				QdrantAPIBase:       "http://localhost:6333",
				QdrantCollection:    "test_collection",
			},
			wantErr: "",
		},
		{
			name: "should fail with invalid similarity threshold (too low)",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "qdrant",
				VectorDimension:     1536,
				SimilarityThreshold: 0,
				QdrantAPIBase:       "http://localhost:6333",
				QdrantCollection:    "test_collection",
			},
			wantErr: "similarity_threshold must be between 0 and 1",
		},
		{
			name: "should fail with invalid similarity threshold (too high)",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "qdrant",
				VectorDimension:     1536,
				SimilarityThreshold: 1.5,
				QdrantAPIBase:       "http://localhost:6333",
				QdrantCollection:    "test_collection",
			},
			wantErr: "similarity_threshold must be between 0 and 1",
		},
		{
			name: "should fail with invalid vector dimension",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "qdrant",
				VectorDimension:     0,
				SimilarityThreshold: 0.95,
				QdrantAPIBase:       "http://localhost:6333",
				QdrantCollection:    "test_collection",
			},
			wantErr: "vector_dimension must be positive",
		},
		{
			name: "should fail with missing embedding model",
			config: Config{
				EmbeddingModel:      "",
				VectorStore:         "qdrant",
				VectorDimension:     1536,
				SimilarityThreshold: 0.95,
				QdrantAPIBase:       "http://localhost:6333",
				QdrantCollection:    "test_collection",
			},
			wantErr: "embedding_model is required",
		},
		{
			name: "should fail with missing qdrant api base",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "qdrant",
				VectorDimension:     1536,
				SimilarityThreshold: 0.95,
				QdrantAPIBase:       "",
				QdrantCollection:    "test_collection",
			},
			wantErr: "qdrant_api_base is required for qdrant vector store",
		},
		{
			name: "should fail with missing qdrant collection",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "qdrant",
				VectorDimension:     1536,
				SimilarityThreshold: 0.95,
				QdrantAPIBase:       "http://localhost:6333",
				QdrantCollection:    "",
			},
			wantErr: "qdrant_collection is required for qdrant vector store",
		},
		{
			name: "should fail with unsupported vector store",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "unknown",
				VectorDimension:     1536,
				SimilarityThreshold: 0.95,
			},
			wantErr: "unsupported vector_store: must be 'qdrant' or 'redis'",
		},
		{
			name: "should fail with missing redis url",
			config: Config{
				EmbeddingModel:      "text-embedding-ada-002",
				VectorStore:         "redis",
				VectorDimension:     1536,
				SimilarityThreshold: 0.95,
				RedisURL:            "",
			},
			wantErr: "redis_url is required for redis vector store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestConfigDistanceThreshold(t *testing.T) {
	tests := []struct {
		name                string
		similarityThreshold float64
		expectedDistance    float64
	}{
		{
			name:                "should convert 0.95 similarity to 0.05 distance",
			similarityThreshold: 0.95,
			expectedDistance:    0.05,
		},
		{
			name:                "should convert 0.8 similarity to 0.2 distance",
			similarityThreshold: 0.8,
			expectedDistance:    0.2,
		},
		{
			name:                "should convert 1.0 similarity to 0.0 distance",
			similarityThreshold: 1.0,
			expectedDistance:    0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{SimilarityThreshold: tt.similarityThreshold}
			assert.InDelta(t, tt.expectedDistance, cfg.DistanceThreshold(), 0.0001)
		})
	}
}
