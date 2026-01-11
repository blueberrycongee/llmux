// Package api provides HTTP handlers for the LLM gateway API.
// This file contains the ClientHandler which wraps llmux.Client for Gateway mode.
package api //nolint:revive // package name is intentional

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/mcp"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/pool"
	"github.com/blueberrycongee/llmux/internal/streaming"
	"github.com/blueberrycongee/llmux/internal/tokenizer"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// ClientHandler handles HTTP requests using llmux.Client.
// This is the recommended handler for Gateway mode as it uses the same
// core logic as Library mode.
type ClientHandler struct {
	swapper     *ClientSwapper
	logger      *slog.Logger
	maxBodySize int64
	store       auth.Store // Storage for usage logging and budget tracking
	mcpManager  mcp.Manager
}

// ClientHandlerConfig contains configuration for ClientHandler.
type ClientHandlerConfig struct {
	MaxBodySize int64      // Maximum request body size in bytes
	Store       auth.Store // Storage for usage logging (optional)
	MCPManager  mcp.Manager
}

// NewClientHandler creates a new handler that wraps llmux.Client.
func NewClientHandler(client *llmux.Client, logger *slog.Logger, cfg *ClientHandlerConfig) *ClientHandler {
	return NewClientHandlerWithSwapper(NewClientSwapper(client), logger, cfg)
}

// NewClientHandlerWithSwapper creates a new handler that uses a hot-swappable llmux.Client.
func NewClientHandlerWithSwapper(swapper *ClientSwapper, logger *slog.Logger, cfg *ClientHandlerConfig) *ClientHandler {
	maxBodySize := int64(DefaultMaxBodySize)
	var store auth.Store
	var manager mcp.Manager
	if cfg != nil {
		if cfg.MaxBodySize > 0 {
			maxBodySize = cfg.MaxBodySize
		}
		store = cfg.Store
		manager = cfg.MCPManager
	}

	return &ClientHandler{
		swapper:     swapper,
		logger:      logger,
		maxBodySize: maxBodySize,
		store:       store,
		mcpManager:  manager,
	}
}

func (h *ClientHandler) acquireClient() (*llmux.Client, func()) {
	if h == nil || h.swapper == nil {
		return nil, func() {}
	}
	return h.swapper.Acquire()
}

func (h *ClientHandler) getMCPManager(ctx context.Context) mcp.Manager {
	if h.mcpManager != nil {
		return h.mcpManager
	}
	return mcp.GetManager(ctx)
}

