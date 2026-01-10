package inmem

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/memory"
)

// MemoryVectorStore is a simple thread-safe in-memory vector database.
// It performs brute-force cosine similarity search.
type MemoryVectorStore struct {
	mu      sync.RWMutex
	entries map[string]*memory.MemoryEntry
}

func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{
		entries: make(map[string]*memory.MemoryEntry),
	}
}

func (s *MemoryVectorStore) Add(ctx context.Context, entry *memory.MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deep copy metadata to avoid side effects
	newEntry := *entry
	if entry.Metadata != nil {
		newEntry.Metadata = make(map[string]interface{})
		for k, v := range entry.Metadata {
			newEntry.Metadata[k] = v
		}
	}
	s.entries[entry.ID] = &newEntry
	return nil
}

func (s *MemoryVectorStore) Search(ctx context.Context, queryVector []float32, filter memory.MemoryFilter) ([]*memory.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type result struct {
		entry *memory.MemoryEntry
		score float32
	}

	var results []result

	for _, entry := range s.entries {
		// Apply Filters
		if filter.UserID != "" && entry.UserID != filter.UserID {
			continue
		}
		if filter.AgentID != "" && entry.AgentID != filter.AgentID {
			continue
		}
		if filter.SessionID != "" && entry.SessionID != filter.SessionID {
			continue
		}

		if len(entry.Embedding) != len(queryVector) {
			continue // Skip mismatched dimensions
		}

		// 1. Semantic Score (Cosine Similarity)
		semanticScore := cosineSimilarity(queryVector, entry.Embedding)

		// 2. Recency Score (Exponential Decay)
		// Assuming memories within last 24h are very relevant, decaying over 30 days.
		// Decay function: e^(-lambda * days_passed)
		// Let's keep it simple: Linear boost for recent items?
		// Or sigmoid?
		// Mem0 uses a mix. Let's use a simple hours-based decay.
		hoursPassed := time.Since(entry.CreatedAt).Hours()
		recencyScore := float32(math.Exp(-0.01 * hoursPassed)) // 0.01 decay rate

		// 3. Final Hybrid Score
		// Weight: 80% Semantic, 20% Recency
		finalScore := semanticScore*0.8 + recencyScore*0.2

		results = append(results, result{entry: entry, score: finalScore})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Top K
	limit := filter.Limit
	if limit <= 0 {
		limit = 5 // Default limit
	}
	if limit > len(results) {
		limit = len(results)
	}
	final := make([]*memory.MemoryEntry, limit)
	for i := 0; i < limit; i++ {
		// Return copy
		e := *results[i].entry
		final[i] = &e
	}

	return final, nil
}

func (s *MemoryVectorStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
	return nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
