package memory

import (
	"time"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// Session represents a conversation session with an agent.
type Session struct {
	ID        string              `json:"id"`
	UserID    string              `json:"user_id"`
	AgentID   string              `json:"agent_id"`
	Messages  []types.ChatMessage `json:"messages"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// MemoryEntry represents a single unit of information in long-term memory.
type MemoryEntry struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Embedding []float32 `json:"embedding,omitempty"`

	// Scoping fields for hierarchical memory management (inspired by Mem0)
	UserID    string `json:"user_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// MemoryFilter defines criteria for retrieving memories.
type MemoryFilter struct {
	UserID    string
	AgentID   string
	SessionID string
	Limit     int
}

// Fact represents a structured piece of information extracted from text.
type Fact struct {
	Content  string `json:"content"`
	Category string `json:"category,omitempty"` // e.g., "preference", "fact", "summary"
	Type     string `json:"type,omitempty"`     // e.g., "ADD", "UPDATE", "DELETE" (for resolution)
}

// ExtractionResult is the output from the Extractor.
type ExtractionResult struct {
	Facts []Fact `json:"facts"`
}
