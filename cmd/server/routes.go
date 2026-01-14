package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/blueberrycongee/llmux/internal/config"
)

type dataHandler interface {
	HealthCheck(http.ResponseWriter, *http.Request)
	ChatCompletions(http.ResponseWriter, *http.Request)
	Completions(http.ResponseWriter, *http.Request)
	Embeddings(http.ResponseWriter, *http.Request)
	ListModels(http.ResponseWriter, *http.Request)
	Responses(http.ResponseWriter, *http.Request)
	AudioTranscriptions(http.ResponseWriter, *http.Request)
	AudioTranslations(http.ResponseWriter, *http.Request)
	AudioSpeech(http.ResponseWriter, *http.Request)
	Batches(http.ResponseWriter, *http.Request)
}

type managementRegistrar interface {
	RegisterRoutes(*http.ServeMux)
}

type multiRegistrar []managementRegistrar

func (m multiRegistrar) RegisterRoutes(mux *http.ServeMux) {
	for _, registrar := range m {
		if registrar == nil {
			continue
		}
		registrar.RegisterRoutes(mux)
	}
}

type muxes struct {
	Data  *http.ServeMux
	Admin *http.ServeMux
}

var errNilConfig = errors.New("config is required")

func buildMuxes(cfg *config.Config, handler dataHandler, mgmtHandler managementRegistrar, logger *slog.Logger, uiAssets fs.FS) (muxes, error) {
	if cfg == nil {
		return muxes{}, errNilConfig
	}

	dataMux := http.NewServeMux()
	registerDataRoutes(dataMux, handler, cfg)

	if cfg.Server.AdminPort > 0 {
		adminMux := http.NewServeMux()
		if mgmtHandler != nil {
			registerAdminRoutes(adminMux, mgmtHandler, logger, uiAssets, true)
		}
		return muxes{Data: dataMux, Admin: adminMux}, nil
	}

	return muxes{Data: dataMux}, nil
}

func registerDataRoutes(mux *http.ServeMux, handler dataHandler, cfg *config.Config) {
	if handler == nil || mux == nil {
		return
	}

	// Health endpoints
	mux.HandleFunc("GET /health/live", handler.HealthCheck)
	mux.HandleFunc("GET /health/ready", handler.HealthCheck)

	// OpenAI-compatible endpoints
	mux.HandleFunc("POST /v1/chat/completions", handler.ChatCompletions)
	mux.HandleFunc("POST /v1/completions", handler.Completions)
	mux.HandleFunc("POST /v1/embeddings", handler.Embeddings)
	mux.HandleFunc("POST /embeddings", handler.Embeddings)
	mux.HandleFunc("POST /v1/responses", handler.Responses)
	mux.HandleFunc("POST /v1/audio/transcriptions", handler.AudioTranscriptions)
	mux.HandleFunc("POST /v1/audio/translations", handler.AudioTranslations)
	mux.HandleFunc("POST /v1/audio/speech", handler.AudioSpeech)
	mux.HandleFunc("POST /v1/batches", handler.Batches)
	mux.HandleFunc("GET /v1/models", handler.ListModels)

	// Metrics endpoint
	if cfg != nil && cfg.Metrics.Enabled {
		mux.Handle("GET "+cfg.Metrics.Path, promhttp.Handler())
	}
}

func registerAdminRoutes(mux *http.ServeMux, mgmtHandler managementRegistrar, logger *slog.Logger, uiAssets fs.FS, enableUI bool) {
	if mux == nil || mgmtHandler == nil {
		return
	}

	mgmtHandler.RegisterRoutes(mux)

	if !enableUI || uiAssets == nil {
		return
	}

	uiFS, err := fs.Sub(uiAssets, "ui_assets")
	if err != nil {
		if logger != nil {
			logger.Error("failed to load UI assets", "error", err)
		}
		return
	}

	// Serve UI at root
	mux.Handle("/", http.FileServer(http.FS(uiFS)))
}
