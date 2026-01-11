// Package main is the entry point for the LLMux gateway server.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/blueberrycongee/llmux/internal/secret"
	"github.com/blueberrycongee/llmux/internal/secret/env"
	"github.com/blueberrycongee/llmux/internal/secret/vault"
	"github.com/blueberrycongee/llmux/routers"
)

//go:embed all:ui_assets
var uiAssets embed.FS

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

	// Initialize Secret Manager
	secretManager := secret.NewManager()
	defer func() {
		if err := secretManager.Close(); err != nil {
			logger.Error("failed to close secret manager", "error", err)
		}
	}()

	// Register 'env' provider
	secretManager.Register("env", env.New())

	// Load configuration
	cfgManager, err := config.NewManager(*configPath, logger)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	defer func() { _ = cfgManager.Close() }()

	cfg := cfgManager.Get()

	// Register 'vault' provider if configured
	var vConfig vault.Config
	if cfg.Vault.Enabled {
		vConfig = vault.Config{
			Address:    cfg.Vault.Address,
			AuthMethod: cfg.Vault.AuthMethod,
			RoleID:     cfg.Vault.RoleID,
			SecretID:   cfg.Vault.SecretID,
			CACert:     cfg.Vault.CACert,
			ClientCert: cfg.Vault.ClientCert,
			ClientKey:  cfg.Vault.ClientKey,
		}
	} else if os.Getenv("VAULT_ADDR") != "" {
		// Backward compatibility: Construct from Env
		vConfig = vault.Config{
			Address:    os.Getenv("VAULT_ADDR"),
			AuthMethod: "approle", // Default for env var legacy
			RoleID:     os.Getenv("VAULT_ROLE_ID"),
			SecretID:   os.Getenv("VAULT_SECRET_ID"),
		}
	}

	if vConfig.Address != "" {
		logger.Info("initializing vault secret provider", "addr", vConfig.Address, "auth_method", vConfig.AuthMethod)
		vProvider, vErr := vault.New(vConfig)
		if vErr != nil {
			return fmt.Errorf("failed to initialize vault provider: %w", vErr)
		}
		// Wrap with cache (TTL 5 minutes)
		cachedVault := secret.NewCachedProvider(vProvider, 5*time.Minute)
		secretManager.Register("vault", cachedVault)
	} else {
		logger.Info("vault provider disabled")
	}

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
	opts := buildClientOptions(cfg, logger, secretManager)

	// Create llmux.Client
	client, err := llmux.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create llmux client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// ========================================================================
	// ENTERPRISE FEATURE INTEGRATION (P0 Fix)
	// Initialize auth stores, management handlers, and SSO sync
	// ========================================================================

	// Initialize auth Store (Memory or Postgres based on config)
	authStore, auditStore, err := initAuthStores(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize auth stores: %w", err)
	}

	// Ensure store is closed on shutdown
	defer func() {
		if authStore != nil {
			if closeErr := authStore.Close(); closeErr != nil {
				logger.Error("failed to close auth store", "error", closeErr)
			}
		}
	}()

	// Create AuditLogger
	auditLogger := auth.NewAuditLogger(auditStore, true)
	_ = auditLogger // Will be used by management handlers

	// Initialize UserTeamSyncer for SSO user-team synchronization
	var syncer *auth.UserTeamSyncer
	if cfg.Auth.Enabled && cfg.Auth.OIDC.UserTeamSync.Enabled {
		syncCfg := auth.UserTeamSyncConfig{
			Enabled:                 cfg.Auth.OIDC.UserTeamSync.Enabled,
			AutoCreateUsers:         cfg.Auth.OIDC.UserTeamSync.AutoCreateUsers,
			AutoCreateTeams:         cfg.Auth.OIDC.UserTeamSync.AutoCreateTeams,
			RemoveFromUnlistedTeams: cfg.Auth.OIDC.UserTeamSync.RemoveFromUnlistedTeams,
			SyncUserRole:            cfg.Auth.OIDC.UserTeamSync.SyncUserRole,
			DefaultRole:             cfg.Auth.OIDC.UserTeamSync.DefaultRole,
			DefaultOrganizationID:   cfg.Auth.OIDC.UserTeamSync.DefaultOrganizationID,
		}
		syncer = auth.NewUserTeamSyncer(authStore, syncCfg, logger)
		logger.Info("user-team sync enabled", "auto_create_users", syncCfg.AutoCreateUsers)
	}

	// Initialize API handler using ClientHandler (wraps llmux.Client)
	// Now with Store integration for usage logging and budget tracking
	handlerCfg := &api.ClientHandlerConfig{
		Store: authStore,
	}
	handler := api.NewClientHandler(client, logger, handlerCfg)

	// Initialize ManagementHandler for enterprise API endpoints
	mgmtHandler := api.NewManagementHandler(authStore, auditStore, logger)

	// Setup HTTP routes
	muxes, err := buildMuxes(cfg, handler, mgmtHandler, logger, uiAssets)
	if err != nil {
		return fmt.Errorf("failed to build routes: %w", err)
	}

	logger.Info("management endpoints registered",
		"endpoints", []string{"/key/*", "/team/*", "/user/*", "/organization/*", "/spend/*", "/audit/*"},
		"admin_port", cfg.Server.AdminPort,
	)

	middleware, err := buildMiddlewareStack(cfg, authStore, logger, syncer)
	if err != nil {
		return fmt.Errorf("failed to initialize middleware stack: %w", err)
	}

	dataHandler := middleware(muxes.Data)

	// Create data server
	dataServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      dataHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	var adminServer *http.Server
	if muxes.Admin != nil {
		adminHandler := middleware(muxes.Admin)
		adminServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.AdminPort),
			Handler:      adminHandler,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}
	}

	// Start server(s) in goroutines
	serverErr := make(chan error, 2)
	go func() {
		logger.Info("server listening", "port", cfg.Server.Port)
		if err := dataServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()
	if adminServer != nil {
		go func() {
			logger.Info("admin server listening", "port", cfg.Server.AdminPort)
			if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErr <- err
			}
		}()
	}

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

	if err := dataServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	if adminServer != nil {
		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("admin server shutdown error", "error", err)
		}
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

