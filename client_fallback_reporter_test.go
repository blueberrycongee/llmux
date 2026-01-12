package llmux

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/stretchr/testify/require"
)

type retryableHTTPProvider struct {
	name    string
	models  []string
	baseURL string
}

func (p *retryableHTTPProvider) Name() string                     { return p.name }
func (p *retryableHTTPProvider) SupportedModels() []string        { return p.models }
func (p *retryableHTTPProvider) SupportsModel(model string) bool   { return model == p.models[0] }
func (p *retryableHTTPProvider) SupportEmbedding() bool            { return false }
func (p *retryableHTTPProvider) ParseStreamChunk([]byte) (*StreamChunk, error) {
	return nil, nil
}

func (p *retryableHTTPProvider) BuildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (p *retryableHTTPProvider) ParseResponse(resp *http.Response) (*ChatResponse, error) {
	body, _ := io.ReadAll(resp.Body)
	var chatResp ChatResponse
	_ = json.Unmarshal(body, &chatResp)
	return &chatResp, nil
}

func (p *retryableHTTPProvider) MapError(statusCode int, body []byte) error {
	return NewServiceUnavailableError(p.name, "", "upstream error")
}

func (p *retryableHTTPProvider) BuildEmbeddingRequest(context.Context, *types.EmbeddingRequest) (*http.Request, error) {
	return nil, NewInvalidRequestError(p.name, "", "not supported")
}

func (p *retryableHTTPProvider) ParseEmbeddingResponse(*http.Response) (*types.EmbeddingResponse, error) {
	return nil, NewInvalidRequestError(p.name, "", "not supported")
}

func TestClient_FallbackReporter_Success(t *testing.T) {
	var primaryHits atomic.Int64
	var secondaryHits atomic.Int64

	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom","type":"server_error"}}`))
	}))
	defer primaryServer.Close()

	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryHits.Add(1)
		resp := ChatResponse{
			ID:      "secondary",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: json.RawMessage(`"ok"`),
					},
					FinishReason: "stop",
				},
			},
			Usage: &Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer secondaryServer.Close()

	var mu sync.Mutex
	events := make([]fallbackEvent, 0, 1)
	reporter := func(ctx context.Context, originalModel, fallbackModel string, err error, success bool) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, fallbackEvent{
			original: originalModel,
			fallback: fallbackModel,
			success:  success,
			err:      err,
		})
	}

	client, err := New(
		WithProviderInstance("primary", &retryableHTTPProvider{
			name:    "primary",
			models:  []string{"test-model"},
			baseURL: primaryServer.URL,
		}, []string{"test-model"}),
		WithProviderInstance("secondary", &retryableHTTPProvider{
			name:    "secondary",
			models:  []string{"test-model"},
			baseURL: secondaryServer.URL,
		}, []string{"test-model"}),
		WithRouterStrategy(StrategyRoundRobin),
		WithFallback(true),
		WithRetry(1, 5*time.Millisecond),
		WithFallbackReporter(reporter),
		withTestPricing(t, "test-model"),
	)
	require.NoError(t, err)
	defer client.Close()

	resp, err := client.ChatCompletion(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(1), primaryHits.Load())
	require.Equal(t, int64(1), secondaryHits.Load())

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	require.True(t, events[0].success)
	require.NotNil(t, events[0].err)
}

type fallbackEvent struct {
	original string
	fallback string
	success  bool
	err      error
}
