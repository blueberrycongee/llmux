package api //nolint:revive // package name is intentional

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientHandler_writeError_DoesNotLeakRawErrorMessage(t *testing.T) {
	h := &ClientHandler{
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}

	rr := httptest.NewRecorder()
	h.writeError(rr, errors.New("LEAKME: db password=secret"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if got := rr.Body.String(); strings.Contains(got, "LEAKME") || strings.Contains(got, "password") {
		t.Fatalf("response leaked internal error details: %s", got)
	}
}
