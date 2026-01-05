package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("expected non-empty request ID")
	}
	if id1 == id2 {
		t.Error("expected unique request IDs")
	}
	if len(id1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("expected 32 char request ID, got %d", len(id1))
	}
}

func TestContextWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	ctx = ContextWithRequestID(ctx, requestID)
	extracted := RequestIDFromContext(ctx)

	if extracted != requestID {
		t.Errorf("expected %q, got %q", requestID, extracted)
	}
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	extracted := RequestIDFromContext(ctx)

	if extracted != "" {
		t.Errorf("expected empty string, got %q", extracted)
	}
}

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	var capturedID string

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check context had request ID
	if capturedID == "" {
		t.Error("expected request ID in context")
	}

	// Check response header
	responseID := rec.Header().Get(RequestIDHeader)
	if responseID == "" {
		t.Error("expected request ID in response header")
	}
	if responseID != capturedID {
		t.Error("response header should match context ID")
	}
}

func TestRequestIDMiddleware_PreservesExisting(t *testing.T) {
	existingID := "existing-request-id-123"
	var capturedID string

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("expected preserved ID %q, got %q", existingID, capturedID)
	}

	responseID := rec.Header().Get(RequestIDHeader)
	if responseID != existingID {
		t.Errorf("expected response header %q, got %q", existingID, responseID)
	}
}

func TestGetOrCreateRequestID_Existing(t *testing.T) {
	existingID := "existing-id"
	ctx := ContextWithRequestID(context.Background(), existingID)

	newCtx, id := GetOrCreateRequestID(ctx)

	if id != existingID {
		t.Errorf("expected existing ID %q, got %q", existingID, id)
	}
	if RequestIDFromContext(newCtx) != existingID {
		t.Error("context should have existing ID")
	}
}

func TestGetOrCreateRequestID_New(t *testing.T) {
	ctx := context.Background()

	newCtx, id := GetOrCreateRequestID(ctx)

	if id == "" {
		t.Error("expected generated ID")
	}
	if RequestIDFromContext(newCtx) != id {
		t.Error("context should have generated ID")
	}
}
