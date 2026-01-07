// Package testutil provides testing utilities for E2E tests.
package testutil

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
)

// RecordedRequest stores information about a received request.
type RecordedRequest struct {
	Method  string
	Path    string
	Body    []byte
	Headers http.Header
	Time    time.Time
}

// MockResponse defines a custom response for the mock server.
type MockResponse struct {
	Content    string
	StatusCode int
	Error      *MockError
	Delay      time.Duration
}

// MockError defines an error response.
type MockError struct {
	Message string
	Type    string
	Code    string
}

// MockLLMServer simulates an OpenAI-compatible LLM API for testing.
type MockLLMServer struct {
	server   *httptest.Server
	requests []RecordedRequest
	mu       sync.Mutex

	// Configurable behavior
	Latency       time.Duration // Simulated latency
	ErrorRate     float64       // Error rate (0-1)
	StreamDelay   time.Duration // Delay between stream chunks
	DefaultModel  string        // Default model name in responses
	responseQueue []MockResponse
	nextError     *MockError
	nextStatus    int
}

// NewMockLLMServer creates and starts a new mock LLM server.
func NewMockLLMServer() *MockLLMServer {
	m := &MockLLMServer{
		requests:     make([]RecordedRequest, 0),
		DefaultModel: "gpt-4o-mock",
	}

	mux := http.NewServeMux()
	// Support both /v1/... and /... paths for flexibility
	mux.HandleFunc("/v1/chat/completions", m.handleChatCompletions)
	mux.HandleFunc("/chat/completions", m.handleChatCompletions)
	mux.HandleFunc("/v1/models", m.handleListModels)
	mux.HandleFunc("/models", m.handleListModels)
	mux.HandleFunc("/v1/embeddings", m.handleEmbeddings)
	mux.HandleFunc("/embeddings", m.handleEmbeddings)
	mux.HandleFunc("/health", m.handleHealth)

	m.server = httptest.NewServer(mux)
	return m
}

// URL returns the mock server's URL.
func (m *MockLLMServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server.
func (m *MockLLMServer) Close() {
	m.server.Close()
}

// GetRequests returns all recorded requests.
func (m *MockLLMServer) GetRequests() []RecordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]RecordedRequest, len(m.requests))
	copy(result, m.requests)
	return result
}

// Reset clears all recorded requests and resets configuration.
func (m *MockLLMServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
	m.responseQueue = m.responseQueue[:0]
	m.nextError = nil
	m.nextStatus = 0
	m.Latency = 0
	m.ErrorRate = 0
}

// SetNextResponse sets the content for the next response.
func (m *MockLLMServer) SetNextResponse(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseQueue = append(m.responseQueue, MockResponse{Content: content})
}

// SetNextError sets an error for the next request.
func (m *MockLLMServer) SetNextError(statusCode int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextStatus = statusCode
	m.nextError = &MockError{
		Message: message,
		Type:    "api_error",
		Code:    fmt.Sprintf("error_%d", statusCode),
	}
}

// QueueResponse adds a response to the queue.
func (m *MockLLMServer) QueueResponse(resp MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseQueue = append(m.responseQueue, resp)
}

func (m *MockLLMServer) recordRequest(r *http.Request, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Body:    body,
		Headers: r.Header.Clone(),
		Time:    time.Now(),
	})
}

func (m *MockLLMServer) getNextResponse() *MockResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.responseQueue) > 0 {
		resp := m.responseQueue[0]
		m.responseQueue = m.responseQueue[1:]
		return &resp
	}
	return nil
}

func (m *MockLLMServer) getAndClearError() (*MockError, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.nextError
	status := m.nextStatus
	m.nextError = nil
	m.nextStatus = 0
	return err, status
}

func (m *MockLLMServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Read and record request
	body, _ := readBody(r) //nolint:errcheck // test code
	m.recordRequest(r, body)

	// Apply latency
	if m.Latency > 0 {
		time.Sleep(m.Latency)
	}

	// Check for configured error
	if err, status := m.getAndClearError(); err != nil {
		writeErrorResponse(w, status, err)
		return
	}

	// Parse request to check for streaming
	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &req) //nolint:errcheck // ignore error in test code

	// Get custom response content
	content := "Hello! This is a mock response from the test server."
	if resp := m.getNextResponse(); resp != nil {
		if resp.Delay > 0 {
			time.Sleep(resp.Delay)
		}
		if resp.Error != nil {
			writeErrorResponse(w, resp.StatusCode, resp.Error)
			return
		}
		content = resp.Content
	}

	model := req.Model
	if model == "" {
		model = m.DefaultModel
	}

	if req.Stream {
		m.handleStreamingResponse(w, model, content)
		return
	}

	// Non-streaming response
	resp := map[string]any{
		"id":      "chatcmpl-mock-" + randomID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockLLMServer) handleStreamingResponse(w http.ResponseWriter, model, content string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	id := "chatcmpl-mock-" + randomID()
	created := time.Now().Unix()

	// Split content into chunks
	chunks := splitIntoChunks(content, 5)

	for i, chunk := range chunks {
		// Build delta content
		delta := map[string]any{
			"content": chunk,
		}
		// Add role in first chunk
		if i == 0 {
			delta["role"] = "assistant"
		}

		choice := map[string]any{
			"index": 0,
			"delta": delta,
		}
		// Add finish_reason in last chunk
		if i == len(chunks)-1 {
			choice["finish_reason"] = "stop"
		}

		data := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{choice},
		}

		jsonData, _ := json.Marshal(data) //nolint:errcheck // test code
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()

		if m.StreamDelay > 0 {
			time.Sleep(m.StreamDelay)
		}
	}

	// Send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (m *MockLLMServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	m.recordRequest(r, nil)

	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       "gpt-4o-mock",
				"object":   "model",
				"created":  1700000000,
				"owned_by": "mock",
			},
			{
				"id":       "gpt-3.5-turbo-mock",
				"object":   "model",
				"created":  1700000000,
				"owned_by": "mock",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockLLMServer) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	body, _ := readBody(r) //nolint:errcheck // test code
	m.recordRequest(r, body)

	// Generate mock embedding (1536 dimensions for ada-002 compatibility)
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = float64(i) * 0.001
	}

	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"object":    "embedding",
				"index":     0,
				"embedding": embedding,
			},
		},
		"model": "text-embedding-ada-002",
		"usage": map[string]int{
			"prompt_tokens": 5,
			"total_tokens":  5,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockLLMServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Helper functions

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	var buf strings.Builder
	scanner := bufio.NewScanner(r.Body)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
	}
	return []byte(buf.String()), scanner.Err()
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, err *MockError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": err.Message,
			"type":    err.Type,
			"code":    err.Code,
		},
	})
}

func randomID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func splitIntoChunks(s string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	if len(chunks) == 0 {
		chunks = []string{""}
	}
	return chunks
}
