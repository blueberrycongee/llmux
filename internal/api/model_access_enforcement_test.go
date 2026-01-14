package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberrycongee/llmux/internal/auth"
)

func TestClientHandler_ModelAccessEnforcedWithoutGovernance(t *testing.T) {
	store := auth.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := NewClientHandlerWithSwapper(nil, logger, &ClientHandlerConfig{
		Store: store,
		// Governance intentionally nil to ensure enforcement does not depend on governance.enabled.
		Governance: nil,
	})

	apiKey := &auth.APIKey{
		ID:            "k1",
		KeyHash:       auth.HashKey("sk-test"),
		KeyPrefix:     "sk-test",
		IsActive:      true,
		AllowedModels: []string{"allowed-model"},
	}
	if err := store.CreateAPIKey(context.Background(), apiKey); err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}

	reqBody := []byte(`{"model":"blocked-model","messages":[{"role":"user","content":"hi"}]}`)
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	r = r.WithContext(auth.WithAuthContext(r.Context(), &auth.AuthContext{APIKey: apiKey}))

	w := httptest.NewRecorder()
	h.ChatCompletions(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
}
