package api //nolint:revive // package name is intentional

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/governance"
	"github.com/blueberrycongee/llmux/internal/mcp"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/tokenizer"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// Responses handles POST /v1/responses requests.
func (h *ClientHandler) Responses(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	r, requestID := h.ensureRequestID(r)

	limitedReader := io.LimitReader(r.Body, h.maxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "failed to read request body"))
		return
	}
	defer func() { _ = r.Body.Close() }()

	if int64(len(body)) > h.maxBodySize {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "request body too large"))
		return
	}

	var req types.ResponseRequest
	if unmarshalErr := json.Unmarshal(body, &req); unmarshalErr != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "invalid JSON: "+unmarshalErr.Error()))
		return
	}
	if req.Model == "" {
		h.writeError(w, llmerrors.NewInvalidRequestError("", "", "model is required"))
		return
	}

	chatReq, err := req.ToChatRequest()
	if err != nil {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, err.Error()))
		return
	}
	if len(chatReq.Messages) == 0 {
		h.writeError(w, llmerrors.NewInvalidRequestError("", req.Model, "input is required"))
		return
	}

	payload := h.buildChatObservabilityPayload(r, chatReq, start, requestID)
	payload.CallType = observability.CallTypeResponse
	ctx, endSpan := h.startSpan(r.Context(), payload)
	defer endSpan()
	h.observePre(ctx, payload)

	if evalErr := h.evaluateGovernance(ctx, r, chatReq.Model, chatReq.User, chatReq.Tags, governance.CallTypeChatCompletion); evalErr != nil {
		h.observePost(ctx, payload, evalErr)
		h.writeError(w, evalErr)
		return
	}

	manager := h.getMCPManager(ctx)

	client, release := h.acquireClient()
	defer release()
	if client == nil {
		err := llmerrors.NewInternalError("", chatReq.Model, "client not initialized")
		h.observePost(ctx, payload, err)
		h.writeError(w, err)
		return
	}

	if chatReq.Stream {
		if manager != nil {
			if injector, ok := manager.(mcp.ToolInjector); ok {
				injector.InjectTools(ctx, chatReq)
			}
		}

		h.handleResponseStream(ctx, w, r, client, chatReq, start, requestID, payload)
		return
	}

	var resp *llmux.ChatResponse
	if manager != nil {
		executor := mcp.NewAgentExecutor(manager, 0, h.logger)
		resp, err = executor.Execute(ctx, chatReq, func(execCtx context.Context, r *llmux.ChatRequest) (*llmux.ChatResponse, error) {
			return client.ChatCompletion(execCtx, r)
		})
	} else {
		resp, err = client.ChatCompletion(ctx, chatReq)
	}
	if err != nil {
		h.observePost(ctx, payload, err)
		h.logger.Error("response completion failed", "model", chatReq.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", chatReq.Model, err.Error()))
		}
		return
	}

	latency := time.Since(start)

	if resp.Usage == nil || resp.Usage.TotalTokens == 0 {
		promptTokens := tokenizer.EstimatePromptTokens(chatReq.Model, chatReq)
		completionTokens := tokenizer.EstimateCompletionTokens(chatReq.Model, resp, "")
		resp.Usage = &llmux.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	metrics.RecordRequest("llmux", chatReq.Model, http.StatusOK, latency)
	if resp.Usage != nil {
		metrics.RecordTokens("llmux", chatReq.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}

	modelName := chatReq.Model
	if resp.Model != "" {
		modelName = resp.Model
	}
	cost := 0.0
	if resp.Usage != nil {
		cost = client.CalculateCost(modelName, resp.Usage)
	}
	h.accountUsage(ctx, governance.AccountInput{
		RequestID:   requestID,
		Model:       modelName,
		CallType:    governance.CallTypeChatCompletion,
		EndUserID:   chatReq.User,
		RequestTags: chatReq.Tags,
		Usage: governance.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			Cost:             cost,
			Provider:         resp.Usage.Provider,
		},
		Start:   start,
		Latency: latency,
	})

	response := types.ResponseResponseFromChat(resp)
	if response != nil {
		response.Usage = resp.Usage
	}

	if payload != nil {
		payload.PromptTokens = resp.Usage.PromptTokens
		payload.CompletionTokens = resp.Usage.CompletionTokens
		payload.TotalTokens = resp.Usage.TotalTokens
		payload.ResponseCost = cost
		if resp.Usage.Provider != "" {
			payload.APIProvider = resp.Usage.Provider
		}
		if payload.APIProvider == "" {
			payload.APIProvider = "llmux"
		}
		if resp.Model != "" {
			payload.Model = resp.Model
		}
		payload.ID = resp.ID
		payload.Response = response
	}
	h.observePost(ctx, payload, nil)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *ClientHandler) handleResponseStream(ctx context.Context, w http.ResponseWriter, r *http.Request, client *llmux.Client, req *llmux.ChatRequest, start time.Time, requestID string, payload *observability.StandardLoggingPayload) {
	stream, err := client.ChatCompletionStream(ctx, req)
	if err != nil {
		h.observePost(ctx, payload, err)
		h.logger.Error("response stream creation failed", "model", req.Model, "error", err)
		if llmErr, ok := err.(*llmerrors.LLMError); ok {
			h.writeError(w, llmErr)
		} else {
			h.writeError(w, llmerrors.NewServiceUnavailableError("", req.Model, err.Error()))
		}
		return
	}
	defer func() { _ = stream.Close() }()

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
	var responseID string
	var responseModel string
	var responseCreated int64
	var completionContent strings.Builder
	var streamErr error
	completed := false

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			completed = true
			break
		}
		if err != nil {
			streamErr = err
			if r.Context().Err() != nil {
				h.logger.Debug("client disconnected during response stream", "model", req.Model)
			} else {
				h.logger.Error("response stream recv error", "error", err, "model", req.Model)
			}
			break
		}

		h.observeStreamEvent(ctx, payload, chunk)

		if responseID == "" && chunk.ID != "" {
			responseID = chunk.ID
			responseModel = chunk.Model
			responseCreated = chunk.Created
		}

		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				completionContent.WriteString(delta)
				event := types.ResponseStreamChunk{
					Type:  "response.output_text.delta",
					Delta: delta,
				}
				h.writeResponseEvent(w, flusher, event)
			}
		}
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

	cost := 0.0
	if finalUsage != nil {
		cost = client.CalculateCost(req.Model, finalUsage)
	}
	h.accountUsage(ctx, governance.AccountInput{
		RequestID:   requestID,
		Model:       req.Model,
		CallType:    governance.CallTypeChatCompletion,
		EndUserID:   req.User,
		RequestTags: req.Tags,
		Usage: governance.Usage{
			PromptTokens:     finalUsage.PromptTokens,
			CompletionTokens: finalUsage.CompletionTokens,
			TotalTokens:      finalUsage.TotalTokens,
			Cost:             cost,
			Provider:         finalUsage.Provider,
		},
		Start:   start,
		Latency: latency,
	})

	if payload != nil && finalUsage != nil {
		payload.PromptTokens = finalUsage.PromptTokens
		payload.CompletionTokens = finalUsage.CompletionTokens
		payload.TotalTokens = finalUsage.TotalTokens
		payload.ResponseCost = cost
		if finalUsage.Provider != "" {
			payload.APIProvider = finalUsage.Provider
		}
		if payload.APIProvider == "" {
			payload.APIProvider = "llmux"
		}
		payload.Response = completionContent.String()
	}

	if completed {
		response := responseFromStream(responseID, responseModel, responseCreated, req.Model, completionContent.String(), finalUsage)
		event := types.ResponseStreamChunk{
			Type:     "response.completed",
			Response: response,
		}
		h.writeResponseEvent(w, flusher, event)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}

	h.observePost(ctx, payload, streamErr)
}

func (h *ClientHandler) writeResponseEvent(w http.ResponseWriter, flusher http.Flusher, event types.ResponseStreamChunk) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal response stream event", "error", err)
		return
	}
	if _, err := w.Write([]byte("data: ")); err != nil {
		return
	}
	if _, err := w.Write(data); err != nil {
		return
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		return
	}
	flusher.Flush()
}

func responseFromStream(responseID, responseModel string, created int64, fallbackModel string, content string, usage *llmux.Usage) *types.ResponseResponse {
	model := responseModel
	if model == "" {
		model = fallbackModel
	}
	contentJSON, _ := json.Marshal(content)
	resp := &types.ChatResponse{
		ID:      responseID,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []types.Choice{
			{
				Index: 0,
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: contentJSON,
				},
				FinishReason: "stop",
			},
		},
		Usage: usage,
	}
	if resp.ID == "" {
		resp.ID = "resp-stream"
	}
	if resp.Created == 0 {
		resp.Created = time.Now().Unix()
	}
	response := types.ResponseResponseFromChat(resp)
	if response != nil {
		response.Usage = usage
	}
	return response
}
