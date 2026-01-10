// Package api provides HTTP handlers for the LLM gateway API.
// It implements OpenAI-compatible endpoints for chat completions.
package api //nolint:revive // package name is intentional

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/pool"
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/streaming"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	pkgprovider "github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	// DefaultMaxBodySize is the default maximum request body size (10MB).
	// This accommodates large context windows while preventing abuse.
	DefaultMaxBodySize = 10 * 1024 * 1024
)

// HandlerConfig contains configuration for the API handler.
type HandlerConfig struct {
	MaxBodySize int64 // Maximum request body size in bytes
}

// Handler handles HTTP requests for the LLM gateway.
type Handler struct {
	registry    *provider.Registry
	llmRouter   router.Router
	logger      *slog.Logger
	httpClient  *http.Client
	maxBodySize int64
}

// NewHandler creates a new API handler with a shared HTTP client.
func NewHandler(registry *provider.Registry, r router.Router, logger *slog.Logger, cfg *HandlerConfig) *Handler {
	maxBodySize := int64(DefaultMaxBodySize)
	if cfg != nil && cfg.MaxBodySize > 0 {
		maxBodySize = cfg.MaxBodySize
	}

	// Create shared HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Handler{
		registry:    registry,
		llmRouter:   r,
		logger:      logger,
		maxBodySize: maxBodySize,
		httpClient: &http.Client{
			Transport: transport,
			// Timeout is set per-request based on deployment config
		},
	}
}

// ChatCompletions handles POST /v1/chat/completions requests.
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Limit request body size to prevent OOM
	limitedReader := io.LimitReader(r.Body, h.maxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "failed to read request body"))
		return
	}
	defer func() { _ = r.Body.Close() }()

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

	// Route to deployment
	deployment, err := h.llmRouter.Pick(r.Context(), req.Model)
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

	// Build upstream request with timeout context
	timeout := time.Duration(deployment.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	reqCtx, reqCancel := context.WithTimeout(r.Context(), timeout)
	defer reqCancel()

	upstreamReq, err := prov.BuildRequest(reqCtx, req)
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), req.Model, "failed to build request: "+err.Error()))
		return
	}

	// Execute request using shared client
	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		h.llmRouter.ReportFailure(deployment, err)
		metrics.RecordError(prov.Name(), "connection_error")
		h.writeError(w, llmerrors.NewServiceUnavailableError(prov.Name(), req.Model, "upstream request failed"))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Apply response transformer if present
	if transformer, ok := upstreamReq.Context().Value(pkgprovider.ResponseTransformerKey).(pkgprovider.ResponseTransformer); ok {
		resp.Body = transformer(resp.Body)
	}

	latency := time.Since(start)

	// Handle error responses
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		llmErr := prov.MapError(resp.StatusCode, respBody)
		h.llmRouter.ReportFailure(deployment, llmErr)
		metrics.RecordRequest(prov.Name(), req.Model, resp.StatusCode, latency)
		h.writeError(w, llmErr)
		return
	}

	// Handle streaming response
	if req.Stream {
		h.handleStreamResponse(w, r, resp, prov, deployment, req.Model, start)
		return
	}

	// Parse non-streaming response
	chatResp, err := prov.ParseResponse(resp)
	if err != nil {
		h.llmRouter.ReportFailure(deployment, err)
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), req.Model, "failed to parse response"))
		return
	}
	defer pool.PutChatResponse(chatResp)

	// Record success metrics
	h.llmRouter.ReportSuccess(deployment, &router.ResponseMetrics{Latency: latency})
	metrics.RecordRequest(prov.Name(), req.Model, http.StatusOK, latency)
	if chatResp.Usage != nil {
		metrics.RecordTokens(prov.Name(), req.Model, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens)
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chatResp)
}

