package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberrycongee/llmux/internal/config"
)

type fakeDataHandler struct{}

func (fakeDataHandler) HealthCheck(http.ResponseWriter, *http.Request)         {}
func (fakeDataHandler) ChatCompletions(http.ResponseWriter, *http.Request)     {}
func (fakeDataHandler) Completions(http.ResponseWriter, *http.Request)         {}
func (fakeDataHandler) Embeddings(http.ResponseWriter, *http.Request)          {}
func (fakeDataHandler) ListModels(http.ResponseWriter, *http.Request)          {}
func (fakeDataHandler) Responses(http.ResponseWriter, *http.Request)           {}
func (fakeDataHandler) AudioTranscriptions(http.ResponseWriter, *http.Request) {}
func (fakeDataHandler) AudioTranslations(http.ResponseWriter, *http.Request)   {}
func (fakeDataHandler) AudioSpeech(http.ResponseWriter, *http.Request)         {}
func (fakeDataHandler) Batches(http.ResponseWriter, *http.Request)             {}

type fakeManagementHandler struct{}

func (fakeManagementHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /key/list", func(http.ResponseWriter, *http.Request) {})
}

type fakeInvitationHandler struct{}

func (fakeInvitationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /invitation/new", func(http.ResponseWriter, *http.Request) {})
}

func TestBuildMuxes_AdminPortDisabled_RegistersOnlyDataRoutes(t *testing.T) {
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

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/completions"); got != "POST /v1/completions" {
		t.Fatalf("data mux missing completions route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/responses"); got != "POST /v1/responses" {
		t.Fatalf("data mux missing responses route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/audio/transcriptions"); got != "POST /v1/audio/transcriptions" {
		t.Fatalf("data mux missing audio transcription route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/audio/translations"); got != "POST /v1/audio/translations" {
		t.Fatalf("data mux missing audio translation route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/audio/speech"); got != "POST /v1/audio/speech" {
		t.Fatalf("data mux missing audio speech route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/batches"); got != "POST /v1/batches" {
		t.Fatalf("data mux missing batch route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodGet, "/key/list"); got != "" {
		t.Fatalf("data mux should not have management routes, got pattern %q", got)
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

	muxes, err := buildMuxes(cfg, fakeDataHandler{}, multiRegistrar{fakeManagementHandler{}, fakeInvitationHandler{}}, nil, nil)
	if err != nil {
		t.Fatalf("buildMuxes() error = %v", err)
	}

	if muxes.Admin == nil {
		t.Fatalf("expected admin mux when admin_port is enabled")
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/chat/completions"); got != "POST /v1/chat/completions" {
		t.Fatalf("data mux missing chat route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/completions"); got != "POST /v1/completions" {
		t.Fatalf("data mux missing completions route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/responses"); got != "POST /v1/responses" {
		t.Fatalf("data mux missing responses route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/audio/transcriptions"); got != "POST /v1/audio/transcriptions" {
		t.Fatalf("data mux missing audio transcription route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/audio/translations"); got != "POST /v1/audio/translations" {
		t.Fatalf("data mux missing audio translation route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/audio/speech"); got != "POST /v1/audio/speech" {
		t.Fatalf("data mux missing audio speech route, got pattern %q", got)
	}

	if got := routePattern(muxes.Data, http.MethodPost, "/v1/batches"); got != "POST /v1/batches" {
		t.Fatalf("data mux missing batch route, got pattern %q", got)
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

	if got := routePattern(muxes.Admin, http.MethodPost, "/invitation/new"); got != "POST /invitation/new" {
		t.Fatalf("admin mux missing invitation routes, got pattern %q", got)
	}
}

func routePattern(mux *http.ServeMux, method, path string) string {
	req := httptest.NewRequest(method, path, nil)
	_, pattern := mux.Handler(req)
	return pattern
}
