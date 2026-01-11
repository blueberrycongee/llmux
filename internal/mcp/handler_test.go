package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestHTTPHandlerRegisterRoutes(t *testing.T) {
	manager := NewMockManager()
	manager.AddMockClient("client-1", "Client 1", ConnectionTypeHTTP, []types.Tool{
		{
			Type: "function",
			Function: types.ToolFunction{
				Name: "tool_one",
			},
		},
		{
			Type: "function",
			Function: types.ToolFunction{
				Name: "tool_two",
			},
		},
	})

	handler := NewHTTPHandler(manager)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/mcp/tools", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != 200 {
		t.Fatalf("status code = %d, want %d", recorder.Code, 200)
	}

	var payload struct {
		Tools []string `json:"tools"`
		Count int      `json:"count"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Count != 2 {
		t.Fatalf("count = %d, want %d", payload.Count, 2)
	}

	toolSet := make(map[string]bool)
	for _, name := range payload.Tools {
		toolSet[name] = true
	}

	if !toolSet["tool_one"] || !toolSet["tool_two"] {
		t.Fatalf("tools = %v, want tool_one and tool_two", payload.Tools)
	}
}
