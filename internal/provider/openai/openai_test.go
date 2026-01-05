package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestNew(t *testing.T) {
	t.Run("with default base URL", func(t *testing.T) {
		cfg := provider.ProviderConfig{
			APIKey: "test-key",
			Models: []string{"gpt-4"},
		}

		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if p.Name() != ProviderName {
			t.Errorf("Name() = %s, want %s", p.Name(), ProviderName)
		}
	})

	t.Run("with custom base URL", func(t *testing.T) {
		cfg := provider.ProviderConfig{
			APIKey:  "test-key",
			BaseURL: "https://custom.api.com/v1/",
			Models:  []string{"gpt-4"},
		}

		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		op := p.(*Provider)
		if op.baseURL != "https://custom.api.com/v1" {
			t.Errorf("baseURL = %s, want https://custom.api.com/v1", op.baseURL)
		}
	})
}

func TestProvider_SupportsModel(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"gpt-4", "gpt-3.5-turbo"},
	}

	p, _ := New(cfg)

	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4", true},
		{"gpt-3.5-turbo", true},
		{"gpt-4o", true},  // Matches gpt- prefix
		{"o1-mini", true}, // Matches o1- prefix
		{"claude-3", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := p.SupportsModel(tt.model); got != tt.want {
				t.Errorf("SupportsModel(%s) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestProvider_BuildRequest(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey:  "test-api-key",
		BaseURL: "https://api.openai.com/v1",
		Models:  []string{"gpt-4"},
	}

	p, _ := New(cfg)

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	httpReq, err := p.BuildRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	// Check URL
	if httpReq.URL.String() != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("URL = %s, want https://api.openai.com/v1/chat/completions", httpReq.URL.String())
	}

	// Check method
	if httpReq.Method != http.MethodPost {
		t.Errorf("Method = %s, want POST", httpReq.Method)
	}

	// Check headers
	if httpReq.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header should be application/json")
	}

	if httpReq.Header.Get("Authorization") != "Bearer test-api-key" {
		t.Error("Authorization header should be set correctly")
	}
}

func TestProvider_ParseResponse(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"gpt-4"},
	}

	p, _ := New(cfg)

	t.Run("valid response", func(t *testing.T) {
		body := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello!"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15
			}
		}`

		resp := &http.Response{
			StatusCode: 200,
			Body:       newReadCloser(body),
		}

		chatResp, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("ParseResponse() error = %v", err)
		}

		if chatResp.ID != "chatcmpl-123" {
			t.Errorf("ID = %s, want chatcmpl-123", chatResp.ID)
		}

		if chatResp.Model != "gpt-4" {
			t.Errorf("Model = %s, want gpt-4", chatResp.Model)
		}

		if len(chatResp.Choices) != 1 {
			t.Fatalf("Choices count = %d, want 1", len(chatResp.Choices))
		}

		if chatResp.Usage == nil {
			t.Fatal("Usage should not be nil")
		}

		if chatResp.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", chatResp.Usage.TotalTokens)
		}
	})
}

func TestProvider_ParseStreamChunk(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"gpt-4"},
	}

	p, _ := New(cfg)

	tests := []struct {
		name    string
		data    []byte
		wantNil bool
		wantErr bool
	}{
		{
			name:    "empty line",
			data:    []byte(""),
			wantNil: true,
		},
		{
			name:    "DONE marker",
			data:    []byte("[DONE]"),
			wantNil: true,
		},
		{
			name:    "data prefix with DONE",
			data:    []byte("data: [DONE]"),
			wantNil: true,
		},
		{
			name:    "valid chunk",
			data:    []byte(`data: {"id":"123","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hi"}}]}`),
			wantNil: false,
		},
		{
			name:    "invalid json",
			data:    []byte(`data: {invalid`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := p.ParseStreamChunk(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseStreamChunk() error = %v", err)
			}

			if tt.wantNil && chunk != nil {
				t.Error("expected nil chunk")
			}

			if !tt.wantNil && chunk == nil {
				t.Error("expected non-nil chunk")
			}
		})
	}
}

func TestProvider_MapError(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"gpt-4"},
	}

	p, _ := New(cfg)

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantType   string
	}{
		{
			name:       "rate limit",
			statusCode: 429,
			body:       `{"error":{"message":"Rate limit exceeded"}}`,
			wantType:   "rate_limit_error",
		},
		{
			name:       "unauthorized",
			statusCode: 401,
			body:       `{"error":{"message":"Invalid API key"}}`,
			wantType:   "authentication_error",
		},
		{
			name:       "bad request",
			statusCode: 400,
			body:       `{"error":{"message":"Invalid model"}}`,
			wantType:   "invalid_request_error",
		},
		{
			name:       "not found",
			statusCode: 404,
			body:       `{"error":{"message":"Model not found"}}`,
			wantType:   "not_found_error",
		},
		{
			name:       "server error",
			statusCode: 500,
			body:       `{"error":{"message":"Internal error"}}`,
			wantType:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.MapError(tt.statusCode, []byte(tt.body))
			if err == nil {
				t.Fatal("expected error")
			}

			// Check error message contains expected type
			errMsg := err.Error()
			if !containsString(errMsg, tt.wantType) {
				t.Errorf("error type = %s, want to contain %s", errMsg, tt.wantType)
			}
		})
	}
}

func TestProvider_Integration(t *testing.T) {
	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing or invalid Authorization header")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from mock!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	cfg := provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Models:  []string{"gpt-4"},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	httpReq, err := p.BuildRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	chatResp, err := p.ParseResponse(resp)
	if err != nil {
		t.Fatalf("ParseResponse() error = %v", err)
	}

	if chatResp.ID != "chatcmpl-test" {
		t.Errorf("ID = %s, want chatcmpl-test", chatResp.ID)
	}
}

// Helper functions

type readCloser struct {
	data []byte
	pos  int
}

func newReadCloser(s string) *readCloser {
	return &readCloser{data: []byte(s)}
}

func (r *readCloser) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *readCloser) Close() error {
	return nil
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
