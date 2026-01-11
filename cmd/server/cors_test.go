package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/config"
)

func TestCORSMiddleware_RejectsDisallowedOrigin(t *testing.T) {
	corsCfg := config.CORSConfig{
		Enabled:          true,
		AllowCredentials: true,
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		DataOrigins: config.CORSOrigins{
			Allowlist: []string{"https://app.example"},
		},
		AdminPathPrefixes: []string{"/key/"},
	}

	called := false
	handler := corsMiddleware(corsCfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost/v1/chat/completions", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("expected handler not to be called for disallowed origin")
	}
}

func TestCORSMiddleware_PreflightAllowed(t *testing.T) {
	corsCfg := config.CORSConfig{
		Enabled:          true,
		AllowCredentials: true,
		AllowMethods:     []string{"POST"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		MaxAge:           10 * time.Second,
		DataOrigins: config.CORSOrigins{
			Allowlist: []string{"https://app.example"},
		},
	}

	called := false
	handler := corsMiddleware(corsCfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "http://localhost/v1/chat/completions", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("allow-origin = %q, want %q", got, "https://app.example")
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow-credentials = %q, want %q", got, "true")
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "POST" {
		t.Fatalf("allow-methods = %q, want %q", got, "POST")
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("allow-headers = %q, want %q", got, "Content-Type, Authorization")
	}
	if called {
		t.Fatal("expected handler not to be called for preflight")
	}
}

func TestCORSMiddleware_AdminPolicy(t *testing.T) {
	corsCfg := config.CORSConfig{
		Enabled:          true,
		AllowCredentials: true,
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Authorization"},
		DataOrigins: config.CORSOrigins{
			Allowlist: []string{"https://app.example"},
		},
		AdminOrigins: config.CORSOrigins{
			Allowlist: []string{"https://admin.example"},
		},
		AdminPathPrefixes: []string{"/key/"},
	}

	handler := corsMiddleware(corsCfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost/key/list", nil)
	req.Header.Set("Origin", "https://admin.example")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example" {
		t.Fatalf("allow-origin = %q, want %q", got, "https://admin.example")
	}
}

func TestCORSMiddleware_DenylistWins(t *testing.T) {
	corsCfg := config.CORSConfig{
		Enabled:          true,
		AllowCredentials: true,
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Authorization"},
		DataOrigins: config.CORSOrigins{
			Allowlist: []string{"https://app.example"},
			Denylist:  []string{"https://app.example"},
		},
	}

	handler := corsMiddleware(corsCfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost/v1/models", nil)
	req.Header.Set("Origin", "https://app.example")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}
