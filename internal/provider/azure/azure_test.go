package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestNew(t *testing.T) {
	t.Run("requires base_url", func(t *testing.T) {
		cfg := provider.ProviderConfig{
			APIKey: "test-key",
			Models: []string{"gpt-4"},
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("New() should require base_url")
		}
	})

	t.Run("creates provider with base_url", func(t *testing.T) {
		cfg := provider.ProviderConfig{
			APIKey:  "test-key",
			BaseURL: "https://my-resource.openai.azure.com",
			Models:  []string{"gpt-4"},
		}

		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if p.Name() != ProviderName {
			t.Errorf("Name() = %s, want %s", p.Name(), ProviderName)
		}
	})
}

func TestProvider_SupportsModel(t *testing.T) {
	cfg := provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://my-resource.openai.azure.com",
		Models:  []string{"gpt-4", "gpt-35-turbo"},
	}

	p, _ := New(cfg)

	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4", true},
		{"gpt-35-turbo", true},
		{"gpt-4o", false}, // Not in configured models
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
		BaseURL: "https://my-resource.openai.azure.com",
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

	// Check URL format
	url := httpReq.URL.String()
	if !strings.Contains(url, "/openai/deployments/gpt-4/chat/completions") {
		t.Errorf("URL should contain deployment path, got %s", url)
	}
	if !strings.Contains(url, "api-version=") {
		t.Errorf("URL should contain api-version, got %s", url)
	}

	// Check headers - Azure uses api-key instead of Authorization
	if httpReq.Header.Get("api-key") != "test-api-key" {
		t.Error("api-key header should be set")
	}
}

func TestProvider_Integration(t *testing.T) {
	// Mock Azure OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL path
		if !strings.Contains(r.URL.Path, "/openai/deployments/") {
			t.Errorf("URL path should contain /openai/deployments/, got %s", r.URL.Path)
		}

		// Verify api-key header
		if r.Header.Get("api-key") != "test-key" {
			t.Error("missing api-key header")
		}

		// Return OpenAI-compatible response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-azure-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from Azure!",
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

	if chatResp.ID != "chatcmpl-azure-test" {
		t.Errorf("ID = %s, want chatcmpl-azure-test", chatResp.ID)
	}

	if chatResp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", chatResp.Usage.TotalTokens)
	}
}
