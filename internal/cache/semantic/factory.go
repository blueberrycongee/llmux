package semantic

import (
	"context"
	"fmt"

	"github.com/blueberrycongee/llmux/internal/cache/semantic/embedding"
	"github.com/blueberrycongee/llmux/internal/cache/semantic/vector"
)

// NewFromConfig creates a semantic cache from configuration.
func NewFromConfig(ctx context.Context, cfg Config) (*Cache, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create embedder
	embedder, err := createEmbedder(cfg)
	if err != nil {
		return nil, fmt.Errorf("create embedder: %w", err)
	}

	// Create vector store
	store, err := createVectorStore(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	// Create cache
	return New(embedder, store, cfg)
}

// createEmbedder creates an embedder based on configuration.
func createEmbedder(cfg Config) (embedding.Embedder, error) {
	switch cfg.EmbeddingProvider {
	case "openai", "":
		return embedding.NewOpenAIEmbedder(embedding.OpenAIConfig{
			APIKey:    cfg.EmbeddingAPIKey,
			APIBase:   cfg.EmbeddingAPIBase,
			Model:     cfg.EmbeddingModel,
			Dimension: cfg.VectorDimension,
		})

	case "azure":
		return embedding.NewAzureEmbedder(embedding.AzureConfig{
			APIKey:     cfg.EmbeddingAPIKey,
			APIBase:    cfg.EmbeddingAPIBase,
			Deployment: cfg.EmbeddingModel,
			Dimension:  cfg.VectorDimension,
		})

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.EmbeddingProvider)
	}
}

// createVectorStore creates a vector store based on configuration.
func createVectorStore(ctx context.Context, cfg Config) (vector.Store, error) {
	switch cfg.VectorStore {
	case "qdrant", "":
		store, err := vector.NewQdrantStore(vector.QdrantConfig{
			APIBase:    cfg.QdrantAPIBase,
			APIKey:     cfg.QdrantAPIKey,
			Collection: cfg.QdrantCollection,
			Dimension:  cfg.VectorDimension,
		})
		if err != nil {
			return nil, err
		}

		// Ensure collection exists
		if err := store.EnsureCollection(ctx); err != nil {
			return nil, fmt.Errorf("ensure collection: %w", err)
		}

		return store, nil

	default:
		return nil, fmt.Errorf("unsupported vector store: %s", cfg.VectorStore)
	}
}
