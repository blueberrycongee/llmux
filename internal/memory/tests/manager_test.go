package tests

import (
	"context"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/memory"
	"github.com/blueberrycongee/llmux/internal/memory/inmem"
	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

func TestManager_SessionLifecycle(t *testing.T) {
	// 1. Session Lifecycle (Create, Update, Get)
	ctx := context.Background()
	store := inmem.NewMemorySessionStore()
	// Manager delegates to store, so testing Manager.CreateSession is sufficient
	// But we need a full manager
	mgr := memory.NewMemoryManager(store, inmem.NewMemoryVectorStore(), inmem.NewSimpleEmbedder(4), nil)

	sessionID := "sess_lifecycle"
	initialSession := &memory.Session{
		ID:        sessionID,
		UserID:    "user_1",
		Messages:  []types.ChatMessage{},
		CreatedAt: time.Now(),
	}

	// Create
	err := mgr.CreateSession(ctx, initialSession)
	assert.NoError(t, err)

	// Update
	initialSession.Messages = append(initialSession.Messages, types.ChatMessage{Role: "user", Content: json.RawMessage(`"Hi"`)})
	err = mgr.UpdateSession(ctx, initialSession)
	assert.NoError(t, err)

	// Get
	retrieved, err := mgr.GetSession(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved.Messages, 1)
}

func TestManager_VectorRetrieval(t *testing.T) {
	// 2. Vector Retrieval (Basic RAG)
	ctx := context.Background()
	mgr := memory.NewMemoryManager(inmem.NewMemorySessionStore(), inmem.NewMemoryVectorStore(), inmem.NewSimpleEmbedder(4), nil)

	// Add Scoped Memory
	fact := "User works at TechCorp"
	entry := &memory.MemoryEntry{Content: fact, UserID: "u_100"}
	err := mgr.AddMemory(ctx, entry)
	assert.NoError(t, err)

	// Retrieve with correct scope
	filter := memory.MemoryFilter{UserID: "u_100"}
	contextStr, err := mgr.RetrieveRelevantContext(ctx, fact, filter)
	assert.NoError(t, err)
	assert.Contains(t, contextStr, "User works at TechCorp")

	// Retrieve with wrong scope
	wrongFilter := memory.MemoryFilter{UserID: "u_999"}
	emptyContext, err := mgr.RetrieveRelevantContext(ctx, fact, wrongFilter)
	assert.NoError(t, err)
	assert.Empty(t, emptyContext)
}

func TestManager_HybridRetrieval_Recency(t *testing.T) {
	// 3. Hybrid Retrieval (Recency Boost)
	ctx := context.Background()
	store := inmem.NewMemoryVectorStore()
	// We test vector store directly for fine-grained ranking check

	locVec := []float32{0.5, 0.5, 0.5}
	oldMem := &memory.MemoryEntry{
		ID:        "mem_old",
		Content:   "User is in London",
		Embedding: locVec,
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
	newMem := &memory.MemoryEntry{
		ID:        "mem_new",
		Content:   "User is in New York",
		Embedding: locVec, // Same vector
		CreatedAt: time.Now(),
	}

	store.Add(ctx, oldMem)
	store.Add(ctx, newMem)

	results, err := store.Search(ctx, locVec, memory.MemoryFilter{Limit: 2})
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	// New memory should be first
	assert.Equal(t, "User is in New York", results[0].Content)
}

func TestManager_SmartIngestion(t *testing.T) {
	ctx := context.Background()
	store := inmem.NewMemorySessionStore()
	vectorStore := inmem.NewMemoryVectorStore()
	embedder := inmem.NewSimpleEmbedder(4)

	// Use RealLLMClientSimulator instead of MockLLMClient
	realLLM := inmem.NewRealLLMClientSimulator()

	mgr := memory.NewMemoryManager(store, vectorStore, embedder, realLLM)

	rawText := "I moved to Berlin and I like currywurst."

	// Execute Ingest
	scope := memory.MemoryFilter{UserID: "u_berlin"}
	err := mgr.IngestMemory(ctx, rawText, scope)
	assert.NoError(t, err)

	// Verify persistence
	// Check Fact 1
	contextStr, err := mgr.RetrieveRelevantContext(ctx, "User lives in Berlin", scope)
	assert.NoError(t, err)
	assert.Contains(t, contextStr, "User lives in Berlin")

	// Check Fact 2
	contextStr, err = mgr.RetrieveRelevantContext(ctx, "User likes currywurst", scope)
	assert.NoError(t, err)
	assert.Contains(t, contextStr, "User likes currywurst")
}

func TestManager_MemoryResolution(t *testing.T) {
	ctx := context.Background()
	store := inmem.NewMemorySessionStore()
	vectorStore := inmem.NewMemoryVectorStore()
	embedder := inmem.NewSimpleEmbedder(4)
	realLLM := inmem.NewRealLLMClientSimulator()

	mgr := memory.NewMemoryManager(store, vectorStore, embedder, realLLM)
	scope := memory.MemoryFilter{UserID: "u_conflicted"}

	// 1. Initial State: User loves Java
	// We inject this manually to simulate existing memory
	initialFact := "User loves Java"
	mgr.AddMemory(ctx, &memory.MemoryEntry{
		Content: initialFact,
		UserID:  scope.UserID,
	})

	// 2. User Update: "Forget I love Java"
	// The RealLLMClientSimulator is programmed to return DELETE for this specific phrase.

	// Execute Ingest (Delete)
	err := mgr.IngestMemory(ctx, "Forget I love Java", scope)
	assert.NoError(t, err)

	// Verify Deletion
	// Search for the old fact should return nothing
	contextStr, err := mgr.RetrieveRelevantContext(ctx, "User loves Java", scope)
	assert.NoError(t, err)
	assert.Empty(t, contextStr, "Memory should have been deleted")
}
