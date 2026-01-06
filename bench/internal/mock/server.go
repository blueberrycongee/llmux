// Package mock provides a mock LLM server for benchmarking.
// It simulates OpenAI-compatible API responses without making real API calls.
package mock

import (
	"github.com/goccy/go-json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Server is a mock LLM API server.
type Server struct {
	// Latency simulates API processing time.
	Latency time.Duration

	// RequestCount tracks total requests handled.
	RequestCount atomic.Int64

	// ErrorRate is the probability of returning an error (0.0 to 1.0).
	ErrorRate float64
}

// NewServer creates a new mock server with default settings.
func NewServer() *Server {
	return &Server{
		Latency:   50 * time.Millisecond,
		ErrorRate: 0.0,
	}
}

// ChatCompletionRequest represents the request body.
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents the response body.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Handler returns an http.Handler for the mock server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)

	return mux
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	s.RequestCount.Add(1)

	// Read request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Simulate latency
	if s.Latency > 0 {
		time.Sleep(s.Latency)
	}

	// Handle streaming
	if req.Stream {
		s.handleStreamingResponse(w, req)
		return
	}

	// Non-streaming response
	resp := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "Hello! This is a mock response for benchmarking. The actual content would come from the LLM.",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     countTokens(req.Messages),
			CompletionTokens: 20,
			TotalTokens:      countTokens(req.Messages) + 20,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStreamingResponse(w http.ResponseWriter, req ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	chunks := []string{"Hello", "!", " This", " is", " a", " mock", " streaming", " response", "."}

	for i, chunk := range chunks {
		data := map[string]any{
			"id":      fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": chunk,
					},
					"finish_reason": nil,
				},
			},
		}

		if i == len(chunks)-1 {
			if choices, ok := data["choices"].([]map[string]any); ok && len(choices) > 0 {
				choices[0]["finish_reason"] = "stop"
			}
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()

		time.Sleep(10 * time.Millisecond)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleModels(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": "gpt-4o-mock", "object": "model", "owned_by": "mock"},
			{"id": "gpt-4-mock", "object": "model", "owned_by": "mock"},
			{"id": "gpt-3.5-turbo-mock", "object": "model", "owned_by": "mock"},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"status":        "ok",
		"request_count": s.RequestCount.Load(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func countTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4 // Rough estimate: 4 chars per token
	}
	return total
}

// Stats returns server statistics.
func (s *Server) Stats() map[string]any {
	return map[string]any{
		"request_count": s.RequestCount.Load(),
		"latency_ms":    s.Latency.Milliseconds(),
	}
}
