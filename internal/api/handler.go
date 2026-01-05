// Package api provides HTTP handlers for the LLM gateway API.
// It implements OpenAI-compatible endpoints for chat completions.
package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/router"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// Handler handles HTTP requests for the LLM gateway.
type Handler struct {
	registry *provider.Registry
	router   router.Router
	logger   *slog.Logger
}

// NewHandler creates a new API handler.
func NewHandler(registry *provider.Registry, router router.Router, logger *slog.Logger) *Handler {
	return &Handler{
		registry: registry,
		router:   router,
		logger:   logger,
	}
}

// ChatCompletions handles POST /v1/chat/completions requests.
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "failed to read request body"))
		return
	}
	defer r.Body.Close()

	var req types.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "invalid JSON: "+err.Error()))
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

	// Route to deployment
	deployment, err := h.router.Pick(r.Context(), req.Model)
	if err != nil {
		h.logger.Error("no deployment available", "model", req.Model, "error", err)
		h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, "no available deployment"))
		return
	}

	// Get provider
	prov, ok := h.registry.GetProvider(deployment.ProviderName)
	if !ok {
		h.writeError(w, llmerrors.NewInternalError(deployment.ProviderName, req.Model, "provider not found"))
		return
	}

	// Build upstream request
	upstreamReq, err := prov.BuildRequest(r.Context(), &req)
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), req.Model, "failed to build request: "+err.Error()))
		return
	}

	// Execute request
	client := &http.Client{Timeout: time.Duration(deployment.Timeout) * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		h.router.ReportFailure(deployment, err)
		metrics.RecordError(prov.Name(), "connection_error")
		h.writeError(w, llmerrors.NewServiceUnavailableError(prov.Name(), req.Model, "upstream request failed"))
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)

	// Handle error responses
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		llmErr := prov.MapError(resp.StatusCode, respBody)
		h.router.ReportFailure(deployment, llmErr)
		metrics.RecordRequest(prov.Name(), req.Model, resp.StatusCode, latency)
		h.writeError(w, llmErr)
		return
	}

	// Handle streaming response
	if req.Stream {
		h.handleStreamResponse(w, resp, prov, deployment, req.Model, start)
		return
	}

	// Parse non-streaming response
	chatResp, err := prov.ParseResponse(resp)
	if err != nil {
		h.router.ReportFailure(deployment, err)
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), req.Model, "failed to parse response"))
		return
	}

	// Record success metrics
	h.router.ReportSuccess(deployment, latency)
	metrics.RecordRequest(prov.Name(), req.Model, http.StatusOK, latency)
	if chatResp.Usage != nil {
		metrics.RecordTokens(prov.Name(), req.Model, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens)
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResp)
}

func (h *Handler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, prov provider.Provider, deployment *provider.Deployment, model string, start time.Time) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), model, "streaming not supported"))
		return
	}

	// Forward SSE stream
	// TODO: Implement proper SSE forwarding with buffer pooling
	_, err := io.Copy(w, resp.Body)
	if err != nil {
		h.logger.Error("stream copy error", "error", err)
	}
	flusher.Flush()

	// Record metrics
	latency := time.Since(start)
	h.router.ReportSuccess(deployment, latency)
	metrics.RecordRequest(prov.Name(), model, http.StatusOK, latency)
}

// ErrorResponse represents an OpenAI-compatible error response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
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
	json.NewEncoder(w).Encode(resp)
}

// HealthCheck handles GET /health/live and /health/ready endpoints.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ListModels handles GET /v1/models endpoint.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement model listing from all providers
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   []any{},
	})
}