// ChatCompletions handles POST /v1/chat/completions requests.
func (h *ClientHandler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := uuid.New().String()

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

	manager := h.getMCPManager(r.Context())

	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, llmerrors.NewInternalError("", req.Model, "client not initialized"))
		return
	}

	// Handle streaming response
	if req.Stream {
		if manager != nil {
			if injector, ok := manager.(mcp.ToolInjector); ok {
				injector.InjectTools(r.Context(), req)
			}
		}

		// Force include_usage to get accurate token counts from supported providers (e.g. OpenAI)
		if req.StreamOptions == nil {
			req.StreamOptions = &llmux.StreamOptions{IncludeUsage: true}
		} else {
			req.StreamOptions.IncludeUsage = true
		}

		h.handleStreamResponse(w, r, client, req, start, requestID)
		return
	}

	// Non-streaming request - use Client.ChatCompletion
	var resp *llmux.ChatResponse
	if manager != nil {
		executor := mcp.NewAgentExecutor(manager, 0, h.logger)
		resp, err = executor.Execute(r.Context(), req, func(ctx context.Context, r *llmux.ChatRequest) (*llmux.ChatResponse, error) {
			return client.ChatCompletion(ctx, r)
		})
	} else {
		resp, err = client.ChatCompletion(r.Context(), req)
	}
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

	if resp.Usage == nil || resp.Usage.TotalTokens == 0 {
		promptTokens := tokenizer.EstimatePromptTokens(req.Model, req)
		completionTokens := tokenizer.EstimateCompletionTokens(req.Model, resp, "")
		resp.Usage = &llmux.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	// Record metrics
	metrics.RecordRequest("llmux", req.Model, http.StatusOK, latency)
	if resp.Usage != nil {
		metrics.RecordTokens("llmux", req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}

	// === USAGE LOGGING AND SPEND TRACKING (P0 Fix) ===
	// Record usage and update spent budget for budget control
	h.recordUsage(r.Context(), requestID, req.Model, "chat_completion", resp.Usage, start, latency)

	// Write response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *ClientHandler) handleStreamResponse(w http.ResponseWriter, r *http.Request, client *llmux.Client, req *llmux.ChatRequest, start time.Time, requestID string) {
	stream, err := client.ChatCompletionStream(r.Context(), req)
	if err != nil {
		h.logger.Error("stream creation failed", "model", req.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, err.Error()))
		}
		return
	}
	defer func() { _ = stream.Close() }()

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

	var finalUsage *llmux.Usage
	var completionContent strings.Builder

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

		// Capture usage if present (OpenAI standard puts it in the last chunk)
		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}

		// Accumulate content for fallback token calculation
		if len(chunk.Choices) > 0 {
			completionContent.WriteString(chunk.Choices[0].Delta.Content)
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

	// Calculate fallback usage if not returned by provider
	if finalUsage == nil {
		promptTokens := tokenizer.EstimatePromptTokens(req.Model, req)
		completionTokens := tokenizer.EstimateCompletionTokensFromText(req.Model, completionContent.String())
		finalUsage = &llmux.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	// Record usage and update spent budget
	h.recordUsage(r.Context(), requestID, req.Model, "chat_completion", finalUsage, start, latency)
}

// Completions handles POST /v1/completions requests.
func (h *ClientHandler) Completions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := uuid.New().String()

	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, llmerrors.NewInternalError("", "", "client not initialized"))
		return
	}

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

	var req types.CompletionRequest
	if unmarshalErr := json.Unmarshal(body, &req); unmarshalErr != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "invalid JSON: "+unmarshalErr.Error()))
		return
	}

	if validateErr := req.Validate(); validateErr != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, validateErr.Error()))
		return
	}

	chatReq, err := req.ToChatRequest()
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, err.Error()))
		return
	}

	// Handle streaming response
	if chatReq.Stream {
		// Force include_usage to get accurate token counts from supported providers (e.g. OpenAI)
		if chatReq.StreamOptions == nil {
			chatReq.StreamOptions = &llmux.StreamOptions{IncludeUsage: true}
		} else {
			chatReq.StreamOptions.IncludeUsage = true
		}

		h.handleCompletionStreamResponse(w, r, client, chatReq, start, requestID)
		return
	}

	resp, err := client.ChatCompletion(r.Context(), chatReq)
	if err != nil {
		h.logger.Error("completion failed", "model", req.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, err.Error()))
		}
		return
	}

	latency := time.Since(start)
	completionResp := types.CompletionResponseFromChat(resp)

	if completionResp.Usage == nil || completionResp.Usage.TotalTokens == 0 {
		promptTokens := tokenizer.EstimatePromptTokens(chatReq.Model, chatReq)
		completionTokens := tokenizer.EstimateCompletionTokensFromText(chatReq.Model, firstCompletionText(completionResp))
		completionResp.Usage = &llmux.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	// Record metrics
	metrics.RecordRequest("llmux", req.Model, http.StatusOK, latency)
	if completionResp.Usage != nil {
		metrics.RecordTokens("llmux", req.Model, completionResp.Usage.PromptTokens, completionResp.Usage.CompletionTokens)
	}

	// Record usage and update spent budget for budget control
	h.recordUsage(r.Context(), requestID, req.Model, "completion", completionResp.Usage, start, latency)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(completionResp); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *ClientHandler) handleCompletionStreamResponse(w http.ResponseWriter, r *http.Request, client *llmux.Client, req *llmux.ChatRequest, start time.Time, requestID string) {
	stream, err := client.ChatCompletionStream(r.Context(), req)
	if err != nil {
		h.logger.Error("stream creation failed", "model", req.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, err.Error()))
		}
		return
	}
	defer func() { _ = stream.Close() }()

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

	var finalUsage *llmux.Usage
	var completionContent strings.Builder

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			if _, writeErr := w.Write([]byte("data: [DONE]\n\n")); writeErr != nil {
				h.logger.Debug("failed to write done marker", "error", writeErr)
			}
			flusher.Flush()
			break
		}
		if err != nil {
			if r.Context().Err() != nil {
				h.logger.Debug("client disconnected during stream", "model", req.Model)
			} else {
				h.logger.Error("stream recv error", "error", err, "model", req.Model)
			}
			break
		}

		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}

		if len(chunk.Choices) > 0 {
			completionContent.WriteString(chunk.Choices[0].Delta.Content)
		}

		converted := types.CompletionStreamChunkFromChat(chunk)
		data, marshalErr := json.Marshal(converted)
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

	latency := time.Since(start)
	metrics.RecordRequest("llmux", req.Model, http.StatusOK, latency)

	if finalUsage == nil {
		promptTokens := tokenizer.EstimatePromptTokens(req.Model, req)
		completionTokens := tokenizer.EstimateCompletionTokensFromText(req.Model, completionContent.String())
		finalUsage = &llmux.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	h.recordUsage(r.Context(), requestID, req.Model, "completion", finalUsage, start, latency)
}

