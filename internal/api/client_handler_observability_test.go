package api //nolint:revive // package name is intentional

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/observability"
)

type recordingCallback struct {
	mu            sync.Mutex
	preCount      int
	postCount     int
	successCount  int
	failureCount  int
	successEvents []*observability.StandardLoggingPayload
	failSuccess   bool
}

func (r *recordingCallback) Name() string { return "recording" }

func (r *recordingCallback) LogPreAPICall(ctx context.Context, payload *observability.StandardLoggingPayload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preCount++
	return nil
}

func (r *recordingCallback) LogPostAPICall(ctx context.Context, payload *observability.StandardLoggingPayload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.postCount++
	return nil
}

func (r *recordingCallback) LogStreamEvent(ctx context.Context, payload *observability.StandardLoggingPayload, chunk any) error {
	return nil
}

func (r *recordingCallback) LogSuccessEvent(ctx context.Context, payload *observability.StandardLoggingPayload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.successCount++
	r.successEvents = append(r.successEvents, payload)
	if r.failSuccess {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func (r *recordingCallback) LogFailureEvent(ctx context.Context, payload *observability.StandardLoggingPayload, err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failureCount++
	return nil
}

func (r *recordingCallback) LogFallbackEvent(ctx context.Context, originalModel string, fallbackModel string, err error, success bool) error {
	return nil
}

func (r *recordingCallback) Shutdown(ctx context.Context) error { return nil }

func newMockOpenAIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Model string `json:"model"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		_ = json.Unmarshal(body, &req)
		model := req.Model
		if model == "" {
			model = "gpt-4o"
		}
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
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

func TestClientHandler_ObservabilityRequestID(t *testing.T) {
	mock := newMockOpenAIServer()
	defer mock.Close()

	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:                "openai",
			Type:                "openai",
			APIKey:              "test",
			BaseURL:             mock.URL,
			AllowPrivateBaseURL: true,
			Models:              []string{"gpt-4o"},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	obsMgr, err := observability.NewObservabilityManager(observability.ObservabilityConfig{})
	require.NoError(t, err)

	cb := &recordingCallback{}
	obsMgr.CallbackManager().Register(cb)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	handler := NewClientHandler(client, logger, &ClientHandlerConfig{
		Observability: obsMgr,
	})

	reqBody, err := json.Marshal(llmux.ChatRequest{
		Model: "gpt-4o",
		Messages: []llmux.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req = req.WithContext(observability.ContextWithRequestID(req.Context(), "req-123"))
	rec := httptest.NewRecorder()

	handler.ChatCompletions(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	cb.mu.Lock()
	defer cb.mu.Unlock()
	require.Equal(t, 1, cb.successCount)
	require.Equal(t, "req-123", cb.successEvents[0].RequestID)
}

func TestClientHandler_ObservabilityCallbackFailureDoesNotBlock(t *testing.T) {
	mock := newMockOpenAIServer()
	defer mock.Close()

	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:                "openai",
			Type:                "openai",
			APIKey:              "test",
			BaseURL:             mock.URL,
			AllowPrivateBaseURL: true,
			Models:              []string{"gpt-4o"},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	obsMgr, err := observability.NewObservabilityManager(observability.ObservabilityConfig{})
	require.NoError(t, err)

	cb := &recordingCallback{failSuccess: true}
	obsMgr.CallbackManager().Register(cb)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	handler := NewClientHandler(client, logger, &ClientHandlerConfig{
		Observability: obsMgr,
	})

	reqBody, err := json.Marshal(llmux.ChatRequest{
		Model: "gpt-4o",
		Messages: []llmux.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req = req.WithContext(observability.ContextWithRequestID(req.Context(), "req-456"))
	rec := httptest.NewRecorder()

	handler.ChatCompletions(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
