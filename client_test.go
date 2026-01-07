package llmux

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockProvider implements Provider interface for testing.
type mockProvider struct {
	name   string
	models []string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) SupportedModels() []string {
	return m.models
}

func (m *mockProvider) SupportsModel(model string) bool {
	for _, mod := range m.models {
		if mod == model {
			return true
		}
	}
	return false
}

func (m *mockProvider) BuildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, "POST", "http://localhost/v1/chat/completions", nil)
}

func (m *mockProvider) ParseResponse(resp *http.Response) (*ChatResponse, error) {
	body, _ := io.ReadAll(resp.Body)
	var chatResp ChatResponse
	json.Unmarshal(body, &chatResp)
	return &chatResp, nil
}

func (m *mockProvider) ParseStreamChunk(data []byte) (*StreamChunk, error) {
	return nil, nil
}

func (m *mockProvider) MapError(statusCode int, body []byte) error {
	return NewInternalError(m.name, "", "error")
}

func TestNew_EmptyConfig(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("New() returned nil client")
	}
}

func TestNew_WithProviderInstance(t *testing.T) {
	mock := &mockProvider{name: "test", models: []string{"test-model"}}

	client, err := New(
		WithProviderInstance("test", mock, []string{"test-model"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	providers := client.GetProviders()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
}

func TestClient_ListModels(t *testing.T) {
	mock := &mockProvider{name: "test", models: []string{"model-a", "model-b"}}

	client, err := New(
		WithProviderInstance("test", mock, []string{"model-a", "model-b"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestClient_AddProvider(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	mock := &mockProvider{name: "dynamic", models: []string{"dynamic-model"}}
	err = client.AddProvider("dynamic", mock, []string{"dynamic-model"})
	if err != nil {
		t.Fatalf("AddProvider() error = %v", err)
	}

	providers := client.GetProviders()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
}

func TestClient_AddProvider_Duplicate(t *testing.T) {
	mock := &mockProvider{name: "test", models: []string{"model"}}

	client, err := New(
		WithProviderInstance("test", mock, []string{"model"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	err = client.AddProvider("test", mock, []string{"model"})
	if err == nil {
		t.Error("expected error for duplicate provider")
	}
}

func TestClient_RemoveProvider(t *testing.T) {
	mock := &mockProvider{name: "test", models: []string{"model"}}

	client, err := New(
		WithProviderInstance("test", mock, []string{"model"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	err = client.RemoveProvider("test")
	if err != nil {
		t.Fatalf("RemoveProvider() error = %v", err)
	}

	providers := client.GetProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}
}

func TestClient_RemoveProvider_NotFound(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	err = client.RemoveProvider("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestClient_ChatCompletion_NilRequest(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestClient_ChatCompletion_MissingModel(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
	})
	if err == nil {
		t.Error("expected error for missing model")
	}
}

func TestClient_ChatCompletion_MissingMessages(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Model: "test-model",
	})
	if err == nil {
		t.Error("expected error for missing messages")
	}
}

func TestClient_ChatCompletion_NoDeployment(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Model:    "nonexistent-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
	})
	if err == nil {
		t.Error("expected error for no deployment")
	}
}

func TestOptions_WithRouterStrategy(t *testing.T) {
	client, err := New(
		WithRouterStrategy(StrategyLowestLatency),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.config.RouterStrategy != StrategyLowestLatency {
		t.Errorf("expected strategy %s, got %s", StrategyLowestLatency, client.config.RouterStrategy)
	}
}

func TestOptions_WithRetry(t *testing.T) {
	client, err := New(
		WithRetry(5, 2*time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.config.RetryCount != 5 {
		t.Errorf("expected retry count 5, got %d", client.config.RetryCount)
	}
	if client.config.RetryBackoff != 2*time.Second {
		t.Errorf("expected retry backoff 2s, got %v", client.config.RetryBackoff)
	}
}

func TestOptions_WithFallback(t *testing.T) {
	client, err := New(
		WithFallback(false),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.config.FallbackEnabled {
		t.Error("expected fallback disabled")
	}
}

func TestOptions_WithTimeout(t *testing.T) {
	client, err := New(
		WithTimeout(60 * time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.config.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", client.config.Timeout)
	}
}

func TestOptions_WithCooldown(t *testing.T) {
	client, err := New(
		WithCooldown(120 * time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.config.CooldownPeriod != 120*time.Second {
		t.Errorf("expected cooldown 120s, got %v", client.config.CooldownPeriod)
	}
}

func TestSimpleRouter_Pick(t *testing.T) {
	router := newSimpleRouter(60*time.Second, StrategySimpleShuffle)

	deployment := &Deployment{
		ID:           "test-1",
		ProviderName: "test",
		ModelName:    "test-model",
	}
	router.AddDeployment(deployment)

	picked, err := router.Pick(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}

	if picked.ID != "test-1" {
		t.Errorf("expected deployment test-1, got %s", picked.ID)
	}
}

func TestSimpleRouter_Pick_NoDeployment(t *testing.T) {
	router := newSimpleRouter(60*time.Second, StrategySimpleShuffle)

	_, err := router.Pick(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for no deployment")
	}
}

func TestSimpleRouter_Cooldown(t *testing.T) {
	router := newSimpleRouter(100*time.Millisecond, StrategySimpleShuffle)

	deployment := &Deployment{
		ID:           "test-1",
		ProviderName: "test",
		ModelName:    "test-model",
	}
	router.AddDeployment(deployment)

	// Report failure to trigger cooldown
	router.ReportFailure(deployment, NewRateLimitError("test", "test-model", "rate limited"))

	// Should fail during cooldown
	_, err := router.Pick(context.Background(), "test-model")
	if err == nil {
		t.Error("expected error during cooldown")
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Should succeed after cooldown
	picked, err := router.Pick(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("Pick() after cooldown error = %v", err)
	}
	if picked.ID != "test-1" {
		t.Errorf("expected deployment test-1, got %s", picked.ID)
	}
}

func TestSimpleRouter_Stats(t *testing.T) {
	router := newSimpleRouter(60*time.Second, StrategySimpleShuffle)

	deployment := &Deployment{
		ID:           "test-1",
		ProviderName: "test",
		ModelName:    "test-model",
	}
	router.AddDeployment(deployment)

	// Report some metrics
	router.ReportRequestStart(deployment)
	router.ReportSuccess(deployment, &ResponseMetrics{
		Latency:      100 * time.Millisecond,
		InputTokens:  10,
		OutputTokens: 20,
	})
	router.ReportRequestEnd(deployment)

	stats := router.GetStats("test-1")
	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	if stats.TotalRequests != 1 {
		t.Errorf("expected 1 total request, got %d", stats.TotalRequests)
	}
	if stats.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", stats.SuccessCount)
	}
}

// httpMockProvider is a provider that makes real HTTP requests to a mock server.
type httpMockProvider struct {
	name    string
	models  []string
	baseURL string
}

func (m *httpMockProvider) Name() string {
	return m.name
}

func (m *httpMockProvider) SupportedModels() []string {
	return m.models
}

func (m *httpMockProvider) SupportsModel(model string) bool {
	for _, mod := range m.models {
		if mod == model {
			return true
		}
	}
	return false
}

func (m *httpMockProvider) BuildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (m *httpMockProvider) ParseResponse(resp *http.Response) (*ChatResponse, error) {
	body, _ := io.ReadAll(resp.Body)
	var chatResp ChatResponse
	json.Unmarshal(body, &chatResp)
	return &chatResp, nil
}

func (m *httpMockProvider) ParseStreamChunk(data []byte) (*StreamChunk, error) {
	return nil, nil
}

func (m *httpMockProvider) MapError(statusCode int, body []byte) error {
	return NewInternalError(m.name, "", "error")
}

// Integration test with mock HTTP server
func TestClient_ChatCompletion_Integration(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
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

	// Create provider that uses mock server
	mock := &httpMockProvider{
		name:    "mock",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock", mock, []string{"test-model"}),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	resp, err := client.ChatCompletion(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if resp.ID != "test-id" {
		t.Errorf("expected ID test-id, got %s", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(resp.Choices))
	}
}
