# LLMux Cache Module

This directory contains the caching logic for LLMux, including both exact match and semantic caching.

## Cache Types

1. **Memory Cache**: Local in-memory cache for fast lookups.
2. **Redis Cache**: Distributed cache for multi-instance deployments.
3. **Dual Cache**: Combined memory and Redis cache with local-first strategy.
4. **Semantic Cache**: Similarity-based cache using vector embeddings.

## Semantic Cache Pipeline

The semantic cache allows for hits even when the prompt is not identical but semantically similar. It follows a multi-stage pipeline:

1. **Prompt Processing**: The incoming prompt is cleaned and prepared.
2. **Embedding Generation**: The prompt is converted into a high-dimensional vector using a configured embedding model (e.g., OpenAI's `text-embedding-3-small`).
3. **Vector Retrieval**:
    - The vector is used to query a vector database (e.g., Qdrant).
    - If re-ranking is disabled, it retrieves the Top-1 result.
    - If re-ranking is enabled, it retrieves the Top-N (default 5) candidates.
4. **Primary Filtering**: Candidates are filtered based on the `SimilarityThreshold` (Cosine similarity).
5. **Re-ranking (Optional)**:
    - If enabled, candidates are further evaluated using a secondary metric (e.g., Jaccard string similarity).
    - This ensures that among semantically similar results, we pick the one that is most textually aligned with the original prompt, reducing "hallucinated" cache hits.
    - Results must pass the `RerankingThreshold` to be considered a hit.
6. **Result Return**: The cached response from the best-matched candidate is returned.

## Configuration

Semantic cache can be configured via `semantic.Config`:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `SimilarityThreshold` | Minimum cosine similarity for vector search | 0.95 |
| `EnableReranking` | Enable secondary string similarity check | `false` |
| `RerankingThreshold` | Minimum secondary similarity score | 0.8 |
| `EmbeddingModel` | Model used for embeddings | `text-embedding-ada-002` |
| `VectorStore` | Backend for vector storage (e.g., `qdrant`) | `qdrant` |
