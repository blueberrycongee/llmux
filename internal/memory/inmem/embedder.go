package inmem

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// SimpleEmbedder creates deterministic embeddings based on text hash.
// This ensures that "Apple" always has the same vector, and similar strings
// might NOT have similar vectors (since it's a hash), but it allows
// verifying the flow of data.
//
// For "Semantic" testing, we can implement a keyword-based heuristic if needed,
// but for integration testing, exact match or deterministic output is often enough.
// TODO: [Real Data Fetching] - Replace with real Embedding Model API (e.g. text-embedding-3-small) in production.
type SimpleEmbedder struct {
	Dimensions int
}

func NewSimpleEmbedder(dims int) *SimpleEmbedder {
	return &SimpleEmbedder{Dimensions: dims}
}

func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Use SHA256 to generate deterministic float values
	hash := sha256.Sum256([]byte(text))

	vec := make([]float32, e.Dimensions)
	for i := 0; i < e.Dimensions; i++ {
		// Take slices of the hash to generate floats
		// This is just to generate *some* numbers, not semantically meaningful
		start := (i * 4) % (len(hash) - 4)
		val := binary.BigEndian.Uint32(hash[start : start+4])
		// Normalize to 0-1 range roughly
		vec[i] = float32(val) / float32(math.MaxUint32)
	}

	// Normalize vector to unit length for cosine similarity
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec, nil
}
