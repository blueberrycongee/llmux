package openailike

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		cfg      provider.ProviderConfig
		info     ProviderInfo
		wantName string
	}{
		{
			name: "creates provider with custom base URL",
			cfg: provider.ProviderConfig{
				APIKey:  "test-key",
				BaseURL: "https://custom.api.com/v1",
				Models:  []string{"model-1"},
			},
			info: ProviderInfo{
				Name:           "test-provider",
				DefaultBaseURL: "https://default.api.com/v1",
			},
			wantName: "test-provider",
		},
		{
			name: "uses default base URL when not specified",
			cfg: provider.ProviderConfig{
				APIKey: "test-key",
				Models: []string{"model-1"},
			},
			info: ProviderInfo{
				Name:           "test-provider",
				DefaultBaseURL: "https://default.api.com/v1",
			},
			wantName: "test-provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.cfg, tt.info)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if p.Name() != tt.wantName {
				t.Errorf("Name() = %v, want %v", p.Name(), tt.wantName)
			}
		})
	}
}

func TestProvider_SupportsModel(t *testing.T) {
	p, _ := New(
		provider.ProviderConfig{
			APIKey: "test-key",
			Models: []string{"explicit-model"},
		},
		ProviderInfo{
			Name:           "test",
			DefaultBaseURL: "https://api.test.com/v1",
			ModelPrefixes:  []string{"llama-", "mixtral-"},
		},
	)

	tests := []struct {
		model string
		want  bool
	}{
		{"explicit-model", true},
		{"llama-3.1-70b", true},
		{"mixtral-8x7b", true},
		{"gpt-4", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := p.SupportsModel(tt.model); got != tt.want {
				t.Errorf("SupportsModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestProvider_BuildRequest(t *testing.T) {
	p, _ := New(
		provider.ProviderConfig{
			APIKey: "test-api-key",
		},
		ProviderInfo{
			Name:              "test",
			DefaultBaseURL:    "https://api.test.com/v1",
			SupportsStreaming: true,
		},
	)

	req := &types.ChatRequest{
		Model: "test-model",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	httpReq, err := p.BuildRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	// Check URL
	if httpReq.URL.String() != "https://api.test.com/v1/chat/completions" {
		t.Errorf("URL = %v, want %v", httpReq.URL.String(), "https://api.test.com/v1/chat/completions")
	}

	// Check headers
	if httpReq.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not set correctly")
	}
	if httpReq.Header.Get("Authorization") != "Bearer test-api-key" {
		t.Error("Authorization header not set correctly")
	}
}

func TestProvider_BuildRequest_CustomHeaders(t *testing.T) {
	p, _ := New(
		provider.ProviderConfig{
			APIKey: "test-api-key",
		},
		ProviderInfo{
			Name:           "test",
			DefaultBaseURL: "https://api.test.com/v1",
			APIKeyHeader:   "X-API-Key",
			APIKeyPrefix:   "",
			ExtraHeaders: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
	)

	req := &types.ChatRequest{
		Model: "test-model",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	httpReq, err := p.BuildRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	// Check custom API key header
	if httpReq.Header.Get("X-API-Key") != "test-api-key" {
		t.Errorf("X-API-Key = %v, want test-api-key", httpReq.Header.Get("X-API-Key"))
	}

	// Check extra headers
	if httpReq.Header.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("X-Custom-Header = %v, want custom-value", httpReq.Header.Get("X-Custom-Header"))
	}
}

func TestProvider_ParseResponse(t *testing.T) {
	p, _ := New(
		provider.ProviderConfig{APIKey: "test"},
		ProviderInfo{Name: "test", DefaultBaseURL: "https://api.test.com/v1"},
	)

	respBody := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "test-model",
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(respBody))
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)
	chatResp, err := p.ParseResponse(resp)
	if err != nil {
		t.Fatalf("ParseResponse() error = %v", err)
	}

	if chatResp.ID != "chatcmpl-123" {
		t.Errorf("ID = %v, want chatcmpl-123", chatResp.ID)
	}
	if len(chatResp.Choices) != 1 {
		t.Errorf("Choices count = %d, want 1", len(chatResp.Choices))
	}
	if chatResp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", chatResp.Usage.TotalTokens)
	}
}

func TestProvider_ParseStreamChunk(t *testing.T) {
	p, _ := New(
		provider.ProviderConfig{APIKey: "test"},
		ProviderInfo{Name: "test", DefaultBaseURL: "https://api.test.com/v1"},
	)

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
			data:    []byte(`data: {"id":"123","choices":[{"delta":{"content":"Hi"}}]}`),
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := p.ParseStreamChunk(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStreamChunk() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (chunk == nil) != tt.wantNil {
				t.Errorf("ParseStreamChunk() chunk = %v, wantNil %v", chunk, tt.wantNil)
			}
		})
	}
}

func TestProvider_MapError(t *testing.T) {
	p, _ := New(
		provider.ProviderConfig{APIKey: "test"},
		ProviderInfo{Name: "test-provider", DefaultBaseURL: "https://api.test.com/v1"},
	)

	tests := []struct {
		name       string
		statusCode int
		body       []byte
	}{
		{"unauthorized", http.StatusUnauthorized, []byte(`{"error":{"message":"Invalid API key"}}`)},
		{"rate limit", http.StatusTooManyRequests, []byte(`{"error":{"message":"Rate limit exceeded"}}`)},
		{"bad request", http.StatusBadRequest, []byte(`{"error":{"message":"Invalid request"}}`)},
		{"not found", http.StatusNotFound, []byte(`{"error":{"message":"Model not found"}}`)},
		{"server error", http.StatusInternalServerError, []byte(`{"error":{"message":"Internal error"}}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.MapError(tt.statusCode, tt.body)
			if err == nil {
				t.Error("MapError() should return an error")
			}
		})
	}
}
