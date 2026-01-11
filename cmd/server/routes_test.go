package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberrycongee/llmux/internal/config"
)

type fakeDataHandler struct{}

func (fakeDataHandler) HealthCheck(http.ResponseWriter, *http.Request)     {}
func (fakeDataHandler) ChatCompletions(http.ResponseWriter, *http.Request) {}
func (fakeDataHandler) Embeddings(http.ResponseWriter, *http.Request)      {}
func (fakeDataHandler) ListModels(http.ResponseWriter, *http.Request)      {}

type fakeManagementHandler struct{}

func (fakeManagementHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /key/list", func(http.ResponseWriter, *http.Request) {})
}

func TestBuildMuxes_AdminPortDisabled_RegistersAllOnDataMux(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 8080},
		Metrics: config.MetricsConfig{Enabled: true, Path: "/metrics"},
	}

	muxes, err := buildMuxes(cfg, fakeDataHandler{}, fakeManagementHandler{}, nil, nil)
	if err != nil {
		t.Fatalf("buildMuxes() error = %v", err)
	}

	if muxes.Admin != nil {
		t.Fatalf("expected no admin mux when admin_port is disabled")
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/chat/completions"); got != "POST /v1/chat/completions" {
		t.Fatalf("data mux missing chat route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodGet, "/key/list"); got != "GET /key/list" {
		t.Fatalf("data mux missing management route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodGet, "/metrics"); got != "GET /metrics" {
		t.Fatalf("data mux missing metrics route, got pattern %q", got)
	}
}

func TestBuildMuxes_AdminPortEnabled_SplitsRoutes(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 8080, AdminPort: 9090},
		Metrics: config.MetricsConfig{Enabled: true, Path: "/metrics"},
	}

	muxes, err := buildMuxes(cfg, fakeDataHandler{}, fakeManagementHandler{}, nil, nil)
	if err != nil {
		t.Fatalf("buildMuxes() error = %v", err)
	}

	if muxes.Admin == nil {
		t.Fatalf("expected admin mux when admin_port is enabled")
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/chat/completions"); got != "POST /v1/chat/completions" {
		t.Fatalf("data mux missing chat route, got pattern %q", got)
	}

	if got := routePattern(muxes.Admin, http.MethodPost, "/v1/chat/completions"); got != "" {
		t.Fatalf("admin mux should not have data routes, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodGet, "/key/list"); got != "" {
		t.Fatalf("data mux should not have management routes, got pattern %q", got)
	}

	if got := routePattern(muxes.Admin, http.MethodGet, "/key/list"); got != "GET /key/list" {
		t.Fatalf("admin mux missing management routes, got pattern %q", got)
	}
}

func routePattern(mux *http.ServeMux, method, path string) string {
	req := httptest.NewRequest(method, path, nil)
	_, pattern := mux.Handler(req)
	return pattern
}