func (h *Handler) handleStreamResponse(w http.ResponseWriter, r *http.Request, resp *http.Response, prov provider.Provider, deployment *pkgprovider.Deployment, model string, start time.Time) {
	// Get provider-specific parser for chunk transformation
	parser := streaming.GetParser(prov.Name())

	// Create SSE forwarder with client context for disconnect detection
	forwarder, err := streaming.NewForwarder(streaming.ForwarderConfig{
		Upstream:   resp.Body,
		Downstream: w,
		Parser:     parser,
		ClientCtx:  r.Context(),
	})
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), model, "streaming not supported"))
		return
	}
	defer forwarder.Close()

	// Forward the stream - blocks until complete or client disconnects
	if err := forwarder.Forward(); err != nil {
		// Client disconnect is not an error worth logging at error level
		if r.Context().Err() != nil {
			h.logger.Debug("client disconnected during stream", "model", model)
		} else {
			h.logger.Error("stream forward error", "error", err, "model", model)
		}
	}

	// Record metrics
	latency := time.Since(start)
	h.llmRouter.ReportSuccess(deployment, &router.ResponseMetrics{Latency: latency})
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
	_ = json.NewEncoder(w).Encode(resp)
}

// HealthCheck handles GET /health/live and /health/ready endpoints.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ListModels handles GET /v1/models endpoint.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement model listing from all providers
	// TODO: [Real Data Fetching] - Retrieve available models from ProviderRegistry and potentially filter by user access.
	// Currently returning hardcoded empty list.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   []any{},
	})
}

// Embeddings handles POST /v1/embeddings requests.
func (h *Handler) Embeddings(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Limit request body size to prevent OOM
	limitedReader := io.LimitReader(r.Body, h.maxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "failed to read request body"))
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Check if body exceeded limit
	if int64(len(body)) > h.maxBodySize {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "request body too large"))
		return
	}

	var req types.EmbeddingRequest
	if unmarshalErr := json.Unmarshal(body, &req); unmarshalErr != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "invalid JSON: "+unmarshalErr.Error()))
		return
	}

	// Validate request
	if req.Model == "" {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "model is required"))
		return
	}
	if req.Input == nil || req.Input.IsEmpty() {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, "input is required"))
		return
	}
	if validateErr := req.Input.Validate(); validateErr != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, validateErr.Error()))
		return
	}

	// Route to deployment
	deployment, err := h.llmRouter.Pick(r.Context(), req.Model)
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

	// Check if provider supports embedding
	if !prov.SupportEmbedding() {
		h.writeError(w, llmerrors.NewInvalidRequestError(prov.Name(), req.Model, "provider does not support embeddings"))
		return
	}

	// Build upstream request with timeout context
	timeout := time.Duration(deployment.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	reqCtx, reqCancel := context.WithTimeout(r.Context(), timeout)
	defer reqCancel()

	upstreamReq, err := prov.BuildEmbeddingRequest(reqCtx, &req)
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), req.Model, "failed to build request: "+err.Error()))
		return
	}

	// Execute request using shared client
	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		h.llmRouter.ReportFailure(deployment, err)
		metrics.RecordError(prov.Name(), "connection_error")
		h.writeError(w, llmerrors.NewServiceUnavailableError(prov.Name(), req.Model, "upstream request failed"))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(start)

	// Handle error responses
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		llmErr := prov.MapError(resp.StatusCode, respBody)
		h.llmRouter.ReportFailure(deployment, llmErr)
		metrics.RecordRequest(prov.Name(), req.Model, resp.StatusCode, latency)
		h.writeError(w, llmErr)
		return
	}

	// Parse embedding response
	embResp, err := prov.ParseEmbeddingResponse(resp)
	if err != nil {
		h.llmRouter.ReportFailure(deployment, err)
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), req.Model, "failed to parse response"))
		return
	}

	// Record success metrics
	h.llmRouter.ReportSuccess(deployment, &router.ResponseMetrics{Latency: latency})
	metrics.RecordRequest(prov.Name(), req.Model, http.StatusOK, latency)
	if embResp.Usage.TotalTokens > 0 {
		metrics.RecordTokens(prov.Name(), req.Model, embResp.Usage.PromptTokens, 0)
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(embResp)
}
