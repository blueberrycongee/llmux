package streaming

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"
)

func TestForwarder_AllowsLargeSSELines(t *testing.T) {
	large := make([]byte, 32*1024)
	for i := range large {
		large[i] = 'a'
	}
	stream := append([]byte("data: "), large...)
	stream = append(stream, []byte("\n\n")...)
	stream = append(stream, []byte("data: [DONE]\n\n")...)

	rec := httptest.NewRecorder()
	fwd, err := NewForwarder(ForwarderConfig{
		Upstream:   ioNopCloser{r: bytes.NewReader(stream)},
		Downstream: rec,
		ClientCtx:  context.Background(),
	})
	if err != nil {
		t.Fatalf("NewForwarder() error = %v", err)
	}

	if err := fwd.Forward(); err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

type ioNopCloser struct {
	r *bytes.Reader
}

func (c ioNopCloser) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c ioNopCloser) Close() error               { return nil }
