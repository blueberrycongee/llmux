package llmux

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

func TestClient_ChatCompletion_MissingPricing(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		resp := ChatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "missing-price-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: json.RawMessage(`"Hello!"`),
					},
					FinishReason: "stop",
				},
			},
			Usage: &Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mock := &httpMockProvider{
		name:    "mock",
		models:  []string{"missing-price-model"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock", mock, []string{"missing-price-model"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Model: "missing-price-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing pricing, got nil")
	}
	if llmErr, ok := err.(*llmerrors.LLMError); !ok || llmErr.Type != llmerrors.TypeInternalError {
		t.Fatalf("expected internal error for missing pricing, got %T (%v)", err, err)
	}
	if atomic.LoadInt32(&requestCount) != 0 {
		t.Errorf("expected no upstream calls, got %d", requestCount)
	}
}

func TestClient_ChatCompletionStream_MissingPricing(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	mock := &httpMockProvider{
		name:    "mock-stream",
		models:  []string{"missing-price-stream"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock-stream", mock, []string{"missing-price-stream"}),
		WithRetry(1, 10*time.Millisecond),
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model: "missing-price-stream",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing pricing, got nil")
	}
	if llmErr, ok := err.(*llmerrors.LLMError); !ok || llmErr.Type != llmerrors.TypeInternalError {
		t.Fatalf("expected internal error for missing pricing, got %T (%v)", err, err)
	}
	if atomic.LoadInt32(&requestCount) != 0 {
		t.Errorf("expected no upstream calls, got %d", requestCount)
	}
}