const routingRetryBackoff = 100 * time.Millisecond

// buildRoutingOptions converts routing-related config to llmux.Option slice.
func buildRoutingOptions(cfg *config.Config) []llmux.Option {
	opts := make([]llmux.Option, 0, 4)

	strategy := mapRoutingStrategy(cfg.Routing.Strategy)
	opts = append(opts, llmux.WithRouterStrategy(strategy))

	if cfg.Routing.CooldownPeriod > 0 {
		opts = append(opts, llmux.WithCooldown(cfg.Routing.CooldownPeriod))
	}

	if cfg.Server.WriteTimeout > 0 {
		opts = append(opts, llmux.WithTimeout(cfg.Server.WriteTimeout))
	}

	opts = append(opts,
		llmux.WithRetry(cfg.Routing.RetryCount, routingRetryBackoff),
		llmux.WithFallback(cfg.Routing.FallbackEnabled),
	)

	return opts
}

// buildClientOptions converts config.Config to llmux.Option slice.
func buildClientOptions(cfg *config.Config, logger *slog.Logger, secretManager *secret.Manager) []llmux.Option {
	// Pre-allocate with estimated capacity
	opts := make([]llmux.Option, 0, len(cfg.Providers)+6)

	// Add logger
	opts = append(opts, llmux.WithLogger(logger))

	// Add providers from config
	for _, provCfg := range cfg.Providers {
		pCfg := llmux.ProviderConfig{
			Name:    provCfg.Name,
			Type:    provCfg.Type,
			APIKey:  provCfg.APIKey,
			BaseURL: provCfg.BaseURL,
			Models:  provCfg.Models,
		}

		// Check if APIKey is a secret URI (contains "://")
		if strings.Contains(provCfg.APIKey, "://") {
			pCfg.TokenSource = &SecretTokenSource{
				mgr:  secretManager,
				path: provCfg.APIKey,
			}
		}

		opts = append(opts, llmux.WithProvider(pCfg))
	}

	opts = append(opts, buildRoutingOptions(cfg)...)

	// Set pricing file
	if cfg.PricingFile != "" {
		opts = append(opts, llmux.WithPricingFile(cfg.PricingFile))
	}

	// Stream recovery mode
	if cfg.Stream.RecoveryMode != "" {
		opts = append(opts, llmux.WithStreamRecoveryMode(mapStreamRecoveryMode(cfg.Stream.RecoveryMode)))
	}

	// Initialize distributed routing
	if cfg.Routing.Distributed {
		if cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0 {
			var redisClient *redis.Client

			if len(cfg.Cache.Redis.ClusterAddrs) > 0 {
				logger.Warn("Redis cluster mode not fully supported for routing stats, using single-node mode")
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.ClusterAddrs[0],
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			} else {
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.Addr,
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			}

			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Error("failed to connect to Redis for distributed routing", "error", err)
			} else {
				statsStore := routers.NewRedisStatsStore(redisClient)
				opts = append(opts, llmux.WithStatsStore(statsStore))
				logger.Info("distributed routing enabled", "redis_addr", cfg.Cache.Redis.Addr)
			}
		} else {
			logger.Warn("distributed routing enabled but no Redis configured")
		}
	}

	// Initialize distributed rate limiting
	if cfg.RateLimit.Enabled && cfg.RateLimit.Distributed {
		// Use Redis from Cache config for distributed rate limiting
		if cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0 {
			var redisClient *redis.Client

			if len(cfg.Cache.Redis.ClusterAddrs) > 0 {
				// For cluster mode, we need ClusterClient, but RedisLimiter uses *redis.Client
				// For now, use single node. Full cluster support requires updating RedisLimiter.
				logger.Warn("Redis cluster mode not fully supported for rate limiting, using single-node mode")
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.ClusterAddrs[0],
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			} else {
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.Addr,
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			}

			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Error("failed to connect to Redis for rate limiting", "error", err)
			} else {
				limiter := resilience.NewRedisLimiter(redisClient)
				opts = append(opts, llmux.WithRateLimiter(limiter))
				logger.Info("distributed rate limiting enabled",
					"redis_addr", cfg.Cache.Redis.Addr,
					"rpm_limit", cfg.RateLimit.RequestsPerMinute,
					"tpm_limit", cfg.RateLimit.TokensPerMinute,
				)
			}
		} else {
			logger.Warn("distributed rate limiting enabled but no Redis configured")
		}
	}

	// Set rate limiter config
	if cfg.RateLimit.Enabled {
		windowSize := cfg.RateLimit.WindowSize
		if windowSize == 0 {
			windowSize = time.Minute
		}
		opts = append(opts, llmux.WithRateLimiterConfig(llmux.RateLimiterConfig{
			Enabled:     cfg.RateLimit.Enabled,
			RPMLimit:    cfg.RateLimit.RequestsPerMinute,
			TPMLimit:    cfg.RateLimit.TokensPerMinute,
			WindowSize:  windowSize,
			KeyStrategy: mapKeyStrategy(cfg.RateLimit.KeyStrategy),
		}))
	}

	return opts
}