func firstCompletionText(resp *types.CompletionResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Text
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

// recordUsage logs API usage to the store and updates spend budgets.
// This is called after successful completion (streaming or non-streaming).
func (h *ClientHandler) recordUsage(ctx context.Context, requestID, model, callType string, usage *llmux.Usage, start time.Time, latency time.Duration) {
	if h.store == nil {
		return
	}

	// Extract authentication context for tracking
	authCtx := auth.GetAuthContext(ctx)

	// Build usage log entry
	log := &auth.UsageLog{
		RequestID: requestID,
		Model:     model,
		Provider:  "llmux",
		CallType:  callType,
		StartTime: start,
		EndTime:   time.Now(),
		LatencyMs: int(latency.Milliseconds()),
		CacheHit:  nil, // No cache hit for now
	}

	// Set token counts if available
	if usage != nil {
		log.InputTokens = usage.PromptTokens
		log.OutputTokens = usage.CompletionTokens
		log.TotalTokens = usage.TotalTokens
		client, release := h.acquireClient()
		if client != nil {
			log.Cost = client.CalculateCost(model, usage)
		}
		release()
	}

	// Set API key and team info from auth context
	if authCtx != nil && authCtx.APIKey != nil {
		log.APIKeyID = authCtx.APIKey.ID
		log.TeamID = authCtx.APIKey.TeamID
		log.OrganizationID = authCtx.APIKey.OrganizationID
		log.UserID = authCtx.APIKey.UserID
	}

	// Record usage asynchronously to avoid blocking the response
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Log usage
		if err := h.store.LogUsage(bgCtx, log); err != nil {
			h.logger.Warn("failed to log usage", "error", err, "request_id", requestID)
		}

		// Update API key spend (if we have cost data)
		if authCtx != nil && authCtx.APIKey != nil && log.Cost > 0 {
			if err := h.store.UpdateAPIKeySpent(bgCtx, authCtx.APIKey.ID, log.Cost); err != nil {
				h.logger.Warn("failed to update api key spend", "error", err, "key_id", authCtx.APIKey.ID)
			}

			// Update team spend if applicable
			if authCtx.APIKey.TeamID != nil {
				if err := h.store.UpdateTeamSpent(bgCtx, *authCtx.APIKey.TeamID, log.Cost); err != nil {
					h.logger.Warn("failed to update team spend", "error", err, "team_id", *authCtx.APIKey.TeamID)
				}
			}
		}
	}()
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
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, llmerrors.NewInternalError("", "", "client not initialized"))
		return
	}

	models, err := client.ListModels(r.Context())
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError("", "", "failed to list models: "+err.Error()))
		return
	}

	authCtx := auth.GetAuthContext(r.Context())
	if authCtx != nil {
		access, err := auth.NewModelAccess(r.Context(), h.store, authCtx)
		if err != nil {
			h.logger.Error("failed to evaluate model access", "error", err)
			h.writeError(w, llmerrors.NewInternalError("", "", "failed to evaluate model access"))
			return
		}
		if access != nil {
			filtered := models[:0]
			for _, model := range models {
				if access.Allows(model.ID) {
					filtered = append(filtered, model)
				}
			}
			models = filtered
		}
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
	if h == nil || h.swapper == nil {
		return nil
	}
	return h.swapper.Current()
}

