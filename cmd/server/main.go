// Package main is the entry point for the LLMux gateway server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/internal/provider/anthropic"
	"github.com/blueberrycongee/llmux/internal/provider/azure"
	"github.com/blueberrycongee/llmux/internal/provider/gemini"
	"github.com/blueberrycongee/llmux/internal/provider/openai"
	"github.com/blueberrycongee/llmux/internal/router"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to configuration file")
	flag.Parse()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting LLMux gateway", "version", "0.1.0")

	// Load configuration
	cfgManager, err := config.NewManager(*configPath, logger)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	cfg := cfgManager.Get()

	// Start config watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cfgManager.Watch(ctx); err != nil {
		logger.Warn("config hot-reload disabled", "error", err)
	}

	// Initialize provider registry
	registry := provider.NewRegistry()
	registry.RegisterFactory("openai", openai.New)
	registry.RegisterFactory("anthropic", anthropic.New)
	registry.RegisterFactory("azure", azure.New)
	registry.RegisterFactory("gemini", gemini.New)

	// Create providers from config
	for _, provCfg := range cfg.Providers {
		pCfg := provider.ProviderConfig{
			Name:          provCfg.Name,
			Type:          provCfg.Type,
			APIKey:        provCfg.APIKey,
			BaseURL:       provCfg.BaseURL,
			Models:        provCfg.Models,
			MaxConcurrent: provCfg.MaxConcurrent,
			TimeoutSec:    int(provCfg.Timeout.Seconds()),
		}

		prov, err := registry.CreateProvider(pCfg)
		if err != nil {
			logger.Error("failed to create provider", "name", provCfg.Name, "error", err)
			continue
		}
		logger.Info("provider registered", "name", prov.Name(), "models", provCfg.Models)
	}

	// Initialize router
	simpleRouter := router.NewSimpleRouter(cfg.Routing.CooldownPeriod)

	// Register deployments
	for _, provCfg := range cfg.Providers {
		for _, model := range provCfg.Models {
			deployment := &provider.Deployment{
				ID:            fmt.Sprintf("%s-%s", provCfg.Name, model),
				ProviderName:  provCfg.Name,
				ModelName:     model,
				BaseURL:       provCfg.BaseURL,
				APIKey:        provCfg.APIKey,
				MaxConcurrent: provCfg.MaxConcurrent,
				Timeout:       int(provCfg.Timeout.Seconds()),
			}
			simpleRouter.AddDeployment(deployment)
		}
	}

	// Initialize API handler
	handler := api.NewHandler(registry, simpleRouter, logger)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("GET /health/live", handler.HealthCheck)
	mux.HandleFunc("GET /health/ready", handler.HealthCheck)

	// OpenAI-compatible endpoints
	mux.HandleFunc("POST /v1/chat/completions", handler.ChatCompletions)
	mux.HandleFunc("GET /v1/models", handler.ListModels)

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		mux.Handle("GET "+cfg.Metrics.Path, promhttp.Handler())
	}

	// Apply middleware
	var httpHandler http.Handler = mux
	httpHandler = metrics.Middleware(httpHandler)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server listening", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	cfgManager.Close()
	logger.Info("server stopped")
}
