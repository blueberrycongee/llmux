package llmux

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// streamMockProvider extends httpMockProvider to support stream parsing
type streamMockProvider struct {
	*httpMockProvider
}

func (m *streamMockProvider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	// Simple mock parser
	return &types.StreamChunk{}, nil
}

type streamTestPlugin struct {
	preCalled  atomic.Bool
	postCalled atomic.Bool
	chunkCount atomic.Int32
	postErrNil atomic.Bool
}

func (p *streamTestPlugin) Name() string  { return "stream-test" }
func (p *streamTestPlugin) Priority() int { return 0 }
func (p *streamTestPlugin) Cleanup() error {
	return nil
}

func (p *streamTestPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
	return req, nil, nil
}

func (p *streamTestPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	return resp, err, nil
}

func (p *streamTestPlugin) PreStreamHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.StreamShortCircuit, error) {
	p.preCalled.Store(true)
	return req, nil, nil
}

func (p *streamTestPlugin) OnStreamChunk(ctx *plugin.Context, chunk *types.StreamChunk) (*types.StreamChunk, error) {
	p.chunkCount.Add(1)
	return chunk, nil
}

func (p *streamTestPlugin) PostStreamHook(ctx *plugin.Context, err error) error {
	p.postCalled.Store(true)
	p.postErrNil.Store(err == nil)
	return nil
}

func TestClient_ChatCompletionStream_RetrySuccess(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	baseMock := &httpMockProvider{
		name:    "mock-stream",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}
	mock := &streamMockProvider{httpMockProvider: baseMock}

	client, err := New(
		WithProviderInstance("mock-stream", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithRetry(3, 10*time.Millisecond),
		WithCooldown(0),
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

	// We expect 3 requests: 2 failures + 1 success
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("expected 3 requests, got %d", requestCount)
	}
}

func TestClient_ChatCompletionStream_RetryFailure(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	baseMock := &httpMockProvider{
		name:    "mock-stream-fail",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}
	mock := &streamMockProvider{httpMockProvider: baseMock}

	client, err := New(
		WithProviderInstance("mock-stream-fail", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithRetry(2, 10*time.Millisecond),
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// 0(initial) + 1(retry1) + 2(retry2) = 3 attempts
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("expected 3 requests, got %d", requestCount)
	}
}

func TestClient_ChatCompletionStream_PluginHooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {}\n\n"))
		w.Write([]byte("data: {}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	baseMock := &httpMockProvider{
		name:    "mock-stream-plugin",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}
	mock := &streamMockProvider{httpMockProvider: baseMock}
	streamPlugin := &streamTestPlugin{}

	client, err := New(
		WithProviderInstance("mock-stream-plugin", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithRetry(1, 10*time.Millisecond),
		WithCooldown(0),
		WithPlugin(streamPlugin),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)}},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream() unexpected error = %v", err)
	}
	defer stream.Close()

	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() unexpected error = %v", err)
		}
	}

	if !streamPlugin.preCalled.Load() {
		t.Error("expected PreStreamHook to be called")
	}
	if streamPlugin.chunkCount.Load() == 0 {
		t.Error("expected OnStreamChunk to be called")
	}
	if !streamPlugin.postCalled.Load() {
		t.Error("expected PostStreamHook to be called")
	}
	if !streamPlugin.postErrNil.Load() {
		t.Error("expected PostStreamHook err to be nil")
	}
}
