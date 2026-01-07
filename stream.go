package llmux

import (
	"bufio"
	"bytes"
	"io"
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
}

// newStreamReader creates a new StreamReader.
func newStreamReader(
	body io.ReadCloser,
	prov provider.Provider,
	deployment *provider.Deployment,
	r router.Router,
) *StreamReader {
	scanner := bufio.NewScanner(body)
	// Increase buffer size for large chunks
	scanner.Buffer(make([]byte, 4096), 4096*4)

	return &StreamReader{
		body:       body,
		scanner:    scanner,
		provider:   prov,
		deployment: deployment,
		router:     r,
		firstChunk: true,
		startTime:  time.Now(),
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

		return chunk, nil
	}

	// Check for scanner errors
	if err := s.scanner.Err(); err != nil {
		s.router.ReportFailure(s.deployment, err)
		s.close()
		return nil, err
	}

	// Stream ended normally
	s.finish()
	return nil, io.EOF
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

// close releases resources (must be called with lock held).
func (s *StreamReader) close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.router.ReportRequestEnd(s.deployment)
	return s.body.Close()
}

// finish reports success metrics and closes the stream.
func (s *StreamReader) finish() {
	if !s.closed {
		latency := time.Since(s.startTime)
		s.router.ReportSuccess(s.deployment, &router.ResponseMetrics{
			Latency:          latency,
			TimeToFirstToken: s.ttft,
		})
		s.close()
	}
}
