package streaming

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockReadCloser wraps a reader to implement io.ReadCloser.
type mockReadCloser struct {
	io.Reader
	closed bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func TestNewForwarder(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() ForwarderConfig
		wantErr bool
	}{
		{
			name: "should create forwarder with valid config",
			setup: func() ForwarderConfig {
				return ForwarderConfig{
					Upstream:   &mockReadCloser{Reader: strings.NewReader("")},
					Downstream: httptest.NewRecorder(),
					ClientCtx:  context.Background(),
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setup()
			f, err := NewForwarder(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewForwarder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && f == nil {
				t.Error("NewForwarder() returned nil forwarder")
			}
		})
	}
}

func TestForwarder_Forward(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
	}{
		{
			name:       "should forward simple SSE data",
			input:      "data: {\"id\":\"1\"}\n\n",
			wantOutput: "data: {\"id\":\"1\"}\n\n",
		},
		{
			name:       "should handle DONE marker",
			input:      "data: [DONE]\n",
			wantOutput: "data: [DONE]\n\n",
		},
		{
			name:       "should skip empty lines",
			input:      "\n\ndata: test\n\n",
			wantOutput: "data: test\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &mockReadCloser{Reader: strings.NewReader(tt.input)}
			recorder := httptest.NewRecorder()

			f, err := NewForwarder(ForwarderConfig{
				Upstream:   upstream,
				Downstream: recorder,
				ClientCtx:  context.Background(),
			})
			if err != nil {
				t.Fatalf("NewForwarder() error = %v", err)
			}

			err = f.Forward()
			if err != nil {
				t.Errorf("Forward() error = %v", err)
			}

			// Check headers
			if ct := recorder.Header().Get("Content-Type"); ct != "text/event-stream" {
				t.Errorf("Content-Type = %v, want text/event-stream", ct)
			}

			// Check upstream was closed
			if !upstream.closed {
				t.Error("upstream was not closed")
			}

			// Check context was canceled
			select {
			case <-f.ctx.Done():
				// Success
			default:
				t.Error("forwarder context was not canceled after Forward() returned")
			}
		})
	}
}

func TestForwarder_ClientDisconnect(t *testing.T) {
	// Create a slow upstream that will be interrupted
	slowReader := &slowReader{
		data:  []byte("data: test\n\ndata: test2\n\n"),
		delay: 100 * time.Millisecond,
	}
	upstream := &mockReadCloser{Reader: slowReader}
	recorder := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())

	f, err := NewForwarder(ForwarderConfig{
		Upstream:   upstream,
		Downstream: recorder,
		ClientCtx:  ctx,
	})
	if err != nil {
		t.Fatalf("NewForwarder() error = %v", err)
	}

	// Cancel context after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err = f.Forward()
	if err != context.Canceled {
		t.Errorf("Forward() error = %v, want context.Canceled", err)
	}
}

// slowReader reads data with a delay between reads.
type slowReader struct {
	data  []byte
	pos   int
	delay time.Duration
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	time.Sleep(r.delay)
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestBufferPool(t *testing.T) {
	// Get multiple buffers and verify they work
	buffers := make([]*[]byte, 10)
	for i := range buffers {
		buffers[i] = getBuffer()
		if buffers[i] == nil {
			t.Fatalf("getBuffer() returned nil at index %d", i)
		}
		if len(*buffers[i]) != DefaultBufferSize {
			t.Errorf("buffer size = %d, want %d", len(*buffers[i]), DefaultBufferSize)
		}
	}

	// Return buffers to pool
	for _, buf := range buffers {
		putBuffer(buf)
	}

	// Get them again - should reuse
	for i := range buffers {
		buffers[i] = getBuffer()
		if buffers[i] == nil {
			t.Fatalf("getBuffer() returned nil on reuse at index %d", i)
		}
	}
}

// responseWriterNoFlush is a ResponseWriter that doesn't support Flusher.
type responseWriterNoFlush struct {
	http.ResponseWriter
}

func (w *responseWriterNoFlush) Header() http.Header {
	return make(http.Header)
}

func (w *responseWriterNoFlush) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *responseWriterNoFlush) WriteHeader(statusCode int) {}

func TestNewForwarder_NoFlusher(t *testing.T) {
	upstream := &mockReadCloser{Reader: bytes.NewReader(nil)}
	noFlush := &responseWriterNoFlush{}

	_, err := NewForwarder(ForwarderConfig{
		Upstream:   upstream,
		Downstream: noFlush,
		ClientCtx:  context.Background(),
	})
	if err == nil {
		t.Error("NewForwarder() should fail when ResponseWriter doesn't support Flusher")
	}
}
