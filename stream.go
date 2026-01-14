package llmux

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/internal/tokenizer"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// StreamReader provides an iterator interface for streaming responses.
// It handles SSE parsing and provides a simple Recv() method for consuming chunks.
//
// Example:
//
//	stream, err := client.ChatCompletionStream(ctx, req)
//	if err != nil {
//	    return err
//	}
//	defer stream.Close()
//
//	for {
//	    chunk, err := stream.Recv()
//	    if err == io.EOF {
//	        break
//	    }
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	}
type StreamReader struct {
	body       io.ReadCloser
	scanner    *bufio.Scanner
	provider   provider.Provider
	deployment *provider.Deployment
	router     router.Router

	closed     bool
	firstChunk bool
	startTime  time.Time
	ttft       time.Duration // Time To First Token

	mu sync.Mutex

	// Resilience fields
	ctx             context.Context
	client          *Client
	originalReq     *types.ChatRequest
	accumulated     strings.Builder
	retryCount      int
	maxRetries      int
	fallbackEnabled bool
	recoveryMode    StreamRecoveryMode
	skipRemaining   int
	seenDone        bool
	requestEnded    bool // tracks whether ReportRequestEnd has been called for current deployment

	pluginStream  <-chan *types.StreamChunk
	pipeline      *plugin.Pipeline
	pluginCtx     *plugin.Context
	streamRunFrom int
	postHooksRun  bool

	release func()
}

// newStreamReader creates a new StreamReader.
func newStreamReader(
	ctx context.Context,
	client *Client,
	req *types.ChatRequest,
	body io.ReadCloser,
	prov provider.Provider,
	deployment *provider.Deployment,
	r router.Router,
	pipeline *plugin.Pipeline,
	pluginCtx *plugin.Context,
	runFrom int,
	release func(),
) *StreamReader {
	scanner := bufio.NewScanner(body)
	// Allow larger SSE lines (bufio.Scanner defaults to 64K, and old code used 16KB).
	// Keep a small initial buffer to reduce allocations.
	scanner.Buffer(make([]byte, 4096), 256*1024)

	return &StreamReader{
		body:            body,
		scanner:         scanner,
		provider:        prov,
		deployment:      deployment,
		router:          r,
		firstChunk:      true,
		startTime:       time.Now(),
		ctx:             ctx,
		client:          client,
		originalReq:     req,
		maxRetries:      client.config.RetryCount,
		fallbackEnabled: client.config.FallbackEnabled,
		recoveryMode:    client.config.StreamRecoveryMode,
		pipeline:        pipeline,
		pluginCtx:       pluginCtx,
		streamRunFrom:   runFrom,
		release:         release,
	}
}

func newStreamReaderFromChannel(
	ctx context.Context,
	client *Client,
	req *types.ChatRequest,
	stream <-chan *types.StreamChunk,
	pipeline *plugin.Pipeline,
	pluginCtx *plugin.Context,
	runFrom int,
) *StreamReader {
	return &StreamReader{
		ctx:             ctx,
		client:          client,
		originalReq:     req,
		firstChunk:      true,
		startTime:       time.Now(),
		maxRetries:      client.config.RetryCount,
		fallbackEnabled: client.config.FallbackEnabled,
		recoveryMode:    client.config.StreamRecoveryMode,
		pluginStream:    stream,
		pipeline:        pipeline,
		pluginCtx:       pluginCtx,
		streamRunFrom:   runFrom,
	}
}

