// Package streaming provides SSE (Server-Sent Events) streaming utilities.
// It handles efficient forwarding of streaming responses from LLM providers
// with buffer pooling and client disconnect detection.
package streaming

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"io"
	"net/http"
	"sync"

	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	// DefaultBufferSize is the default size for SSE buffers.
	DefaultBufferSize = 4096

	// SSEDataPrefix is the prefix for SSE data lines.
	SSEDataPrefix = "data: "

	// SSEDone is the marker for stream completion.
	SSEDone = "[DONE]"
)

// bufferPool provides reusable byte buffers to reduce GC pressure.
var bufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, DefaultBufferSize)
		return &buf
	},
}

// getBuffer retrieves a buffer from the pool.
func getBuffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

// putBuffer returns a buffer to the pool.
func putBuffer(buf *[]byte) {
	bufferPool.Put(buf)
}

// ChunkParser defines the interface for parsing provider-specific stream chunks.
type ChunkParser interface {
	// ParseChunk parses raw SSE data into a unified StreamChunk.
	// Returns nil, nil for keep-alive or non-content events.
	ParseChunk(data []byte) (*types.StreamChunk, error)
}

// Forwarder handles SSE stream forwarding from upstream to downstream.
type Forwarder struct {
	upstream   io.ReadCloser
	downstream http.ResponseWriter
	flusher    http.Flusher
	parser     ChunkParser
	ctx        context.Context
	cancel     context.CancelFunc
}

// ForwarderConfig contains configuration for the SSE forwarder.
type ForwarderConfig struct {
	Upstream   io.ReadCloser
	Downstream http.ResponseWriter
	Parser     ChunkParser
	ClientCtx  context.Context
}

// NewForwarder creates a new SSE forwarder.
func NewForwarder(cfg ForwarderConfig) (*Forwarder, error) {
	flusher, ok := cfg.Downstream.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	ctx, cancel := context.WithCancel(cfg.ClientCtx)

	return &Forwarder{
		upstream:   cfg.Upstream,
		downstream: cfg.Downstream,
		flusher:    flusher,
		parser:     cfg.Parser,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Forward streams data from upstream to downstream with transformation.
// It returns when the stream completes, an error occurs, or the client disconnects.
func (f *Forwarder) Forward() error {
	defer f.upstream.Close()

	// Set SSE headers
	f.downstream.Header().Set("Content-Type", "text/event-stream")
	f.downstream.Header().Set("Cache-Control", "no-cache")
	f.downstream.Header().Set("Connection", "keep-alive")
	f.downstream.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	scanner := bufio.NewScanner(f.upstream)
	buf := getBuffer()
	defer putBuffer(buf)
	scanner.Buffer(*buf, DefaultBufferSize*4)

	for scanner.Scan() {
		// Check for client disconnect
		select {
		case <-f.ctx.Done():
			return f.ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if err := f.processLine(line); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func (f *Forwarder) processLine(line []byte) error {
	// Skip empty lines (SSE keep-alive)
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil
	}

	// Check for stream end marker
	if bytes.Equal(trimmed, []byte(SSEDataPrefix+SSEDone)) ||
		bytes.Equal(trimmed, []byte(SSEDone)) {
		f.writeLine([]byte(SSEDataPrefix + SSEDone))
		f.writeLine(nil) // Empty line to complete SSE event
		f.flusher.Flush()
		return nil
	}

	// Parse and transform the chunk if parser is provided
	if f.parser != nil {
		chunk, err := f.parser.ParseChunk(trimmed)
		if err != nil {
			// Log error but continue streaming
			return nil
		}
		if chunk != nil {
			return f.writeChunk(chunk)
		}
		// Non-content event, forward as-is for compatibility
	}

	// Forward raw line
	f.writeLine(line)
	f.writeLine(nil)
	f.flusher.Flush()
	return nil
}

func (f *Forwarder) writeChunk(chunk *types.StreamChunk) error {
	data, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal chunk: %w", err)
	}

	f.writeLine(append([]byte(SSEDataPrefix), data...))
	f.writeLine(nil) // Empty line to complete SSE event
	f.flusher.Flush()
	return nil
}

func (f *Forwarder) writeLine(line []byte) {
	if line == nil {
		f.downstream.Write([]byte("\n"))
		return
	}
	f.downstream.Write(line)
	f.downstream.Write([]byte("\n"))
}

// Close cancels the forwarding and releases resources.
func (f *Forwarder) Close() {
	f.cancel()
	f.upstream.Close()
}
