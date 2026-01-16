// Package semantic provides semantic caching functionality using vector similarity.
// It allows caching LLM responses based on semantic similarity of prompts,
// enabling cache hits even when prompts are not identical but carry similar meaning.
package semantic

import (
	"errors"
	"time"
)

// Config holds configuration for semantic cache.
type Config struct {
	// Embedding configuration
	EmbeddingModel    string `yaml:"embedding_model"`    // Model for generating embeddings, e.g., "text-embedding-ada-002"
	EmbeddingProvider string `yaml:"embedding_provider"` // Provider: "openai", "azure", "local"
	EmbeddingAPIKey   string `yaml:"embedding_api_key"`  // API key for embedding provider
	EmbeddingAPIBase  string `yaml:"embedding_api_base"` // API base URL (optional)

	// Vector store configuration
	VectorStore     string `yaml:"vector_store"`     // Vector store type: "qdrant", "redis"
	VectorDimension int    `yaml:"vector_dimension"` // Vector dimension, default 1536 for ada-002

	// Qdrant configuration
	QdrantAPIBase    string `yaml:"qdrant_api_base"`   // Qdrant API base URL
	QdrantAPIKey     string `yaml:"qdrant_api_key"`    // Qdrant API key (optional)
	QdrantCollection string `yaml:"qdrant_collection"` // Qdrant collection name

	// Redis Vector Search configuration (for future use)
	RedisURL   string `yaml:"redis_url"`   // Redis URL for vector search
	RedisIndex string `yaml:"redis_index"` // Redis index name

	// Similarity configuration
	SimilarityThreshold float64 `yaml:"similarity_threshold"` // Threshold for cache hit (0.0-1.0), default 0.95

	// Cache behavior
	DefaultTTL time.Duration `yaml:"default_ttl"` // Default TTL for cached entries

	// Re-ranking configuration
	EnableReranking    bool    `yaml:"enable_reranking"`    // Enable secondary re-ranking
	RerankingThreshold float64 `yaml:"reranking_threshold"` // Threshold for re-ranking (0.0-1.0), default 0.8
}

// DefaultConfig returns sensible defaults for semantic cache.
func DefaultConfig() Config {
	return Config{
		EmbeddingModel:      "text-embedding-ada-002",
		EmbeddingProvider:   "openai",
		VectorStore:         "qdrant",
		VectorDimension:     1536,
		SimilarityThreshold: 0.95,
		DefaultTTL:          time.Hour,
		QdrantCollection:    "llmux_semantic_cache",
		EnableReranking:     false,
		RerankingThreshold:  0.8,
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.SimilarityThreshold <= 0 || c.SimilarityThreshold > 1 {
		return errors.New("similarity_threshold must be between 0 and 1")
	}

	if c.EnableReranking && (c.RerankingThreshold <= 0 || c.RerankingThreshold > 1) {
		return errors.New("reranking_threshold must be between 0 and 1 when reranking is enabled")
	}

	if c.VectorDimension <= 0 {
		return errors.New("vector_dimension must be positive")
	}

	if c.EmbeddingModel == "" {
		return errors.New("embedding_model is required")
	}

	switch c.VectorStore {
	case "qdrant":
		if c.QdrantAPIBase == "" {
			return errors.New("qdrant_api_base is required for qdrant vector store")
		}
		if c.QdrantCollection == "" {
			return errors.New("qdrant_collection is required for qdrant vector store")
		}
	case "redis":
		if c.RedisURL == "" {
			return errors.New("redis_url is required for redis vector store")
		}
	default:
		return errors.New("unsupported vector_store: must be 'qdrant' or 'redis'")
	}

	return nil
}

// DistanceThreshold converts similarity threshold to distance threshold.
// For cosine distance: 0 = most similar, 2 = least similar
// While similarity: 1 = most similar, 0 = least similar
func (c *Config) DistanceThreshold() float64 {
	return 1 - c.SimilarityThreshold
}
