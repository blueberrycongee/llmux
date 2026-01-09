package llmux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// mockEmbeddingProvider implements Provider interface for embedding testing.
type mockEmbeddingProvider struct {
	name              string
	models            []string
	baseURL           string
	supportsEmbedding bool
	failCount         int
	failCode          int
}

func (m *mockEmbeddingProvider) Name() string {
	return m.name
}

func (m *mockEmbeddingProvider) SupportedModels() []string {
	return m.models
}

func (m *mockEmbeddingProvider) SupportsModel(model string) bool {
	for _, mod := range m.models {
		if mod == model {
			return true
		}
	}
	return false
}

func (m *mockEmbeddingProvider) BuildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockEmbeddingProvider) ParseResponse(resp *http.Response) (*ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockEmbeddingProvider) ParseStreamChunk(data []byte) (*StreamChunk, error) {
	return nil, nil
}

func (m *mockEmbeddingProvider) MapError(statusCode int, body []byte) error {
	if statusCode == http.StatusServiceUnavailable {
		return errors.NewServiceUnavailableError(m.name, "", "service unavailable")
	}
	return errors.NewInternalError(m.name, "", "error")
}

func (m *mockEmbeddingProvider) SupportEmbedding() bool {
	return m.supportsEmbedding
}

func (m *mockEmbeddingProvider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (m *mockEmbeddingProvider) ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error) {
	body, _ := io.ReadAll(resp.Body)
	var embResp types.EmbeddingResponse
	json.Unmarshal(body, &embResp)
	return &embResp, nil
}

func TestClient_Embedding_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.EmbeddingResponse{
			Object: "list",
			Data: []types.EmbeddingObject{
				{
					Object:    "embedding",
					Embedding: []float64{0.1, 0.2, 0.3},
					Index:     0,
				},
			},
			Model: "text-embedding-3-small",
			Usage: types.Usage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mock := &mockEmbeddingProvider{
		name:              "mock-embedding",
		models:            []string{"text-embedding-3-small"},
		baseURL:           server.URL,
		supportsEmbedding: true,
	}

	client, err := New(
		WithProviderInstance("mock-embedding", mock, []string{"text-embedding-3-small"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello"),
	}

	resp, err := client.Embedding(context.Background(), req)
	if err != nil {
		t.Fatalf("Embedding() error = %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 embedding, got %d", len(resp.Data))
	}
	if resp.Data[0].Embedding[0] != 0.1 {
		t.Errorf("expected embedding value 0.1, got %f", resp.Data[0].Embedding[0])
	}
}

func TestClient_Embedding_NotSupported(t *testing.T) {
	mock := &mockEmbeddingProvider{
		name:              "mock-no-embedding",
		models:            []string{"text-embedding-3-small"},
		supportsEmbedding: false,
	}

	client, err := New(
		WithProviderInstance("mock-no-embedding", mock, []string{"text-embedding-3-small"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello"),
	}

	_, err = client.Embedding(context.Background(), req)
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestClient_Embedding_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := types.EmbeddingResponse{
			Data: []types.EmbeddingObject{{Embedding: []float64{0.1}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mock := &mockEmbeddingProvider{
		name:              "mock-retry",
		models:            []string{"text-embedding-3-small"},
		baseURL:           server.URL,
		supportsEmbedding: true,
	}

	client, err := New(
		WithProviderInstance("mock-retry", mock, []string{"text-embedding-3-small"}),
		WithRetry(2, 10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello"),
	}

	resp, err := client.Embedding(context.Background(), req)
	if err != nil {
		t.Fatalf("Embedding() error = %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 embedding, got %d", len(resp.Data))
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}
