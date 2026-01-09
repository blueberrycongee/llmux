package llmux

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// sequenceRouter implements router.Router for testing.
// It returns deployments in a pre-defined sequence.
type sequenceRouter struct {
	mu           sync.Mutex
	deployments  []*provider.Deployment
	currentIndex int
}

func (r *sequenceRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentIndex >= len(r.deployments) {
		return nil, fmt.Errorf("no more deployments")
	}
	d := r.deployments[r.currentIndex]
	r.currentIndex++
	return d, nil
}

// Stub implementations for other interface methods
func (r *sequenceRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	return r.Pick(ctx, reqCtx.Model)
}
func (r *sequenceRouter) ReportSuccess(d *provider.Deployment, m *router.ResponseMetrics)           {}
func (r *sequenceRouter) ReportFailure(d *provider.Deployment, err error)                           {}
func (r *sequenceRouter) ReportRequestStart(d *provider.Deployment)                                 {}
func (r *sequenceRouter) ReportRequestEnd(d *provider.Deployment)                                   {}
func (r *sequenceRouter) IsCircuitOpen(d *provider.Deployment) bool                                 { return false }
func (r *sequenceRouter) AddDeployment(d *provider.Deployment)                                      {}
func (r *sequenceRouter) AddDeploymentWithConfig(d *provider.Deployment, c router.DeploymentConfig) {}
func (r *sequenceRouter) RemoveDeployment(id string)                                                {}
func (r *sequenceRouter) GetDeployments(model string) []*provider.Deployment                        { return nil }
func (r *sequenceRouter) GetStats(id string) *router.DeploymentStats                                { return nil }
func (r *sequenceRouter) GetStrategy() router.Strategy                                              { return router.StrategySimpleShuffle }

// Test Scenario 1: Cross-Provider Fallback
// Simulates: Provider A (Fail) -> Provider B (Success)
func TestClient_ChatCompletionStream_CrossProviderFallback(t *testing.T) {
	// Setup Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/providerA/") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "provider A failed"}`))
			return
		}
		if strings.Contains(r.URL.Path, "/providerB/") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: {}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Setup Providers
	provA := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "providerA", models: []string{"test-model"}, baseURL: server.URL + "/providerA",
	}}
	provB := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "providerB", models: []string{"test-model"}, baseURL: server.URL + "/providerB",
	}}

	// Setup Router
	depA := &provider.Deployment{ID: "depA", ProviderName: "providerA", ModelName: "test-model"}
	depB := &provider.Deployment{ID: "depB", ProviderName: "providerB", ModelName: "test-model"}
	mockRouter := &sequenceRouter{
		deployments: []*provider.Deployment{depA, depB},
	}

	// Setup Client
	client, err := New(
		WithProviderInstance("providerA", provA, []string{"test-model"}),
		WithProviderInstance("providerB", provB, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(3, 10*time.Millisecond),
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Execute
	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	defer stream.Close()

	// Verify we got a stream (from Provider B)
	_, err = stream.Recv()
	if err != nil && err.Error() != "EOF" { // Depending on mock implementation
		t.Errorf("Unexpected error during stream Recv: %v", err)
	}
}

// Test Scenario 2: Mixed Error Types
// Simulates: 500 -> 503 -> Success
func TestClient_ChatCompletionStream_MixedErrors(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&reqCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError) // 500
			return
		}
		if count == 2 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503
			return
		}
		// Success
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	prov := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "mixed-error-prov", models: []string{"test-model"}, baseURL: server.URL,
	}}

	// Router always returns same deployment
	dep := &provider.Deployment{ID: "dep1", ProviderName: "mixed-error-prov", ModelName: "test-model"}
	mockRouter := &sequenceRouter{
		deployments: []*provider.Deployment{dep, dep, dep, dep}, // Enough for retries
	}

	client, err := New(
		WithProviderInstance("mixed-error-prov", prov, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(3, 10*time.Millisecond),
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	defer stream.Close()

	if atomic.LoadInt32(&reqCount) != 3 {
		t.Errorf("Expected 3 requests, got %d", reqCount)
	}
}

// Test Scenario 3: Non-Retryable Error Short-circuiting
// Simulates: 400 Bad Request -> Stop immediately
func TestClient_ChatCompletionStream_NonRetryableError(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusBadRequest) // 400
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	prov := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "bad-req-prov", models: []string{"test-model"}, baseURL: server.URL,
	}}

	dep := &provider.Deployment{ID: "dep1", ProviderName: "bad-req-prov", ModelName: "test-model"}
	mockRouter := &sequenceRouter{
		deployments: []*provider.Deployment{dep, dep, dep},
	}

	client, err := New(
		WithProviderInstance("bad-req-prov", prov, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(3, 10*time.Millisecond),
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Should only try once
	if atomic.LoadInt32(&reqCount) != 1 {
		t.Errorf("Expected 1 request (no retry), got %d", reqCount)
	}
}

// Test Scenario 4: Context Cancellation
// Simulates: Cancel context during backoff
func TestClient_ChatCompletionStream_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // Force retry
	}))
	defer server.Close()

	prov := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "cancel-prov", models: []string{"test-model"}, baseURL: server.URL,
	}}

	dep := &provider.Deployment{ID: "dep1", ProviderName: "cancel-prov", ModelName: "test-model"}
	mockRouter := &sequenceRouter{
		deployments: []*provider.Deployment{dep, dep, dep},
	}

	client, err := New(
		WithProviderInstance("cancel-prov", prov, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(3, 200*time.Millisecond), // Long backoff
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context shortly after start
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = client.ChatCompletionStream(ctx, &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Should return faster than the full backoff (200ms)
	if duration >= 200*time.Millisecond {
		t.Errorf("Expected cancellation to interrupt backoff, but took %v", duration)
	}
}

// Test Scenario 5: Timeout Retry
// Simulates: Connection Hang -> Timeout -> Retry -> Success
func TestClient_ChatCompletionStream_TimeoutRetry(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&reqCount, 1)
		if count == 1 {
			// Hang longer than client timeout
			time.Sleep(100 * time.Millisecond)
			return
		}
		// Success
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	prov := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "timeout-prov", models: []string{"test-model"}, baseURL: server.URL,
	}}

	dep := &provider.Deployment{ID: "dep1", ProviderName: "timeout-prov", ModelName: "test-model"}
	mockRouter := &sequenceRouter{
		deployments: []*provider.Deployment{dep, dep},
	}

	client, err := New(
		WithProviderInstance("timeout-prov", prov, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(3, 10*time.Millisecond),
		WithTimeout(50*time.Millisecond), // Short timeout
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})
	if err != nil {
		t.Fatalf("Expected success after timeout retry, got error: %v", err)
	}
	defer stream.Close()

	if atomic.LoadInt32(&reqCount) != 2 {
		t.Errorf("Expected 2 requests (1 timeout + 1 retry), got %d", reqCount)
	}
}