// Recv returns the next chunk from the stream.
// Returns io.EOF when the stream is complete.
// Returns an error if the stream encounters an error.
func (s *StreamReader) Recv() (*types.StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, io.EOF
	}

	if s.pluginStream != nil {
		return s.recvFromPluginStreamLocked()
	}

	for s.scanner.Scan() {
		line := s.scanner.Bytes()

		// Skip empty lines
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}

		// Check for stream end markers
		if bytes.Equal(trimmed, []byte("data: [DONE]")) ||
			bytes.Equal(trimmed, []byte("[DONE]")) {
			s.seenDone = true
			s.finish()
			return nil, io.EOF
		}

		// Parse chunk using provider-specific parser
		chunk, err := s.provider.ParseStreamChunk(trimmed)
		if err != nil {
			// Skip unparseable chunks (could be comments or keep-alive)
			continue
		}

		if chunk == nil {
			// Skip non-content events
			continue
		}

		if s.recoveryMode == StreamRecoveryRetry && s.skipRemaining > 0 && len(chunk.Choices) > 0 {
			delta := &chunk.Choices[0].Delta
			if delta.Content != "" {
				trimmed, remaining := trimLeadingRunes(delta.Content, s.skipRemaining)
				s.skipRemaining = remaining
				delta.Content = trimmed
			}
			if delta.Content == "" && delta.Role == "" && len(delta.ToolCalls) == 0 {
				continue
			}
		}

		chunk = s.applyStreamPluginsLocked(chunk)
		if chunk == nil {
			continue
		}

		// Record Time To First Token on first content chunk
		if s.firstChunk {
			s.ttft = time.Since(s.startTime)
			s.firstChunk = false
		}

		// Accumulate content for recovery
		if len(chunk.Choices) > 0 {
			s.accumulated.WriteString(chunk.Choices[0].Delta.Content)
		}

		return chunk, nil
	}

	// Check for scanner errors
	if err := s.scanner.Err(); err != nil {
		s.reportFailure(err)

		// Try to recover
		for s.canRecover(err) {
			s.mu.Unlock() // Unlock before retry to avoid deadlock in recursive Recv
			chunk, retryErr := s.tryRecover(err)
			s.mu.Lock() // Re-lock
			if retryErr == nil {
				return chunk, nil
			}
			// If retry failed, try again (retryCount is incremented in tryRecover)
		}

		s.finalizeStreamLocked(err)
		_ = s.close()
		return nil, err
	}

	// Check for premature EOF
	if !s.seenDone {
		err := io.ErrUnexpectedEOF
		s.reportFailure(err)

		for s.canRecover(err) {
			s.mu.Unlock()
			chunk, retryErr := s.tryRecover(err)
			s.mu.Lock()
			if retryErr == nil {
				return chunk, nil
			}
			// Retry failed, loop again
		}
		// If recovery failed or not possible, report failure
		s.finalizeStreamLocked(err)
		_ = s.close()
		return nil, err
	}

	// Stream ended normally
	s.finish()
	return nil, io.EOF
}

func (s *StreamReader) recvFromPluginStreamLocked() (*types.StreamChunk, error) {
	for {
		chunk, ok := <-s.pluginStream
		if !ok {
			s.seenDone = true
			s.finish()
			return nil, io.EOF
		}
		if chunk == nil {
			continue
		}

		chunk = s.applyStreamPluginsLocked(chunk)
		if chunk == nil {
			continue
		}

		if s.firstChunk {
			s.ttft = time.Since(s.startTime)
			s.firstChunk = false
		}

		if len(chunk.Choices) > 0 {
			s.accumulated.WriteString(chunk.Choices[0].Delta.Content)
		}

		return chunk, nil
	}
}

func (s *StreamReader) applyStreamPluginsLocked(chunk *types.StreamChunk) *types.StreamChunk {
	if s.pipeline == nil || s.pluginCtx == nil || chunk == nil {
		return chunk
	}

	out, _ := s.pipeline.RunOnStreamChunk(s.pluginCtx, chunk)
	return out
}

func (s *StreamReader) finalizeStreamLocked(err error) {
	if s.postHooksRun {
		return
	}
	s.postHooksRun = true

	if s.pipeline == nil || s.pluginCtx == nil {
		return
	}

	_ = s.pipeline.RunStreamPostHooks(s.pluginCtx, err, s.streamRunFrom)
	s.pipeline.PutContext(s.pluginCtx)
	s.pluginCtx = nil
}

func (s *StreamReader) reportFailure(err error) {
	if s.router == nil || s.deployment == nil {
		return
	}
	s.router.ReportFailure(s.ctx, s.deployment, err)
}

//nolint:unparam // err parameter kept for future error classification
func (s *StreamReader) canRecover(err error) bool {
	if s.recoveryMode == StreamRecoveryOff {
		return false
	}
	if s.retryCount >= s.maxRetries {
		return false
	}
	// We assume most scanner errors (unexpected EOF, connection reset) are recoverable
	// unless context is canceled
	if s.ctx.Err() != nil {
		return false
	}
	return true
}

