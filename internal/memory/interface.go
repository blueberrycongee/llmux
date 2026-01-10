package memory

import (
	"context"
)

// SessionStore defines the interface for session storage.
type SessionStore interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, id string) error
	ListUserSessions(ctx context.Context, userID string) ([]*Session, error)
}

// VectorStore defines the interface for long-term memory retrieval.
type VectorStore interface {
	Add(ctx context.Context, entry *MemoryEntry) error
	// Search retrieves relevant memories based on vector similarity and optional filters.
	Search(ctx context.Context, queryVector []float32, filter MemoryFilter) ([]*MemoryEntry, error)
	Delete(ctx context.Context, id string) error
}

// Manager combines different memory capabilities.
type Manager interface {
	SessionStore

	// RetrieveRelevantContext performs RAG (Retrieval-Augmented Generation).
	// It retrieves relevant memories based on the query and formats them as a context string.
	// Supports optional scoping filters.
	RetrieveRelevantContext(ctx context.Context, query string, filter MemoryFilter) (string, error)

	// AddMemory adds a new memory entry to the long-term storage.
	// Supports scoping (UserID, AgentID, SessionID) via options or struct.
	AddMemory(ctx context.Context, entry *MemoryEntry) error

	// IngestMemory processes raw text using the Extractor to identify facts,
	// and then stores them as memory entries.
	// This is the "Smart Ingestion" entry point.
	IngestMemory(ctx context.Context, content string, scope MemoryFilter) error
}