// Test Scenario 6: 429 Rate Limit Retry
// Simulates: 429 -> Success
func TestClient_ChatCompletionStream_RateLimitRetry(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&reqCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests) // 429
			w.Write([]byte(`{"error": "rate limit exceeded"}`))
			return
		}
		// Success
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	// Need a provider that maps 429 to Retryable error
	// The mockProvider in client_test.go returns NewInternalError by default
	// We need to override MapError for this test
	prov := &rateLimitMockProvider{
		streamMockProvider: &streamMockProvider{
			httpMockProvider: &httpMockProvider{
				name: "ratelimit-prov", models: []string{"test-model"}, baseURL: server.URL,
			},
		},
	}

	dep := &provider.Deployment{ID: "dep1", ProviderName: "ratelimit-prov", ModelName: "test-model"}
	mockRouter := &sequenceRouter{
		deployments: []*provider.Deployment{dep, dep},
	}

	client, err := New(
		WithProviderInstance("ratelimit-prov", prov, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(3, 10*time.Millisecond),
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	stream, err := client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})
	if err != nil {
		t.Fatalf("Expected success after rate limit retry, got error: %v", err)
	}
	defer stream.Close()

	if atomic.LoadInt32(&reqCount) != 2 {
		t.Errorf("Expected 2 requests (1 rate limit + 1 retry), got %d", reqCount)
	}
}

// rateLimitMockProvider overrides MapError to return retryable error for 429
type rateLimitMockProvider struct {
	*streamMockProvider
}

func (m *rateLimitMockProvider) MapError(statusCode int, body []byte) error {
	if statusCode == http.StatusTooManyRequests {
		return errors.NewRateLimitError(m.Name(), "test-model", "rate limit exceeded")
	}
	return m.streamMockProvider.MapError(statusCode, body)
}

// Test Scenario 7: Fallback Disabled
// Simulates: Failure -> Retry on SAME deployment (no new Pick)
func TestClient_ChatCompletionStream_FallbackDisabled(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	prov := &streamMockProvider{httpMockProvider: &httpMockProvider{
		name: "nofallback-prov", models: []string{"test-model"}, baseURL: server.URL,
	}}

	// Router has multiple deployments, but we should only pick ONCE
	dep1 := &provider.Deployment{ID: "dep1", ProviderName: "nofallback-prov", ModelName: "test-model"}
	dep2 := &provider.Deployment{ID: "dep2", ProviderName: "nofallback-prov", ModelName: "test-model"}

	// We use a counting router to verify Pick calls
	mockRouter := &countingRouter{
		sequenceRouter: sequenceRouter{
			deployments: []*provider.Deployment{dep1, dep2},
		},
	}

	client, err := New(
		WithProviderInstance("nofallback-prov", prov, []string{"test-model"}),
		WithRouter(mockRouter),
		WithRetry(2, 10*time.Millisecond),
		WithFallback(false), // Disable fallback
		WithCooldown(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletionStream(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Should have tried 3 times (1 initial + 2 retries)
	if atomic.LoadInt32(&reqCount) != 3 {
		t.Errorf("Expected 3 requests, got %d", reqCount)
	}

	// Should have called Pick only ONCE
	if mockRouter.pickCount != 1 {
		t.Errorf("Expected 1 Pick call, got %d", mockRouter.pickCount)
	}
}

type countingRouter struct {
	sequenceRouter
	pickCount int
}

func (r *countingRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	r.pickCount++
	return r.sequenceRouter.Pick(ctx, model)
}