// Embeddings handles POST /v1/embeddings requests.
func (h *ClientHandler) Embeddings(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := uuid.New().String()

	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, llmerrors.NewInternalError("", "", "client not initialized"))
		return
	}

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

	// Call client.Embedding
	resp, err := client.Embedding(r.Context(), &req)
	if err != nil {
		h.logger.Error("embedding failed", "model", req.Model, "error", err)
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
	if resp.Usage.TotalTokens > 0 {
		metrics.RecordTokens("llmux", req.Model, resp.Usage.PromptTokens, 0)
	}

	// Record usage for budget tracking
	h.recordEmbeddingUsage(r.Context(), requestID, req.Model, &resp.Usage, start, latency)

	// Write response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// recordEmbeddingUsage logs embedding API usage to the store and updates spend budgets.
func (h *ClientHandler) recordEmbeddingUsage(ctx context.Context, requestID, model string, usage *types.Usage, start time.Time, latency time.Duration) {
	if h.store == nil {
		return
	}

	// Extract authentication context for tracking
	authCtx := auth.GetAuthContext(ctx)

	// Build usage log entry
	log := &auth.UsageLog{
		RequestID: requestID,
		Model:     model,
		Provider:  "llmux",
		CallType:  "embedding",
		StartTime: start,
		EndTime:   time.Now(),
		LatencyMs: int(latency.Milliseconds()),
		CacheHit:  nil,
	}

	// Set token counts if available
	if usage != nil {
		log.InputTokens = usage.PromptTokens
		log.TotalTokens = usage.TotalTokens
		client, release := h.acquireClient()
		if client != nil {
			log.Cost = client.CalculateCost(model, &llmux.Usage{
				PromptTokens:     usage.PromptTokens,
				CompletionTokens: 0,
				TotalTokens:      usage.TotalTokens,
			})
		}
		release()
	}

	// Set API key and team info from auth context
	if authCtx != nil && authCtx.APIKey != nil {
		log.APIKeyID = authCtx.APIKey.ID
		log.TeamID = authCtx.APIKey.TeamID
		log.OrganizationID = authCtx.APIKey.OrganizationID
		log.UserID = authCtx.APIKey.UserID
	}

	// Record usage asynchronously to avoid blocking the response
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Log usage
		if err := h.store.LogUsage(bgCtx, log); err != nil {
			h.logger.Warn("failed to log embedding usage", "error", err, "request_id", requestID)
		}

		// Update API key spend (if we have cost data)
		if authCtx != nil && authCtx.APIKey != nil && log.Cost > 0 {
			if err := h.store.UpdateAPIKeySpent(bgCtx, authCtx.APIKey.ID, log.Cost); err != nil {
				h.logger.Warn("failed to update api key spend", "error", err, "key_id", authCtx.APIKey.ID)
			}

			// Update team spend if applicable
			if authCtx.APIKey.TeamID != nil {
				if err := h.store.UpdateTeamSpent(bgCtx, *authCtx.APIKey.TeamID, log.Cost); err != nil {
					h.logger.Warn("failed to update team spend", "error", err, "team_id", *authCtx.APIKey.TeamID)
				}
			}
		}
	}()
}

// Ensure streaming package is imported for parser registration
var _ = streaming.GetParser
