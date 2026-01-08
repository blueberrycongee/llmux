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

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/observability"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
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
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	defer func() { _ = cfgManager.Close() }()

	cfg := cfgManager.Get()

	// Initialize OpenTelemetry tracing
	tracingCfg := observability.TracingConfig{
		Enabled:     cfg.Tracing.Enabled,
		Endpoint:    cfg.Tracing.Endpoint,
		ServiceName: cfg.Tracing.ServiceName,
		SampleRate:  cfg.Tracing.SampleRate,
		Insecure:    cfg.Tracing.Insecure,
	}
	tracerProvider, err := observability.InitTracing(context.Background(), tracingCfg)
	if err != nil {
		logger.Error("failed to initialize tracing", "error", err)
	} else if cfg.Tracing.Enabled {
		logger.Info("tracing enabled", "endpoint", cfg.Tracing.Endpoint)
	}

	// Start config watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if watchErr := cfgManager.Watch(ctx); watchErr != nil {
		logger.Warn("config hot-reload disabled", "error", watchErr)
	}

	// Build llmux.Client options from config
	opts := buildClientOptions(cfg, logger)

	// Create llmux.Client
	client, err := llmux.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create llmux client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Initialize API handler using ClientHandler (wraps llmux.Client)
	handler := api.NewClientHandler(client, logger, nil)

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
	httpHandler = observability.RequestIDMiddleware(httpHandler)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for shutdown signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("shutting down server...")
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// Shutdown tracing
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			logger.Error("tracer shutdown error", "error", err)
		}
	}

	logger.Info("server stopped")
	return nil
}

// buildClientOptions converts config.Config to llmux.Option slice.
func buildClientOptions(cfg *config.Config, logger *slog.Logger) []llmux.Option {
	// Pre-allocate with estimated capacity
	opts := make([]llmux.Option, 0, len(cfg.Providers)+6)

	// Add logger
	opts = append(opts, llmux.WithLogger(logger))

	// Add providers from config
	for _, provCfg := range cfg.Providers {
		opts = append(opts, llmux.WithProvider(llmux.ProviderConfig{
			Name:    provCfg.Name,
			Type:    provCfg.Type,
			APIKey:  provCfg.APIKey,
			BaseURL: provCfg.BaseURL,
			Models:  provCfg.Models,
		}))
	}

	// Set routing strategy
	strategy := mapRoutingStrategy(cfg.Routing.Strategy)
	opts = append(opts, llmux.WithRouterStrategy(strategy))

	// Set cooldown period
	if cfg.Routing.CooldownPeriod > 0 {
		opts = append(opts, llmux.WithCooldown(cfg.Routing.CooldownPeriod))
	}

	// Set timeout
	if cfg.Server.WriteTimeout > 0 {
		opts = append(opts, llmux.WithTimeout(cfg.Server.WriteTimeout))
	}

	// Enable retry and fallback
	opts = append(opts,
		llmux.WithRetry(3, 100*time.Millisecond),
		llmux.WithFallback(true),
	)

	return opts
}

// mapRoutingStrategy converts config strategy string to llmux.Strategy.
func mapRoutingStrategy(strategy string) llmux.Strategy {
	switch strategy {
	case "shuffle", "random":
		return llmux.StrategyShuffle
	case "round-robin", "roundrobin":
		return llmux.StrategyRoundRobin
	case "lowest-latency", "latency":
		return llmux.StrategyLowestLatency
	case "least-busy", "leastbusy":
		return llmux.StrategyLeastBusy
	case "lowest-tpm-rpm", "tpm-rpm":
		return llmux.StrategyLowestTPMRPM
	case "lowest-cost", "cost":
		return llmux.StrategyLowestCost
	default:
		return llmux.StrategyShuffle
	}
}
