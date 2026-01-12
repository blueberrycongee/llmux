package llmux

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/tokenizer"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

type recordingRouter struct {
	mu         sync.Mutex
	reqCtx     *router.RequestContext
	pickCalled bool
	deployment *provider.Deployment
}

func (r *recordingRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	r.mu.Lock()
	r.pickCalled = true
	r.mu.Unlock()
	return r.deployment, nil
}

func (r *recordingRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	r.mu.Lock()
	r.reqCtx = reqCtx
	r.mu.Unlock()
	return r.deployment, nil
}

func (r *recordingRouter) ReportSuccess(_ *provider.Deployment, _ *router.ResponseMetrics) {}
func (r *recordingRouter) ReportFailure(_ *provider.Deployment, _ error)                   {}
func (r *recordingRouter) ReportRequestStart(_ *provider.Deployment)                       {}
func (r *recordingRouter) ReportRequestEnd(_ *provider.Deployment)                         {}
func (r *recordingRouter) IsCircuitOpen(_ *provider.Deployment) bool                       { return false }
func (r *recordingRouter) SetCooldown(_ string, _ time.Time) error                         { return nil }
func (r *recordingRouter) AddDeployment(_ *provider.Deployment)                            {}
func (r *recordingRouter) AddDeploymentWithConfig(_ *provider.Deployment, _ router.DeploymentConfig) {
}
func (r *recordingRouter) RemoveDeployment(_ string)                      {}
func (r *recordingRouter) GetDeployments(_ string) []*provider.Deployment { return nil }
func (r *recordingRouter) GetStats(_ string) *router.DeploymentStats      { return nil }
func (r *recordingRouter) GetStrategy() router.Strategy                   { return router.StrategySimpleShuffle }

func (r *recordingRouter) snapshot() (*router.RequestContext, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reqCtx, r.pickCalled
}

func TestClient_ChatCompletion_UsesRequestContextAndStripsTags(t *testing.T) {
	var sawTags atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer func() { _ = r.Body.Close() }()

		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if _, ok := payload["tags"]; ok {
			sawTags.Store(true)
		}

		resp := ChatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Model:   "test-model",
			Choices: []Choice{{Index: 0, Message: ChatMessage{Role: "assistant", Content: json.RawMessage(`"ok"`)}}},
			Usage:   &Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mockProvider := &httpMockProvider{
		name:    "mock",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}

	deployment := &provider.Deployment{ID: "dep-1", ProviderName: "mock", ModelName: "test-model"}
	recRouter := &recordingRouter{deployment: deployment}

	client, err := New(
		WithProviderInstance("mock", mockProvider, []string{"test-model"}),
		WithRouter(recRouter),
		withTestPricing(t, "test-model"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
		Tags:     []string{"premium"},
	}

	_, err = client.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if sawTags.Load() {
		t.Fatalf("expected tags to be stripped before provider request")
	}

	reqCtx, pickCalled := recRouter.snapshot()
	if pickCalled {
		t.Fatalf("expected PickWithContext to be used instead of Pick")
	}
	if reqCtx == nil {
		t.Fatalf("expected request context to be recorded")
	}
	if reqCtx.IsStreaming {
		t.Fatalf("expected IsStreaming=false for non-stream request")
	}
	if !reflect.DeepEqual(reqCtx.Tags, req.Tags) {
		t.Fatalf("expected tags %v, got %v", req.Tags, reqCtx.Tags)
	}

	expectedTokens := tokenizer.EstimatePromptTokens(req.Model, req)
	if reqCtx.EstimatedInputTokens != expectedTokens {
		t.Fatalf("expected EstimatedInputTokens=%d, got %d", expectedTokens, reqCtx.EstimatedInputTokens)
	}
}

func TestClient_ChatCompletionStream_UsesStreamingContextAndStripsTags(t *testing.T) {
	var sawTags atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer func() { _ = r.Body.Close() }()

		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if _, ok := payload["tags"]; ok {
			sawTags.Store(true)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	mockProvider := &httpMockProvider{
		name:    "mock",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}

	deployment := &provider.Deployment{ID: "dep-1", ProviderName: "mock", ModelName: "test-model"}
	recRouter := &recordingRouter{deployment: deployment}

	client, err := New(
		WithProviderInstance("mock", mockProvider, []string{"test-model"}),
		WithRouter(recRouter),
		withTestPricing(t, "test-model"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	req := &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
		Tags:     []string{"streaming"},
	}

	stream, err := client.ChatCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	_ = stream.Close()

	if sawTags.Load() {
		t.Fatalf("expected tags to be stripped before provider request")
	}

	reqCtx, pickCalled := recRouter.snapshot()
	if pickCalled {
		t.Fatalf("expected PickWithContext to be used instead of Pick")
	}
	if reqCtx == nil {
		t.Fatalf("expected request context to be recorded")
	}
	if !reqCtx.IsStreaming {
		t.Fatalf("expected IsStreaming=true for streaming request")
	}
	if !reflect.DeepEqual(reqCtx.Tags, req.Tags) {
		t.Fatalf("expected tags %v, got %v", req.Tags, reqCtx.Tags)
	}

	expectedTokens := tokenizer.EstimatePromptTokens(req.Model, req)
	if reqCtx.EstimatedInputTokens != expectedTokens {
		t.Fatalf("expected EstimatedInputTokens=%d, got %d", expectedTokens, reqCtx.EstimatedInputTokens)
	}
}
