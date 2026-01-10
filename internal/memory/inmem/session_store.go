package inmem

import (
	"context"
	"fmt"
	"sync"

	"github.com/blueberrycongee/llmux/internal/memory"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// MemorySessionStore is a thread-safe in-memory implementation of SessionStore.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*memory.Session
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*memory.Session),
	}
}

func (s *MemorySessionStore) CreateSession(ctx context.Context, session *memory.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; exists {
		return fmt.Errorf("session already exists: %s", session.ID)
	}
	// Deep copy to simulate DB isolation
	s.sessions[session.ID] = deepCopySession(session)
	return nil
}

func (s *MemorySessionStore) GetSession(ctx context.Context, id string) (*memory.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return deepCopySession(sess), nil
}

func (s *MemorySessionStore) UpdateSession(ctx context.Context, session *memory.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; !exists {
		return fmt.Errorf("session not found: %s", session.ID)
	}
	s.sessions[session.ID] = deepCopySession(session)
	return nil
}

func (s *MemorySessionStore) DeleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *MemorySessionStore) ListUserSessions(ctx context.Context, userID string) ([]*memory.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*memory.Session
	for _, sess := range s.sessions {
		if sess.UserID == userID {
			result = append(result, deepCopySession(sess))
		}
	}
	return result, nil
}

func deepCopySession(src *memory.Session) *memory.Session {
	dst := *src
	// Copy messages slice
	if src.Messages != nil {
		dst.Messages = make([]types.ChatMessage, len(src.Messages))
		copy(dst.Messages, src.Messages)
	}
	// Copy metadata map
	if src.Metadata != nil {
		dst.Metadata = make(map[string]string)
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}
	return &dst
}
