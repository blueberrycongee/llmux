package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestFetchHandlerExtractsReadableHTML(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>ignore</title><script>bad()</script></head><body><main><h1>Hello</h1><p>Readable text</p></main></body></html>`))
	}))
	defer srv.Close()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url": srv.URL,
			},
		},
	}

	result, err := fetchHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("fetchHandler() error = %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("fetchHandler() returned error result: %+v", result)
	}

	text := resultText(result)
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "Readable text") {
		t.Fatalf("fetchHandler() text = %q, want extracted body text", text)
	}
	if strings.Contains(text, "bad()") {
		t.Fatalf("fetchHandler() text should not contain script content: %q", text)
	}
}

func TestFetchHandlerMarksHTTPFailuresAsToolErrors(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer srv.Close()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url": srv.URL,
			},
		},
	}

	result, err := fetchHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("fetchHandler() error = %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("fetchHandler() result = %+v, want tool error", result)
	}
	if !strings.Contains(resultText(result), "404") {
		t.Fatalf("fetchHandler() text = %q, want status code", resultText(result))
	}
}

func resultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	text, _ := result.Content[0].(mcp.TextContent)
	return text.Text
}
