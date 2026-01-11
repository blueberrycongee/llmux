package llmux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/blueberrycongee/llmux/internal/tokenizer"
	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// mockEmbeddingProvider implements Provider interface for embedding testing.
type mockEmbeddingProvider struct {
	name              string
	models            []string
	baseURL           string
	supportsEmbedding bool
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
		withTestPricing(t, "text-embedding-3-small"),
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

func TestClient_Embedding_MissingPricing(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		resp := types.EmbeddingResponse{
			Object: "list",
			Data: []types.EmbeddingObject{
				{
					Object:    "embedding",
					Embedding: []float64{0.1},
					Index:     0,
				},
			},
			Model: "embedding-missing-price",
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
		models:            []string{"embedding-missing-price"},
		baseURL:           server.URL,
		supportsEmbedding: true,
	}

	client, err := New(
		WithProviderInstance("mock-embedding", mock, []string{"embedding-missing-price"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &types.EmbeddingRequest{
		Model: "embedding-missing-price",
		Input: types.NewEmbeddingInputFromString("Hello"),
	}

	_, err = client.Embedding(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing pricing, got nil")
	}
	if llmErr, ok := err.(*errors.LLMError); !ok || llmErr.Type != errors.TypeInternalError {
		t.Fatalf("expected internal error for missing pricing, got %T (%v)", err, err)
	}
	if atomic.LoadInt32(&requestCount) != 0 {
		t.Errorf("expected no upstream calls, got %d", requestCount)
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
		withTestPricing(t, "text-embedding-3-small"),
		WithRetry(2, 10*time.Millisecond),
		WithCooldown(1*time.Millisecond),
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

func TestClient_Embedding_EstimatesUsageWhenMissing(t *testing.T) {
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
			Usage: types.Usage{},
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
		withTestPricing(t, "text-embedding-3-small"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello embedding tokens"),
	}

	resp, err := client.Embedding(context.Background(), req)
	if err != nil {
		t.Fatalf("Embedding() error = %v", err)
	}

	expected := tokenizer.EstimateEmbeddingTokens(req.Model, req)
	if resp.Usage.PromptTokens != expected {
		t.Errorf("PromptTokens = %d, want %d", resp.Usage.PromptTokens, expected)
	}
	if resp.Usage.TotalTokens != expected {
		t.Errorf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, expected)
	}
	if resp.Usage.CompletionTokens != 0 {
		t.Errorf("CompletionTokens = %d, want 0", resp.Usage.CompletionTokens)
	}
}

func TestClient_Embedding_PreservesProviderUsage(t *testing.T) {
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
				PromptTokens: 12,
				TotalTokens:  12,
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
		withTestPricing(t, "text-embedding-3-small"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello embedding tokens"),
	}

	resp, err := client.Embedding(context.Background(), req)
	if err != nil {
		t.Fatalf("Embedding() error = %v", err)
	}

	if resp.Usage.PromptTokens != 12 {
		t.Errorf("PromptTokens = %d, want 12", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != 12 {
		t.Errorf("TotalTokens = %d, want 12", resp.Usage.TotalTokens)
	}
}

func TestClient_Embedding_UsesEstimatedTokensForRateLimit(t *testing.T) {
	var capturedIncrement int64
	captureLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			results := make([]resilience.LimitResult, len(descriptors))
			for i, desc := range descriptors {
				if desc.Type == resilience.LimitTypeTokens {
					capturedIncrement = desc.Increment
				}
				results[i] = resilience.LimitResult{
					Allowed:   true,
					Current:   1,
					Remaining: desc.Limit - 1,
					ResetAt:   time.Now().Add(time.Minute).Unix(),
				}
			}
			return results, nil
		},
	}

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
				PromptTokens: 0,
				TotalTokens:  0,
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
		withTestPricing(t, "text-embedding-3-small"),
		WithRateLimiter(captureLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			TPMLimit:    100000,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	input := strings.Repeat("hello ", 64)
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString(input),
	}

	_, err = client.Embedding(context.Background(), req)
	if err != nil {
		t.Fatalf("Embedding() error = %v", err)
	}

	expected := int64(tokenizer.CountTextTokens(req.Model, input))
	if expected == 0 {
		t.Fatal("expected non-zero token estimate for input")
	}
	if capturedIncrement != expected {
		t.Errorf("rate limit increment = %d, want %d", capturedIncrement, expected)
	}
}
