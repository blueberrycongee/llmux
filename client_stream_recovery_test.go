package llmux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// TestStreamRecovery_MidStreamFailure verifies that the client can recover from a mid-stream failure
// by connecting to a fallback provider and continuing the generation.
func TestStreamRecovery_MidStreamFailure(t *testing.T) {
	// Server A: The primary server that fails mid-stream
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a retry request (contains partial context)
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "Hello, ") {
			// If we see the partial context, it means we are being retried.
			// Fail hard so the client picks another provider.
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send first chunk
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"Hello, "}}]}`)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)

		// Send second chunk
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"this is "}}]}`)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)

		// Simulate hard failure (TCP reset / close connection) without sending [DONE]
		// This causes io.UnexpectedEOF or similar error on client side
		// We just close the handler, which closes the body.
	}))
	defer serverA.Close()

	// Server B: The fallback server that completes the request
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body to verify context reconstruction
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		// Assert that the request contains the previous context
		// We expect the last message to be the assistant's partial response: "Hello, this is "
		expectedContext := "Hello, this is "
		if !strings.Contains(bodyStr, expectedContext) {
			t.Errorf("Server B did not receive the recovered context. Body: %s", bodyStr)
			http.Error(w, "missing context", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")

		// Send the rest of the content
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"a resilient "}}]}`)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"system."}}]}`)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)

		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer serverB.Close()

	// Setup Client
	client, err := New(
		WithProvider(ProviderConfig{
			Name:    "providerA",
			Type:    "openai", // Use openai compatible provider
			Models:  []string{"gpt-test"},
			APIKey:  "test-key",
			BaseURL: serverA.URL,
		}),
		WithProvider(ProviderConfig{
			Name:    "providerB",
			Type:    "openai",
			Models:  []string{"gpt-test"},
			APIKey:  "test-key",
			BaseURL: serverB.URL,
		}),
		WithFallback(true), // Enable fallback
		WithRetry(3, 10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Manually configure router to ensure deterministic order for the test
	// We want providerA to be picked first, then providerB.
	trackingR := newTrackingRouter(client.router)
	client.router = trackingR

	// Get deployments
	deployments := trackingR.GetDeployments("gpt-test")
	if len(deployments) < 2 {
		t.Fatalf("Expected 2 deployments, got %d", len(deployments))
	}

	var depA, depB *provider.Deployment
	for _, d := range deployments {
		switch d.ProviderName {
		case "providerA":
			depA = d
		case "providerB":
			depB = d
		}
	}
	if depA == nil || depB == nil {
		t.Fatalf("Could not find deployments for providerA and providerB")
	}

	// Force order: A (fails), then B (recovers)
	trackingR.pickDeployments = []*provider.Deployment{depA, depB}

	ctx := context.Background()
	req := &ChatRequest{
		Model: "gpt-test",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Say something"`)},
		},
	}

	stream, err := client.ChatCompletionStream(ctx, req)
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	var fullContent strings.Builder

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
		fullContent.WriteString(chunk.Choices[0].Delta.Content)
	}

	expected := "Hello, this is a resilient system."
	if fullContent.String() != expected {
		t.Errorf("Expected content %q, got %q", expected, fullContent.String())
	}
}

// trackingRouter wraps a router.Router to track ReportRequestStart and ReportRequestEnd calls
// for verifying proper lifecycle management during recovery.
type trackingRouter struct {
	router.Router
	mu              sync.Mutex
	startCalls      map[string]int // deploymentID -> count
	endCalls        map[string]int // deploymentID -> count
	pickIndex       int
	pickDeployments []*provider.Deployment
}

func newTrackingRouter(r router.Router) *trackingRouter {
	return &trackingRouter{
		Router:     r,
		startCalls: make(map[string]int),
		endCalls:   make(map[string]int),
	}
}

func (t *trackingRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pickDeployments != nil && t.pickIndex < len(t.pickDeployments) {
		d := t.pickDeployments[t.pickIndex]
		t.pickIndex++
		return d, nil
	}
	return t.Router.Pick(ctx, model)
}

func (t *trackingRouter) ReportRequestStart(deployment *provider.Deployment) {
	t.mu.Lock()
	t.startCalls[deployment.ID]++
	t.mu.Unlock()
	t.Router.ReportRequestStart(deployment)
}

func (t *trackingRouter) ReportRequestEnd(deployment *provider.Deployment) {
	t.mu.Lock()
	t.endCalls[deployment.ID]++
	t.mu.Unlock()
	t.Router.ReportRequestEnd(deployment)
}

func (t *trackingRouter) GetCounts() (startCalls, endCalls map[string]int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Return copies
	sc := make(map[string]int)
	ec := make(map[string]int)
	for k, v := range t.startCalls {
		sc[k] = v
	}
	for k, v := range t.endCalls {
		ec[k] = v
	}
	return sc, ec
}

