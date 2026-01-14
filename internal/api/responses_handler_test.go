package api //nolint:revive // package name is intentional

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestResponsesHandler_NonStreaming(t *testing.T) {
	mock := newResponsesMockServer()
	defer mock.Close()

	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:    "openai",
			Type:    "openai",
			APIKey:  "test",
			BaseURL: mock.URL,
			Models:  []string{"gpt-4o"},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	handler := NewClientHandler(client, logger, nil)

	reqBody, err := json.Marshal(map[string]any{
		"model": "gpt-4o",
		"input": "hello",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	handler.Responses(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload types.ResponseResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "response", payload.Object)
	require.Len(t, payload.Output, 1)
	require.Len(t, payload.Output[0].Content, 1)
	require.Equal(t, "ok", payload.Output[0].Content[0].Text)
}

func TestResponsesHandler_Streaming(t *testing.T) {
	mock := newResponsesMockServer()
	defer mock.Close()

	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:    "openai",
			Type:    "openai",
			APIKey:  "test",
			BaseURL: mock.URL,
			Models:  []string{"gpt-4o"},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	handler := NewClientHandler(client, logger, nil)

	reqBody, err := json.Marshal(map[string]any{
		"model":  "gpt-4o",
		"input":  "hello",
		"stream": true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	handler.Responses(rec, req)

	body := rec.Body.String()
	require.Contains(t, body, "response.output_text.delta")
	require.Contains(t, body, "response.completed")
	require.Contains(t, body, "[DONE]")
}

func TestResponsesHandler_Streaming_DoesNotForceIncludeUsage(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if bytes.Contains(body, []byte(`"include_usage":true`)) {
			http.Error(w, "include_usage was forced", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "data: {}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mock.Close()

	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:    "openai",
			Type:    "openai",
			APIKey:  "test",
			BaseURL: mock.URL,
			Models:  []string{"gpt-4o"},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	handler := NewClientHandler(client, logger, nil)

	reqBody, err := json.Marshal(map[string]any{
		"model":          "gpt-4o",
		"input":          "hello",
		"stream":         true,
		"stream_options": map[string]any{"include_usage": false},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	handler.Responses(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAudioAndBatchEndpoints_NotSupported(t *testing.T) {
	mock := newResponsesMockServer()
	defer mock.Close()

	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:    "openai",
			Type:    "openai",
			APIKey:  "test",
			BaseURL: mock.URL,
			Models:  []string{"gpt-4o"},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	handler := NewClientHandler(client, logger, nil)

	tests := []struct {
		path   string
		handle func(http.ResponseWriter, *http.Request)
	}{
		{path: "/v1/audio/transcriptions", handle: handler.AudioTranscriptions},
		{path: "/v1/audio/translations", handle: handler.AudioTranslations},
		{path: "/v1/audio/speech", handle: handler.AudioSpeech},
		{path: "/v1/batches", handle: handler.Batches},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("payload"))
		rec := httptest.NewRecorder()
		tc.handle(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var payload ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Equal(t, "invalid_request_error", payload.Error.Type)
	}
}

func newResponsesMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		_ = json.Unmarshal(body, &req)
		model := req.Model
		if model == "" {
			model = "gpt-4o"
		}

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}
			chunk := map[string]any{
				"id":      "chatcmpl-stream",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   model,
				"choices": []map[string]any{
					{
						"index": 0,
						"delta": map[string]any{
							"role":    "assistant",
							"content": "streamed",
						},
					},
				},
			}
			jsonData, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "ok",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
