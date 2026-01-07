package vector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

// QdrantStore implements Store interface using Qdrant vector database.
// Reference: https://qdrant.tech/documentation/concepts/search/
type QdrantStore struct {
	client     *http.Client
	apiBase    string
	apiKey     string
	collection string
	dimension  int
}

// QdrantConfig holds configuration for Qdrant store.
type QdrantConfig struct {
	APIBase    string
	APIKey     string
	Collection string
	Dimension  int
	Timeout    time.Duration
}

// NewQdrantStore creates a new Qdrant vector store.
func NewQdrantStore(cfg QdrantConfig) (*QdrantStore, error) {
	if cfg.APIBase == "" {
		return nil, fmt.Errorf("qdrant api_base is required")
	}
	if cfg.Collection == "" {
		return nil, fmt.Errorf("qdrant collection is required")
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 1536 // Default for text-embedding-ada-002
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	store := &QdrantStore{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		apiBase:    cfg.APIBase,
		apiKey:     cfg.APIKey,
		collection: cfg.Collection,
		dimension:  cfg.Dimension,
	}

	return store, nil
}

// EnsureCollection creates the collection if it doesn't exist.
func (q *QdrantStore) EnsureCollection(ctx context.Context) error {
	// Check if collection exists
	exists, err := q.collectionExists(ctx)
	if err != nil {
		return fmt.Errorf("check collection exists: %w", err)
	}

	if exists {
		return nil
	}

	// Create collection with cosine distance
	createBody := map[string]any{
		"vectors": map[string]any{
			"size":     q.dimension,
			"distance": "Cosine",
		},
		"quantization_config": map[string]any{
			"binary": map[string]any{
				"always_ram": false,
			},
		},
	}

	bodyBytes, err := json.Marshal(createBody)
	if err != nil {
		return fmt.Errorf("marshal create body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", q.apiBase, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	q.setHeaders(req)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("create collection request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func (q *QdrantStore) collectionExists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s/exists", q.apiBase, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false, err
	}

	q.setHeaders(req)

	resp, err := q.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

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

// Search finds similar vectors in Qdrant.
func (q *QdrantStore) Search(ctx context.Context, vector []float64, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 1
	}

	searchBody := map[string]any{
		"vector":       vector,
		"limit":        opts.TopK,
		"with_payload": true,
		"params": map[string]any{
			"quantization": map[string]any{
				"ignore":       false,
				"rescore":      true,
				"oversampling": 3.0,
			},
		},
	}

	bodyBytes, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("marshal search body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", q.apiBase, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q.setHeaders(req)

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var searchResp qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Convert Qdrant results to SearchResult
	results := make([]SearchResult, 0, len(searchResp.Result))
	for _, r := range searchResp.Result {
		// Qdrant returns score for cosine similarity (1 = identical, 0 = orthogonal)
		// Convert to distance: distance = 1 - score
		distance := 1 - r.Score

		// Filter by distance threshold
		if opts.DistanceThreshold > 0 && distance > opts.DistanceThreshold {
			continue
		}

		results = append(results, SearchResult{
			ID:       r.ID,
			Score:    r.Score,
			Distance: distance,
			Payload: Payload{
				Prompt:    r.Payload.Prompt,
				Response:  r.Payload.Response,
				Model:     r.Payload.Model,
				CreatedAt: r.Payload.CreatedAt,
			},
		})
	}

	return results, nil
}

// Insert stores a vector in Qdrant.
func (q *QdrantStore) Insert(ctx context.Context, entry Entry) error {
	return q.InsertBatch(ctx, []Entry{entry})
}

// InsertBatch stores multiple vectors in Qdrant.
func (q *QdrantStore) InsertBatch(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	points := make([]qdrantPoint, 0, len(entries))
	for _, e := range entries {
		id := e.ID
		if id == "" {
			id = uuid.New().String()
		}

		points = append(points, qdrantPoint{
			ID:     id,
			Vector: e.Vector,
			Payload: qdrantPayload{
				Prompt:    e.Payload.Prompt,
				Response:  e.Payload.Response,
				Model:     e.Payload.Model,
				CreatedAt: e.Payload.CreatedAt,
			},
		})
	}

	upsertBody := map[string]any{
		"points": points,
	}

	bodyBytes, err := json.Marshal(upsertBody)
	if err != nil {
		return fmt.Errorf("marshal upsert body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", q.apiBase, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	q.setHeaders(req)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("upsert request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Delete removes a vector from Qdrant.
func (q *QdrantStore) Delete(ctx context.Context, id string) error {
	deleteBody := map[string]any{
		"points": []string{id},
	}

	bodyBytes, err := json.Marshal(deleteBody)
	if err != nil {
		return fmt.Errorf("marshal delete body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", q.apiBase, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	q.setHeaders(req)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Ping checks if Qdrant is healthy.
func (q *QdrantStore) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections", q.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}

	q.setHeaders(req)

	resp, err := q.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant ping failed: status=%d", resp.StatusCode)
	}

	return nil
}

// Close releases resources.
func (q *QdrantStore) Close() error {
	q.client.CloseIdleConnections()
	return nil
}

func (q *QdrantStore) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if q.apiKey != "" {
		req.Header.Set("api-key", q.apiKey)
	}
}

// Qdrant API types

type qdrantPoint struct {
	ID      string        `json:"id"`
	Vector  []float64     `json:"vector"`
	Payload qdrantPayload `json:"payload"`
}

type qdrantPayload struct {
	Prompt    string `json:"prompt"`
	Response  string `json:"response"`
	Model     string `json:"model,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

type qdrantSearchResponse struct {
	Result []qdrantSearchResult `json:"result"`
}

type qdrantSearchResult struct {
	ID      string        `json:"id"`
	Score   float64       `json:"score"`
	Payload qdrantPayload `json:"payload"`
}