// mapStreamRecoveryMode converts config recovery mode to llmux.StreamRecoveryMode.
func mapStreamRecoveryMode(mode string) llmux.StreamRecoveryMode {
	switch mode {
	case "off":
		return llmux.StreamRecoveryOff
	case "append":
		return llmux.StreamRecoveryAppend
	case "retry":
		return llmux.StreamRecoveryRetry
	default:
		return llmux.StreamRecoveryRetry
	}
}

// mapKeyStrategy converts config key strategy string to llmux.RateLimitKeyStrategy.
func mapKeyStrategy(strategy string) llmux.RateLimitKeyStrategy {
	switch strategy {
	case "api_key":
		return llmux.RateLimitKeyByAPIKey
	case "user":
		return llmux.RateLimitKeyByUser
	case "model":
		return llmux.RateLimitKeyByModel
	case "api_key_model":
		return llmux.RateLimitKeyByAPIKeyAndModel
	default:
		return llmux.RateLimitKeyByAPIKey
	}
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

// SecretTokenSource adapts secret.Manager to provider.TokenSource interface.
type SecretTokenSource struct {
	mgr  *secret.Manager
	path string
}

// Token retrieves the secret value using the secret manager.
func (s *SecretTokenSource) Token() (string, error) {
	// Use background context as TokenSource interface doesn't support context
	return s.mgr.Get(context.Background(), s.path)
}

// corsMiddleware adds CORS headers for development (Next.js frontend on :3000)
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from Next.js dev server
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
