package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

func TestManagementAuthzMiddleware_NonManagementPath_AllowsUnauthed(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: true}}
	h := managementAuthzMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestManagementAuthzMiddleware_ManagementPath_UnauthedDenied(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: true}}
	h := managementAuthzMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestManagementAuthzMiddleware_ManagementPath_BootstrapToken_Allows(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: true, BootstrapToken: "boot"}}
	h := managementAuthzMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("X-LLMux-Bootstrap-Token", "boot")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestManagementAuthzMiddleware_ManagementKey_Allows(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: true}}
	h := managementAuthzMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	authCtx := &auth.AuthContext{APIKey: &auth.APIKey{KeyType: auth.KeyTypeManagement}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, authCtx))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestManagementAuthzMiddleware_NonManagementKey_Denied(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: true}}
	h := managementAuthzMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	authCtx := &auth.AuthContext{APIKey: &auth.APIKey{KeyType: auth.KeyTypeLLMAPI}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, authCtx))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestManagementAuthzMiddleware_AuthDisabled_BootstrapToken_Allows(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: false, BootstrapToken: "boot"}}
	h := managementAuthzMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("X-LLMux-Bootstrap-Token", "boot")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestManagementBodyLimitMiddleware_TooLarge_Denied(t *testing.T) {
	h := managementBodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.Repeat([]byte("a"), int(maxManagementBodyBytes)+1)
	req := httptest.NewRequest(http.MethodPost, "/key/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestManagementBodyLimitMiddleware_NonManagementPath_AllowsLargeBody(t *testing.T) {
	h := managementBodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.Repeat([]byte("a"), int(maxManagementBodyBytes)+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}
