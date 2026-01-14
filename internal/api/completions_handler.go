package api //nolint:revive // package name is intentional

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/pool"
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/streaming"
	"github.com/blueberrycongee/llmux/internal/tokenizer"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	pkgprovider "github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// Completions handles POST /v1/completions requests.
func (h *Handler) Completions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

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

	if chatReq.Stream {
		if chatReq.StreamOptions == nil {
			chatReq.StreamOptions = &types.StreamOptions{IncludeUsage: true}
		}
	}

	promptTokens := tokenizer.EstimatePromptTokens(chatReq.Model, chatReq)
	routeCtx := buildRouterRequestContext(chatReq, promptTokens, chatReq.Stream)
	deployment, err := h.llmRouter.PickWithContext(r.Context(), routeCtx)
	if err != nil {
		h.logger.Error("no deployment available", "model", chatReq.Model, "error", err)
		h.writeError(w, llmerrors.NewServiceUnavailableError("", chatReq.Model, "no available deployment"))
		return
	}

	prov, ok := h.registry.GetProvider(deployment.ProviderName)
	if !ok {
		h.writeError(w, llmerrors.NewInternalError(deployment.ProviderName, chatReq.Model, "provider not found"))
		return
	}

	timeout := time.Duration(deployment.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	upstreamCtx, reqCancel := context.WithTimeout(r.Context(), timeout)
	defer reqCancel()

	upstreamReq, err := prov.BuildRequest(upstreamCtx, sanitizeChatRequestForProvider(chatReq))
	if err != nil {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), chatReq.Model, "failed to build request: "+err.Error()))
		return
	}

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		h.llmRouter.ReportFailure(deployment, err)
		metrics.RecordError(prov.Name(), "connection_error")
		h.writeError(w, llmerrors.NewServiceUnavailableError(prov.Name(), chatReq.Model, "upstream request failed"))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if transformer, ok := upstreamReq.Context().Value(pkgprovider.ResponseTransformerKey).(pkgprovider.ResponseTransformer); ok {
		resp.Body = transformer(resp.Body)
	}

	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		llmErr := prov.MapError(resp.StatusCode, respBody)
		h.llmRouter.ReportFailure(deployment, llmErr)
		metrics.RecordRequest(prov.Name(), chatReq.Model, resp.StatusCode, latency)
		h.writeError(w, llmErr)
		return
	}

	if chatReq.Stream {
		h.handleCompletionStreamResponse(w, r, resp, prov, deployment, chatReq.Model, start)
		return
	}

	chatResp, err := prov.ParseResponse(resp)
	if err != nil {
		h.llmRouter.ReportFailure(deployment, err)
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), chatReq.Model, "failed to parse response"))
		return
	}
	defer pool.PutChatResponse(chatResp)

	completionResp := types.CompletionResponseFromChat(chatResp)

	h.llmRouter.ReportSuccess(deployment, &router.ResponseMetrics{Latency: latency})
	metrics.RecordRequest(prov.Name(), chatReq.Model, http.StatusOK, latency)
	if chatResp.Usage != nil {
		metrics.RecordTokens(prov.Name(), chatReq.Model, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(completionResp)
}

func (h *Handler) handleCompletionStreamResponse(w http.ResponseWriter, r *http.Request, resp *http.Response, prov provider.Provider, deployment *pkgprovider.Deployment, model string, start time.Time) {
	parser := streaming.GetParser(prov.Name())

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, llmerrors.NewInternalError(prov.Name(), model, "streaming not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, streaming.DefaultBufferSize)
	scanner.Buffer(buf, streaming.DefaultBufferSize*4)

	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		line := scanner.Bytes()
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}

		if bytes.Equal(trimmed, []byte(streaming.SSEDataPrefix+streaming.SSEDone)) ||
			bytes.Equal(trimmed, []byte(streaming.SSEDone)) {
			_, _ = w.Write([]byte(streaming.SSEDataPrefix + streaming.SSEDone + "\n\n"))
			flusher.Flush()
			break
		}

		if parser == nil {
			continue
		}

		chunk, err := parser.ParseChunk(trimmed)
		if err != nil || chunk == nil {
			continue
		}

		converted := types.CompletionStreamChunkFromChat(chunk)
		data, err := json.Marshal(converted)
		if err != nil {
			continue
		}

		_, _ = w.Write([]byte(streaming.SSEDataPrefix))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		h.logger.Error("stream forward error", "error", err, "model", model)
	}

	latency := time.Since(start)
	h.llmRouter.ReportSuccess(deployment, &router.ResponseMetrics{Latency: latency})
	metrics.RecordRequest(prov.Name(), model, http.StatusOK, latency)
}
