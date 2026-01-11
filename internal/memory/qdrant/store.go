package qdrant

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/blueberrycongee/llmux/internal/memory"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

// Store implements memory.VectorStore interface using Qdrant.
type Store struct {
	client     *http.Client
	apiBase    string
	apiKey     string
	collection string
	dimension  int
}

// Config holds configuration for Qdrant store.
type Config struct {
	Address    string
	APIKey     string
	Collection string
	Dimension  int
	Timeout    time.Duration
}

// NewStore creates a new Qdrant vector store for memory.
func NewStore(cfg Config) (*Store, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("qdrant address is required")
	}

	address := cfg.Address
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	if cfg.Collection == "" {
		cfg.Collection = "llmux_memory"
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 1536 // Default for text-embedding-3-small
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	store := &Store{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		apiBase:    address,
		apiKey:     cfg.APIKey,
		collection: cfg.Collection,
		dimension:  cfg.Dimension,
	}

	return store, nil
}

// EnsureCollection creates the collection if it doesn't exist.
func (s *Store) EnsureCollection(ctx context.Context) error {
	exists, err := s.collectionExists(ctx)
	if err != nil {
		return fmt.Errorf("check collection exists: %w", err)
	}

	if exists {
		return nil
	}

	// Create collection with cosine distance
	createBody := map[string]any{
		"vectors": map[string]any{
			"size":     s.dimension,
			"distance": "Cosine",
		},
	}

	bodyBytes, err := json.Marshal(createBody)
	if err != nil {
		return fmt.Errorf("marshal create body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", s.apiBase, s.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("create collection request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *Store) collectionExists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s/exists", s.apiBase, s.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false, err
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("check collection exists: status=%d", resp.StatusCode)
	}

	var result struct {
		Result struct {
			Exists bool `json:"exists"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}

	return result.Result.Exists, nil
}

// Add adds a new memory entry to Qdrant.
func (s *Store) Add(ctx context.Context, entry *memory.MemoryEntry) error {
	id := entry.ID
	if id == "" {
		id = uuid.New().String()
		entry.ID = id // Update entry ID if generated
	}

	// Convert float32 embedding to float64 for Qdrant JSON
	vector := make([]float64, len(entry.Embedding))
	for i, v := range entry.Embedding {
		vector[i] = float64(v)
	}

	// Prepare payload
	payload := map[string]interface{}{
		"content":    entry.Content,
		"user_id":    entry.UserID,
		"agent_id":   entry.AgentID,
		"session_id": entry.SessionID,
		"created_at": entry.CreatedAt.Unix(),
	}
	// Merge extra metadata
	for k, v := range entry.Metadata {
		payload[k] = v
	}

	point := map[string]any{
		"id":      id,
		"vector":  vector,
		"payload": payload,
	}

	upsertBody := map[string]any{
		"points": []any{point},
	}

	bodyBytes, err := json.Marshal(upsertBody)
	if err != nil {
		return fmt.Errorf("marshal upsert body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", s.apiBase, s.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("upsert request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("upsert request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Search retrieves relevant memories.
func (s *Store) Search(ctx context.Context, queryVector []float32, filter memory.MemoryFilter) ([]*memory.MemoryEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 5
	}

	// Convert float32 to float64
	vector := make([]float64, len(queryVector))
	for i, v := range queryVector {
		vector[i] = float64(v)
	}

	// Build filter
	mustConditions := []map[string]any{}
	if filter.UserID != "" {
		mustConditions = append(mustConditions, map[string]any{
			"key": "user_id",
			"match": map[string]any{
				"value": filter.UserID,
			},
		})
	}
	if filter.AgentID != "" {
		mustConditions = append(mustConditions, map[string]any{
			"key": "agent_id",
			"match": map[string]any{
				"value": filter.AgentID,
			},
		})
	}
	if filter.SessionID != "" {
		mustConditions = append(mustConditions, map[string]any{
			"key": "session_id",
			"match": map[string]any{
				"value": filter.SessionID,
			},
		})
	}

	searchBody := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  true, // We might need vector for something, but usually not returned to user
	}

	if len(mustConditions) > 0 {
		searchBody["filter"] = map[string]any{
			"must": mustConditions,
		}
	}

	bodyBytes, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("marshal search body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", s.apiBase, s.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Result []struct {
			ID      string                 `json:"id"`
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
			Vector  []float64              `json:"vector"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	entries := make([]*memory.MemoryEntry, 0, len(result.Result))
	for _, r := range result.Result {
		entry := &memory.MemoryEntry{
			ID:       r.ID,
			Metadata: make(map[string]interface{}),
		}

		// Parse payload
		if val, ok := r.Payload["content"].(string); ok {
			entry.Content = val
		}
		if val, ok := r.Payload["user_id"].(string); ok {
			entry.UserID = val
		}
		if val, ok := r.Payload["agent_id"].(string); ok {
			entry.AgentID = val
		}
		if val, ok := r.Payload["session_id"].(string); ok {
			entry.SessionID = val
		}
		if val, ok := r.Payload["created_at"].(float64); ok {
			entry.CreatedAt = time.Unix(int64(val), 0)
		}

		// Put everything else in Metadata
		for k, v := range r.Payload {
			if k != "content" && k != "user_id" && k != "agent_id" && k != "session_id" && k != "created_at" {
				entry.Metadata[k] = v
			}
		}

		// Convert vector back if needed (optional)
		if len(r.Vector) > 0 {
			entry.Embedding = make([]float32, len(r.Vector))
			for i, v := range r.Vector {
				entry.Embedding[i] = float32(v)
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// Delete removes a memory entry.
func (s *Store) Delete(ctx context.Context, id string) error {
	deleteBody := map[string]any{
		"points": []string{id},
	}

	bodyBytes, err := json.Marshal(deleteBody)
	if err != nil {
		return fmt.Errorf("marshal delete body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", s.apiBase, s.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *Store) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}
}
