package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDMiddleware_AcceptsReasonableClientID(t *testing.T) {
	var got string
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", http.NoBody)
	req.Header.Set(RequestIDHeader, "req-123_ABC.xyz")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got != "req-123_ABC.xyz" {
		t.Fatalf("expected request id to be preserved, got %q", got)
	}
	if rec.Header().Get(RequestIDHeader) != "req-123_ABC.xyz" {
		t.Fatalf("expected response header to be preserved, got %q", rec.Header().Get(RequestIDHeader))
	}
}

func TestRequestIDMiddleware_ReplacesOverlyLongOrInvalidID(t *testing.T) {
	var got string
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", http.NoBody)
	req.Header.Set(RequestIDHeader, strings.Repeat("a", 200))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got == strings.Repeat("a", 200) {
		t.Fatalf("expected request id to be replaced for long input")
	}
	if rec.Header().Get(RequestIDHeader) != got {
		t.Fatalf("expected response header to match context id")
	}
	if len(got) == 0 {
		t.Fatalf("expected generated request id")
	}
}

func TestRequestIDMiddleware_ReplacesCRLFInjectionAttempt(t *testing.T) {
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", http.NoBody)
	req.Header.Set(RequestIDHeader, "ok\r\nbad: x")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if strings.ContainsAny(rec.Header().Get(RequestIDHeader), "\r\n") {
		t.Fatalf("expected sanitized request id in response header")
	}
}
