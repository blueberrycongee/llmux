package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestNew(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"claude-3-opus-20240229"},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if p.Name() != ProviderName {
		t.Errorf("Name() = %s, want %s", p.Name(), ProviderName)
	}
}

func TestProvider_SupportsModel(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"claude-3-opus-20240229"},
	}

	p, _ := New(cfg)

	tests := []struct {
		model string
		want  bool
	}{
		{"claude-3-opus-20240229", true},
		{"claude-3-5-sonnet-20241022", true}, // Matches claude- prefix
		{"gpt-4", false},
		{"gemini-pro", false},
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
		BaseURL: "https://api.anthropic.com",
		Models:  []string{"claude-3-opus-20240229"},
	}

	p, _ := New(cfg)

	req := &types.ChatRequest{
		Model: "claude-3-opus-20240229",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
		MaxTokens: 1024,
	}

	httpReq, err := p.BuildRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	// Check URL
	if httpReq.URL.String() != "https://api.anthropic.com/v1/messages" {
		t.Errorf("URL = %s, want https://api.anthropic.com/v1/messages", httpReq.URL.String())
	}

	// Check headers
	if httpReq.Header.Get("x-api-key") != "test-api-key" {
		t.Error("x-api-key header should be set")
	}
	if httpReq.Header.Get("anthropic-version") != DefaultAPIVersion {
		t.Error("anthropic-version header should be set")
	}
}

func TestProvider_TransformMessages(t *testing.T) {
	p := &Provider{}

	t.Run("extracts system message", func(t *testing.T) {
		messages := []types.ChatMessage{
			{Role: "system", Content: json.RawMessage(`"You are helpful"`)},
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		}

		result, system, err := p.transformMessages(messages)
		if err != nil {
			t.Fatalf("transformMessages() error = %v", err)
		}

		if system != "You are helpful" {
			t.Errorf("system = %s, want 'You are helpful'", system)
		}

		if len(result) != 1 {
			t.Errorf("result count = %d, want 1", len(result))
		}
	})

	t.Run("maps tool role to user with tool_result", func(t *testing.T) {
		messages := []types.ChatMessage{
			{Role: "tool", Content: json.RawMessage(`"result data"`), ToolCallID: "call_123"},
		}

		result, _, err := p.transformMessages(messages)
		if err != nil {
			t.Fatalf("transformMessages() error = %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("result count = %d, want 1", len(result))
		}

		if result[0].Role != "user" {
			t.Errorf("role = %s, want user", result[0].Role)
		}
	})
}

func TestProvider_MapStopReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"tool_use", "tool_calls"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapStopReason(tt.input); got != tt.want {
				t.Errorf("mapStopReason(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestProvider_Integration(t *testing.T) {
	// Mock Anthropic server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "Hello from Claude!"},
			},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		})
	}))
	defer server.Close()

	cfg := provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Models:  []string{"claude-3-opus-20240229"},
	}

	p, _ := New(cfg)

	req := &types.ChatRequest{
		Model: "claude-3-opus-20240229",
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

	if chatResp.ID != "msg_test" {
		t.Errorf("ID = %s, want msg_test", chatResp.ID)
	}

	if chatResp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", chatResp.Usage.TotalTokens)
	}
}
