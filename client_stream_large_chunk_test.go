package llmux

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/types"
)

type largeChunkProvider struct {
	*httpMockProvider
}

func (p *largeChunkProvider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	// Accept any non-DONE chunk; this test focuses on the scanner token limit.
	return &types.StreamChunk{}, nil
}

func TestClient_ChatCompletionStream_AllowsChunksOver16KB(t *testing.T) {
	large := make([]byte, 32*1024)
	for i := range large {
		large[i] = 'a'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: "))
		w.Write(large)
		w.Write([]byte("\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	baseMock := &httpMockProvider{
		name:    "mock-large-chunk",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}
	mock := &largeChunkProvider{httpMockProvider: baseMock}

	client, err := New(
		WithProviderInstance("mock-large-chunk", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream() unexpected error = %v", err)
	}
	defer stream.Close()

	_, err = stream.Recv()
	if err != nil {
		t.Fatalf("Recv() unexpected error = %v", err)
	}

	_, err = stream.Recv()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestClient_ChatCompletionStream_IgnoresGlobalHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("data: {}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	baseMock := &httpMockProvider{
		name:    "mock-stream-timeout",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}
	mock := &largeChunkProvider{httpMockProvider: baseMock}

	client, err := New(
		WithProviderInstance("mock-stream-timeout", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream() unexpected error = %v", err)
	}
	defer stream.Close()

	_, err = stream.Recv()
	if err != nil {
		t.Fatalf("Recv() unexpected error = %v", err)
	}
}
