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
	seenDone        bool
	requestEnded    bool // tracks whether ReportRequestEnd has been called for current deployment
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
) *StreamReader {
	scanner := bufio.NewScanner(body)
	// Increase buffer size for large chunks
	scanner.Buffer(make([]byte, 4096), 4096*4)

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
		s.router.ReportFailure(s.deployment, err)

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

		_ = s.close()
		return nil, err
	}

	// Check for premature EOF
	if !s.seenDone {
		err := io.ErrUnexpectedEOF
		s.router.ReportFailure(s.deployment, err)

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
		_ = s.close()
		return nil, err
	}

	// Stream ended normally
	s.finish()
	return nil, io.EOF
}

//nolint:unparam // err parameter kept for future error classification
func (s *StreamReader) canRecover(err error) bool {
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
	s.mu.Unlock()

	// Construct new request
	newReq := *s.originalReq
	newReq.Messages = make([]types.ChatMessage, len(s.originalReq.Messages))
	copy(newReq.Messages, s.originalReq.Messages)

	// Append accumulated content as assistant message
	if currentAccumulated != "" {
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
	deployment, err = s.client.router.Pick(s.ctx, newReq.Model)
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
	httpReq, err := prov.BuildRequest(s.ctx, &newReq)
	if err != nil {
		return nil, fmt.Errorf("recovery build request failed: %w", err)
	}

	s.router.ReportRequestStart(deployment)
	s.mu.Lock()
	s.requestEnded = false // New request started
	s.mu.Unlock()

	resp, err := s.client.httpClient.Do(httpReq)
	if err != nil {
		s.router.ReportFailure(deployment, err)
		s.router.ReportRequestEnd(deployment)
		s.mu.Lock()
		s.requestEnded = true
		s.mu.Unlock()
		return nil, fmt.Errorf("recovery execute failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		llmErr := prov.MapError(resp.StatusCode, body)
		s.router.ReportFailure(deployment, llmErr)
		s.router.ReportRequestEnd(deployment)
		s.mu.Lock()
		s.requestEnded = true
		s.mu.Unlock()
		return nil, fmt.Errorf("recovery api error: %w", llmErr)
	}

	// Update StreamReader state
	s.mu.Lock()
	s.body = resp.Body
	s.scanner = bufio.NewScanner(resp.Body)
	s.scanner.Buffer(make([]byte, 4096), 4096*4)
	s.provider = prov
	s.deployment = deployment
	s.mu.Unlock()

	// Recursive call to get next chunk
	return s.Recv()
}

// Close releases resources associated with the stream.
// It's safe to call Close multiple times.
func (s *StreamReader) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if !s.requestEnded && s.deployment != nil {
		s.router.ReportRequestEnd(s.deployment)
		s.requestEnded = true
	}
}

// closeBody closes the body without reporting (must be called with lock held).
func (s *StreamReader) closeBody() error {
	if s.closed {
		return nil
	}
	s.closed = true
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
		latency := time.Since(s.startTime)
		s.router.ReportSuccess(s.deployment, &router.ResponseMetrics{
			Latency:          latency,
			TimeToFirstToken: s.ttft,
		})
		_ = s.close()
	}
}