//nolint:unparam // originalErr kept for future logging/debugging
func (s *StreamReader) tryRecover(originalErr error) (*types.StreamChunk, error) {
	s.mu.Lock()
	// End request for current deployment if not already ended
	s.endRequest()
	// Close current stream resources but keep StreamReader alive
	_ = s.closeBody()
	s.closed = false // Re-open logically
	s.retryCount++
	currentAccumulated := s.accumulated.String()
	if s.recoveryMode == StreamRecoveryRetry {
		s.skipRemaining = utf8.RuneCountInString(currentAccumulated)
	}
	s.mu.Unlock()

	// Construct new request
	newReq := *s.originalReq
	newReq.Messages = make([]types.ChatMessage, len(s.originalReq.Messages))
	copy(newReq.Messages, s.originalReq.Messages)

	// Append accumulated content as assistant message
	if s.recoveryMode == StreamRecoveryAppend && currentAccumulated != "" {
		contentBytes, _ := json.Marshal(currentAccumulated)
		newReq.Messages = append(newReq.Messages, types.ChatMessage{
			Role:    "assistant",
			Content: contentBytes,
		})
	}

	// Pick new deployment
	var deployment *provider.Deployment
	var err error

	// If fallback enabled, try to pick a new node.
	// If disabled, we might still want to retry on the same node if it was a transient error,
	// but Pick() handles that logic (it might return the same node).
	promptTokens := tokenizer.EstimatePromptTokens(newReq.Model, &newReq)
	reqCtx := buildRouterRequestContext(&newReq, promptTokens, true)
	deployment, err = s.client.router.PickWithContext(s.ctx, reqCtx)
	if err != nil {
		return nil, fmt.Errorf("recovery pick failed: %w", err)
	}

	// Get provider
	s.client.mu.RLock()
	prov, ok := s.client.providers[deployment.ProviderName]
	s.client.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("recovery provider not found: %s", deployment.ProviderName)
	}

	// Build request
	httpReq, err := prov.BuildRequest(s.ctx, sanitizeChatRequestForProvider(&newReq))
	if err != nil {
		return nil, fmt.Errorf("recovery build request failed: %w", err)
	}

	release, err := s.client.acquireDeployment(s.ctx, deployment)
	if err != nil {
		return nil, err
	}

	if s.router != nil && deployment != nil {
		s.router.ReportRequestStart(s.ctx, deployment)
	}
	s.mu.Lock()
	s.requestEnded = false // New request started
	s.release = release
	s.mu.Unlock()

	resp, err := s.client.streamHTTPClient.Do(httpReq)
	if err != nil {
		release()
		if s.router != nil && deployment != nil {
			s.router.ReportFailure(s.ctx, deployment, err)
			s.router.ReportRequestEnd(s.ctx, deployment)
		}
		s.mu.Lock()
		s.requestEnded = true
		s.release = nil
		s.mu.Unlock()
		return nil, fmt.Errorf("recovery execute failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		llmErr := prov.MapError(resp.StatusCode, body)
		release()
		if s.router != nil && deployment != nil {
			s.router.ReportFailure(s.ctx, deployment, llmErr)
			s.router.ReportRequestEnd(s.ctx, deployment)
		}
		s.mu.Lock()
		s.requestEnded = true
		s.release = nil
		s.mu.Unlock()
		return nil, fmt.Errorf("recovery api error: %w", llmErr)
	}

	// Update StreamReader state
	s.mu.Lock()
	if s.recoveryMode == StreamRecoveryRetry {
		s.accumulated.Reset()
	}
	s.body = resp.Body
	s.scanner = bufio.NewScanner(resp.Body)
	s.scanner.Buffer(make([]byte, 4096), 256*1024)
	s.provider = prov
	s.deployment = deployment
	if s.pluginCtx != nil {
		s.pluginCtx.Provider = deployment.ProviderName
	}
	s.mu.Unlock()

	// Recursive call to get next chunk
	return s.Recv()
}

// Close releases resources associated with the stream.
// It's safe to call Close multiple times.
func (s *StreamReader) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	streamErr := s.ctx.Err()
	if streamErr == nil && !s.seenDone {
		streamErr = io.ErrUnexpectedEOF
	}
	s.finalizeStreamLocked(streamErr)
	return s.close()
}

// TTFT returns the Time To First Token duration.
// Returns 0 if no chunks have been received yet.
func (s *StreamReader) TTFT() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ttft
}

// endRequest reports request end if not already reported (must be called with lock held).
func (s *StreamReader) endRequest() {
	if s.requestEnded {
		return
	}
	if s.router != nil && s.deployment != nil {
		s.router.ReportRequestEnd(s.ctx, s.deployment)
	}
	if s.release != nil {
		s.release()
		s.release = nil
	}
	s.requestEnded = true
}

// closeBody closes the body without reporting (must be called with lock held).
func (s *StreamReader) closeBody() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.body == nil {
		return nil
	}
	return s.body.Close()
}

// close releases resources and reports request end (must be called with lock held).
func (s *StreamReader) close() error {
	if s.closed {
		return nil
	}
	s.endRequest()
	return s.closeBody()
}

// finish reports success metrics and closes the stream.
func (s *StreamReader) finish() {
	if !s.closed {
		if s.router != nil && s.deployment != nil {
			latency := time.Since(s.startTime)
			promptTokens := tokenizer.EstimatePromptTokens(s.originalReq.Model, s.originalReq)
			completionTokens := tokenizer.EstimateCompletionTokensFromText(s.originalReq.Model, s.accumulated.String())
			s.router.ReportSuccess(s.ctx, s.deployment, &router.ResponseMetrics{
				Latency:          latency,
				TimeToFirstToken: s.ttft,
				InputTokens:      promptTokens,
				OutputTokens:     completionTokens,
				TotalTokens:      promptTokens + completionTokens,
			})
		}
		s.finalizeStreamLocked(nil)
		_ = s.close()
	}
}

func trimLeadingRunes(value string, count int) (string, int) {
	if count <= 0 || value == "" {
		return value, count
	}

	seen := 0
	for i := range value {
		if seen == count {
			return value[i:], 0
		}
		seen++
	}

	return "", count - seen
}
