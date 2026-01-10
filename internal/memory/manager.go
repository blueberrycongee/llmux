package memory

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Embedder defines the interface for generating vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// MemoryManager implements the Manager interface.
// It orchestrates Short-term (Session) and Long-term (Vector) memory.
type MemoryManager struct {
	sessionStore SessionStore
	vectorStore  VectorStore
	embedder     Embedder
	extractor    *Extractor
}

// NewMemoryManager creates a new instance of MemoryManager.
// Optionally pass LLMClient to enable Smart Ingestion.
// TODO: [Real Data Fetching] - Wire up real components (VectorStore, LLMClient, Embedder) in main.go instead of in-memory ones.
func NewMemoryManager(sessionStore SessionStore, vectorStore VectorStore, embedder Embedder, llmClient LLMClient) *MemoryManager {
	var extractor *Extractor
	if llmClient != nil {
		extractor = NewExtractor(llmClient, "gpt-4o") // Default model, can be configured
	}
	return &MemoryManager{
		sessionStore: sessionStore,
		vectorStore:  vectorStore,
		embedder:     embedder,
		extractor:    extractor,
	}
}

// --- SessionStore Delegation ---

func (m *MemoryManager) CreateSession(ctx context.Context, session *Session) error {
	return m.sessionStore.CreateSession(ctx, session)
}

func (m *MemoryManager) GetSession(ctx context.Context, id string) (*Session, error) {
	return m.sessionStore.GetSession(ctx, id)
}

func (m *MemoryManager) UpdateSession(ctx context.Context, session *Session) error {
	return m.sessionStore.UpdateSession(ctx, session)
}

func (m *MemoryManager) DeleteSession(ctx context.Context, id string) error {
	return m.sessionStore.DeleteSession(ctx, id)
}

func (m *MemoryManager) ListUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	return m.sessionStore.ListUserSessions(ctx, userID)
}

// --- RAG & Long-term Memory Implementation ---

// RetrieveRelevantContext performs the "Retrieve" and "Augment" steps of RAG.
func (m *MemoryManager) RetrieveRelevantContext(ctx context.Context, query string, filter MemoryFilter) (string, error) {
	// 1. Generate embedding for the query
	queryVec, err := m.embedder.Embed(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to embed query: %w", err)
	}

	// 2. Retrieve relevant memories from VectorStore
	// Defaulting to top 5 results if limit not set
	if filter.Limit == 0 {
		filter.Limit = 5
	}
	memories, err := m.vectorStore.Search(ctx, queryVec, filter)
	if err != nil {
		return "", fmt.Errorf("failed to search vector store: %w", err)
	}

	if len(memories) == 0 {
		return "", nil
	}

	// 3. Format memories into a context string
	var sb strings.Builder
	sb.WriteString("Relevant Memories:\n")
	for _, mem := range memories {
		sb.WriteString(fmt.Sprintf("- %s\n", mem.Content))
	}

	return sb.String(), nil
}

// AddMemory stores a new fact or experience into long-term memory.
func (m *MemoryManager) AddMemory(ctx context.Context, entry *MemoryEntry) error {
	// 1. Generate embedding for the content if missing
	if len(entry.Embedding) == 0 {
		vec, err := m.embedder.Embed(ctx, entry.Content)
		if err != nil {
			return fmt.Errorf("failed to embed content: %w", err)
		}
		entry.Embedding = vec
	}

	// 2. Set defaults if missing
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// 3. Persist to VectorStore
	if err := m.vectorStore.Add(ctx, entry); err != nil {
		return fmt.Errorf("failed to add memory to vector store: %w", err)
	}

	return nil
}

// IngestMemory processes raw text using the Extractor to identify facts,
// and then stores them as memory entries.
func (m *MemoryManager) IngestMemory(ctx context.Context, content string, scope MemoryFilter) error {
	if m.extractor == nil {
		return fmt.Errorf("smart ingestion not enabled: no LLM client provided")
	}

	// 1. Extract facts from raw text
	facts, err := m.extractor.Extract(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to extract facts: %w", err)
	}

	// 2. Store each fact as a memory entry
	for _, fact := range facts {
		entry := &MemoryEntry{
			Content:   fact.Content,
			UserID:    scope.UserID,
			AgentID:   scope.AgentID,
			SessionID: scope.SessionID,
			Metadata: map[string]interface{}{
				"category": fact.Category,
				"source":   "smart_ingestion",
			},
		}

		// Handle resolution based on type (ADD, UPDATE, DELETE)
		if err := m.resolveAndAdd(ctx, entry, fact.Type); err != nil {
			return fmt.Errorf("failed to resolve and store extracted fact: %w", err)
		}
	}

	return nil
}

// resolveAndAdd handles the logic for adding, updating, or deleting memories
// based on the extracted intent and potential conflicts.
func (m *MemoryManager) resolveAndAdd(ctx context.Context, entry *MemoryEntry, actionType string) error {
	// 1. Generate embedding first as we need it for search
	vec, err := m.embedder.Embed(ctx, entry.Content)
	if err != nil {
		return fmt.Errorf("failed to embed content: %w", err)
	}
	entry.Embedding = vec

	// 2. Search for existing similar memories to detect conflicts
	// We search within the same scope
	filter := MemoryFilter{
		UserID:    entry.UserID,
		AgentID:   entry.AgentID,
		SessionID: entry.SessionID,
		Limit:     1, // Only check top 1 for direct conflict
	}
	existing, err := m.vectorStore.Search(ctx, vec, filter)
	if err != nil {
		return fmt.Errorf("failed to search for existing memories: %w", err)
	}

	// 3. Logic for resolution
	// Threshold for considering it a "conflict" or "duplicate"
	const similarityThreshold = 0.90

	// If explicit DELETE or UPDATE action from LLM
	if actionType == "DELETE" {
		// If we found a match, delete it
		if len(existing) > 0 {
			// For now, we assume top 1 is the target. In real Mem0, we might double check content similarity.
			// Or if the LLM output explicitly referenced the old fact.
			return m.vectorStore.Delete(ctx, existing[0].ID)
		}
		return nil // Nothing to delete
	}

	if actionType == "UPDATE" {
		if len(existing) > 0 {
			// Delete old, Add new
			if err := m.vectorStore.Delete(ctx, existing[0].ID); err != nil {
				return err
			}
		}
		// Fallthrough to Add
	}

	// Default ADD behavior: Check for near-duplicates to avoid redundancy
	if len(existing) > 0 {
		// Calculate similarity (assuming Search returns sorted by score, but we don't have score in struct yet.
		// Wait, Search returns *MemoryEntry, we don't expose score.
		// But VectorStore implementations sort by score.
		// Let's re-calculate similarity here or assume if it was returned it's close?
		// No, Search returns top K regardless of threshold.
		// We need to calc similarity.
		sim := cosineSimilarity(vec, existing[0].Embedding)
		if sim > similarityThreshold {
			// It's a duplicate, skip adding
			return nil
		}
	}

	return m.AddMemory(ctx, entry)
}

// cosineSimilarity calculates similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	// Re-implementing here or exporting it from vector_store?
	// It's better to export from a utility package or make it a method on Embedder/VectorStore?
	// For now, duplicate simple logic to avoid cyclic deps if we put it in utils that imports memory.
	var dotProduct, normA, normB float32
	if len(a) != len(b) {
		return 0
	}
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