// TestStreamReader_RecoveryRequestLifecycle verifies that during recovery:
// - ReportRequestEnd is called exactly once per deployment
// - No duplicate ReportRequestEnd calls occur even when retries fail
//
// Scenario:
// 1. First deployment (D1): stream starts, then fails mid-stream
// 2. Recovery picks D2: HTTP request fails (e.g., 503)
// 3. Recovery picks D3: stream succeeds and completes
//
// Expected:
// - D1: Start=1, End=1 (stream starts, then closed on failure)
// - D2: Start=1, End=1 (HTTP error reported)
// - D3: Start=1, End=1 (success, then stream closed)
func TestStreamReader_RecoveryRequestLifecycle(t *testing.T) {
	// Server 1: Fails mid-stream (no [DONE])
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "partial content") {
			// This is a retry, fail with 503
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"partial content"}}]}`)
		w.(http.Flusher).Flush()
		// Close without [DONE] to simulate mid-stream failure
	}))
	defer server1.Close()

	// Server 2: Always returns 503 (to simulate recovery picking a bad node)
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}))
	defer server2.Close()

	// Server 3: Successfully completes the request
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"final content"}}]}`)
		w.(http.Flusher).Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server3.Close()

	// Create client with 3 providers
	client, err := New(
		WithProvider(ProviderConfig{
			Name:    "provider1",
			Type:    "openai",
			Models:  []string{"test-model"},
			APIKey:  "test-key",
			BaseURL: server1.URL,
		}),
		WithProvider(ProviderConfig{
			Name:    "provider2",
			Type:    "openai",
			Models:  []string{"test-model"},
			APIKey:  "test-key",
			BaseURL: server2.URL,
		}),
		WithProvider(ProviderConfig{
			Name:    "provider3",
			Type:    "openai",
			Models:  []string{"test-model"},
			APIKey:  "test-key",
			BaseURL: server3.URL,
		}),
		WithFallback(true),
		WithRetry(5, 10*time.Millisecond), // Allow enough retries
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Replace router with tracking wrapper
	trackingR := newTrackingRouter(client.router)
	client.router = trackingR

	// Get deployments and configure deterministic pick order
	deployments := trackingR.GetDeployments("test-model")
	if len(deployments) < 3 {
		t.Fatalf("Expected 3 deployments, got %d", len(deployments))
	}

	// Find deployments by provider name
	var d1, d2, d3 *provider.Deployment
	for _, d := range deployments {
		switch d.ProviderName {
		case "provider1":
			d1 = d
		case "provider2":
			d2 = d
		case "provider3":
			d3 = d
		}
	}
	if d1 == nil || d2 == nil || d3 == nil {
		t.Fatalf("Could not find all deployments")
	}

	// Configure pick order: D1 (initial), D2 (first recovery - will fail), D3 (second recovery - success)
	trackingR.pickDeployments = []*provider.Deployment{d1, d2, d3}

	ctx := context.Background()
	req := &ChatRequest{
		Model: "test-model",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Say something"`)},
		},
	}

	stream, err := client.ChatCompletionStream(ctx, req)
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Consume the stream
	var content strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected stream error: %v", err)
		}
		if len(chunk.Choices) > 0 {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	// Verify content (we should see content from D3 after recovery)
	if !strings.Contains(content.String(), "final content") {
		t.Errorf("Expected final content in output, got: %s", content.String())
	}

	// Verify lifecycle: each deployment should have exactly 1 Start and 1 End
	startCalls, endCalls := trackingR.GetCounts()

	// D1: Started by initial ChatCompletionStream, ended by tryRecover
	if startCalls[d1.ID] != 1 {
		t.Errorf("D1 Start: expected 1, got %d", startCalls[d1.ID])
	}
	if endCalls[d1.ID] != 1 {
		t.Errorf("D1 End: expected 1, got %d", endCalls[d1.ID])
	}

	// D2: Started by tryRecover, ended immediately due to HTTP 503 error
	if startCalls[d2.ID] != 1 {
		t.Errorf("D2 Start: expected 1, got %d", startCalls[d2.ID])
	}
	if endCalls[d2.ID] != 1 {
		t.Errorf("D2 End: expected 1, got %d", endCalls[d2.ID])
	}

	// D3: Started by tryRecover, ended by stream.Close() or finish()
	if startCalls[d3.ID] != 1 {
		t.Errorf("D3 Start: expected 1, got %d", startCalls[d3.ID])
	}
	if endCalls[d3.ID] != 1 {
		t.Errorf("D3 End: expected 1, got %d", endCalls[d3.ID])
	}
}
