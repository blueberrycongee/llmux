package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestNew(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey: "test-key",
		Models: []string{"gemini-1.5-pro"},
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
		Models: []string{"gemini-1.5-pro"},
	}

	p, _ := New(cfg)

	tests := []struct {
		model string
		want  bool
	}{
		{"gemini-1.5-pro", true},
		{"gemini-1.5-flash", true}, // Matches gemini- prefix
		{"gpt-4", false},
		{"claude-3", false},
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
		BaseURL: "https://generativelanguage.googleapis.com",
		Models:  []string{"gemini-1.5-pro"},
	}

	p, _ := New(cfg)

	req := &types.ChatRequest{
		Model: "gemini-1.5-pro",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	httpReq, err := p.BuildRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	// Check URL contains model and API key
	url := httpReq.URL.String()
	if !strings.Contains(url, "gemini-1.5-pro") {
		t.Errorf("URL should contain model name, got %s", url)
	}
	if !strings.Contains(url, "key=test-api-key") {
		t.Errorf("URL should contain API key, got %s", url)
	}
	if !strings.Contains(url, "generateContent") {
		t.Errorf("URL should contain generateContent, got %s", url)
	}
}

func TestProvider_TransformMessages(t *testing.T) {
	p := &Provider{}

	t.Run("extracts system instruction", func(t *testing.T) {
		messages := []types.ChatMessage{
			{Role: "system", Content: json.RawMessage(`"You are helpful"`)},
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		}

		contents, sysInstr, err := p.transformMessages(messages)
		if err != nil {
			t.Fatalf("transformMessages() error = %v", err)
		}

		if sysInstr == nil {
			t.Fatal("systemInstruction should not be nil")
		}

		if len(contents) != 1 {
			t.Errorf("contents count = %d, want 1", len(contents))
		}
	})

	t.Run("maps assistant to model role", func(t *testing.T) {
		messages := []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
			{Role: "assistant", Content: json.RawMessage(`"Hi there"`)},
		}

		contents, _, err := p.transformMessages(messages)
		if err != nil {
			t.Fatalf("transformMessages() error = %v", err)
		}

		if len(contents) != 2 {
			t.Fatalf("contents count = %d, want 2", len(contents))
		}

		if contents[1].Role != "model" {
			t.Errorf("assistant role should be mapped to 'model', got %s", contents[1].Role)
		}
	})
}

func TestProvider_MapFinishReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
		{"OTHER", "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapFinishReason(tt.input); got != tt.want {
				t.Errorf("mapFinishReason(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestProvider_Integration(t *testing.T) {
	// Mock Gemini server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL contains API key
		if !strings.Contains(r.URL.String(), "key=test-key") {
			t.Error("URL should contain API key")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "Hello from Gemini!"},
						},
						"role": "model",
					},
					"finishReason": "STOP",
					"index":        0,
				},
			},
			"usageMetadata": map[string]any{
				"promptTokenCount":     10,
				"candidatesTokenCount": 5,
				"totalTokenCount":      15,
			},
		})
	}))
	defer server.Close()

	cfg := provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Models:  []string{"gemini-1.5-pro"},
	}

	p, _ := New(cfg)

	req := &types.ChatRequest{
		Model: "gemini-1.5-pro",
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

	if len(chatResp.Choices) != 1 {
		t.Fatalf("Choices count = %d, want 1", len(chatResp.Choices))
	}

	if chatResp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %s, want stop", chatResp.Choices[0].FinishReason)
	}

	if chatResp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", chatResp.Usage.TotalTokens)
	}
}
