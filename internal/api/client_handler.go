// Package api provides HTTP handlers for the LLM gateway API.
// This file contains the ClientHandler which wraps llmux.Client for Gateway mode.
package api

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/pool"
	"github.com/blueberrycongee/llmux/internal/streaming"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// ClientHandler handles HTTP requests using llmux.Client.
// This is the recommended handler for Gateway mode as it uses the same
// core logic as Library mode.
type ClientHandler struct {
	client      *llmux.Client
	logger      *slog.Logger
	maxBodySize int64
}

// ClientHandlerConfig contains configuration for ClientHandler.
type ClientHandlerConfig struct {
	MaxBodySize int64 // Maximum request body size in bytes
}

// NewClientHandler creates a new handler that wraps llmux.Client.
func NewClientHandler(client *llmux.Client, logger *slog.Logger, cfg *ClientHandlerConfig) *ClientHandler {
	maxBodySize := int64(DefaultMaxBodySize)
	if cfg != nil && cfg.MaxBodySize > 0 {
		maxBodySize = cfg.MaxBodySize
	}

	return &ClientHandler{
		client:      client,
		logger:      logger,
		maxBodySize: maxBodySize,
	}
}

// ChatCompletions handles POST /v1/chat/completions requests.
func (h *ClientHandler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Limit request body size to prevent OOM
	limitedReader := io.LimitReader(r.Body, h.maxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "failed to read request body"))
		return
	}
	defer r.Body.Close()

	// Check if body exceeded limit
	if int64(len(body)) > h.maxBodySize {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "request body too large"))
		return
	}

	req := pool.GetChatRequest()
	defer pool.PutChatRequest(req)

	if unmarshalErr := json.Unmarshal(body, req); unmarshalErr != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "invalid JSON: "+unmarshalErr.Error()))
		return
	}

	// Validate request
	if req.Model == "" {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "model is required"))
		return
	}
	if len(req.Messages) == 0 {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, "messages is required"))
		return
	}

	// Handle streaming response
	if req.Stream {
		h.handleStreamResponse(w, r, req, start)
		return
	}

	// Non-streaming request - use Client.ChatCompletion
	resp, err := h.client.ChatCompletion(r.Context(), req)
	if err != nil {
		h.logger.Error("chat completion failed", "model", req.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, err.Error()))
		}
		return
	}

	latency := time.Since(start)

	// Record metrics
	metrics.RecordRequest("llmux", req.Model, http.StatusOK, latency)
	if resp.Usage != nil {
		metrics.RecordTokens("llmux", req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *ClientHandler) handleStreamResponse(w http.ResponseWriter, r *http.Request, req *llmux.ChatRequest, start time.Time) {
	stream, err := h.client.ChatCompletionStream(r.Context(), req)
	if err != nil {
		h.logger.Error("stream creation failed", "model", req.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, err.Error()))
		}
		return
	}
	defer stream.Close()

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, llmerrors.NewInternalError("", req.Model, "streaming not supported"))
		return
	}

	// Forward stream chunks
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// Send [DONE] marker
			if _, writeErr := w.Write([]byte("data: [DONE]\n\n")); writeErr != nil {
				h.logger.Debug("failed to write done marker", "error", writeErr)
			}
			flusher.Flush()
			break
		}
		if err != nil {
			// Client disconnect is not an error worth logging at error level
			if r.Context().Err() != nil {
				h.logger.Debug("client disconnected during stream", "model", req.Model)
			} else {
				h.logger.Error("stream recv error", "error", err, "model", req.Model)
			}
			break
		}

		// Marshal and send chunk
		data, marshalErr := json.Marshal(chunk)
		if marshalErr != nil {
			h.logger.Error("failed to marshal chunk", "error", marshalErr)
			continue
		}

		if _, writeErr := w.Write([]byte("data: ")); writeErr != nil {
			break
		}
		if _, writeErr := w.Write(data); writeErr != nil {
			break
		}
		if _, writeErr := w.Write([]byte("\n\n")); writeErr != nil {
			break
		}
		flusher.Flush()
	}

	// Record metrics
	latency := time.Since(start)
	metrics.RecordRequest("llmux", req.Model, http.StatusOK, latency)
}

func (h *ClientHandler) writeError(w http.ResponseWriter, err error) {
	var llmErr *llmerrors.LLMError
	if e, ok := err.(*llmerrors.LLMError); ok {
		llmErr = e
	} else {
		llmErr = llmerrors.NewInternalError("", "", err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(llmErr.HTTPStatusCode())

	resp := ErrorResponse{
		Error: ErrorDetail{
			Message: llmErr.Message,
			Type:    llmErr.Type,
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode error response", "error", err)
	}
}

// HealthCheck handles GET /health/live and /health/ready endpoints.
func (h *ClientHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		h.logger.Error("failed to encode health response", "error", err)
	}
}

// ListModels handles GET /v1/models endpoint.
func (h *ClientHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.client.ListModels(r.Context())
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError("", "", "failed to list models: "+err.Error()))
		return
	}

	// Convert to OpenAI format
	data := make([]map[string]any, 0, len(models))
	for _, m := range models {
		data = append(data, map[string]any{
			"id":       m.ID,
			"object":   m.Object,
			"owned_by": m.Provider,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   data,
	}); err != nil {
		h.logger.Error("failed to encode models response", "error", err)
	}
}

// GetClient returns the underlying llmux.Client.
// This is useful for accessing client methods directly.
func (h *ClientHandler) GetClient() *llmux.Client {
	return h.client
}

// Ensure streaming package is imported for parser registration
var _ = streaming.GetParser
