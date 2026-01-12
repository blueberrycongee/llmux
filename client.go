package llmux

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/blueberrycongee/llmux/internal/tokenizer"
	"github.com/blueberrycongee/llmux/pkg/cache"
	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/pricing"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/blueberrycongee/llmux/providers"
	"github.com/blueberrycongee/llmux/routers"
)

// Client is the main entry point for LLMux library mode.
// It manages providers, routing, caching, and request execution.
//
// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	providers      map[string]provider.Provider
	deployments    map[string][]*provider.Deployment // model -> deployments
	router         router.Router
	cache          cache.Cache
	cacheTypeLabel string
	httpClient     *http.Client
	logger         *slog.Logger
	config         *ClientConfig
	pricing        *pricing.Registry
	pipeline       *plugin.Pipeline

	// Provider factories for creating providers from config
	factories map[string]provider.Factory

	// Distributed rate limiting
	rateLimiter       resilience.DistributedLimiter
	rateLimiterConfig RateLimiterConfig

	// Object pools for performance
	requestPool  sync.Pool
	responsePool sync.Pool

	mu sync.RWMutex
}

// New creates a new LLMux client with the given options.
//
// Example:
//
//	client, err := llmux.New(
//	    llmux.WithProvider(llmux.ProviderConfig{
//	        Name:   "openai",
//	        Type:   "openai",
//	        APIKey: os.Getenv("OPENAI_API_KEY"),
//	        Models: []string{"gpt-4o"},
//	    }),
//	    llmux.WithRouterStrategy(llmux.StrategyLowestLatency),
//	)
func New(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	c := &Client{
		providers:   make(map[string]provider.Provider),
		deployments: make(map[string][]*provider.Deployment),
		factories:   make(map[string]provider.Factory),
		config:      cfg,
		logger:      cfg.Logger,
		pricing:     pricing.NewRegistry(),
		requestPool: sync.Pool{
			New: func() any { return new(types.ChatRequest) },
		},
		responsePool: sync.Pool{
			New: func() any { return new(types.ChatResponse) },
		},
	}

	// Initialize HTTP client with connection pooling
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: cfg.Timeout,
	}

	// Register built-in provider factories
	c.registerBuiltinFactories()

	// Load custom pricing if provided
	if cfg.PricingFile != "" {
		if err := c.pricing.Load(cfg.PricingFile); err != nil {
			return nil, fmt.Errorf("load pricing file: %w", err)
		}
	}

	// Initialize providers from config
	for i := range cfg.Providers {
		pcfg := cfg.Providers[i]
		if err := c.addProviderFromConfig(pcfg); err != nil {
			return nil, fmt.Errorf("add provider %s: %w", pcfg.Name, err)
		}
	}

	// Add pre-configured provider instances
	for _, inst := range cfg.ProviderInstances {
		if err := c.addProviderInstance(inst.Name, inst.Provider, inst.Models); err != nil {
			return nil, fmt.Errorf("add provider instance %s: %w", inst.Name, err)
		}
	}

	// Initialize router
	if cfg.Router != nil {
		c.router = cfg.Router
	} else {
		c.router = c.createRouter(cfg.RouterStrategy)
	}

	// Register deployments with router
	for _, deployments := range c.deployments {
		for _, d := range deployments {
			c.router.AddDeployment(d)
		}
	}

	// Initialize cache
	if cfg.CacheEnabled && cfg.Cache != nil {
		c.cache = cfg.Cache
		c.cacheTypeLabel = cfg.CacheTypeLabel
	}

	// Initialize distributed rate limiter
	c.rateLimiterConfig = cfg.RateLimiterConfig
	if cfg.RateLimiter != nil {
		c.rateLimiter = cfg.RateLimiter
	}

	c.logger.Info("llmux client initialized",
		"providers", len(c.providers),
		"strategy", cfg.RouterStrategy,
		"cache_enabled", cfg.CacheEnabled,
		"ratelimiter_enabled", cfg.RateLimiterConfig.Enabled,
	)

	// Initialize plugin pipeline
	pipelineConfig := plugin.DefaultPipelineConfig()
	if cfg.PluginConfig != nil {
		pipelineConfig = *cfg.PluginConfig
	}
	c.pipeline = plugin.NewPipeline(c.logger, pipelineConfig)

	// Register plugins
	// Initialize Observability Plugin if enabled
	if cfg.OTelMetricsConfig.Enabled {
		metrics, err := observability.InitOTelMetrics(context.Background(), cfg.OTelMetricsConfig)
		if err != nil {
			c.logger.Warn("failed to initialize OTel metrics", "error", err)
		} else {
			redactor := observability.NewRedactor() // Use default redactor
			obsPlugin := observability.NewObservabilityPlugin(redactor, metrics)
			if err := c.pipeline.Register(obsPlugin); err != nil {
				c.logger.Warn("failed to register observability plugin", "error", err)
			}
		}
	}

	// Register user-defined plugins
	for _, p := range cfg.Plugins {
		if err := c.pipeline.Register(p); err != nil {
			return nil, fmt.Errorf("register plugin %s: %w", p.Name(), err)
		}
	}
	c.logger.Info("plugin pipeline initialized", "plugins", c.pipeline.PluginCount())

	return c, nil
}

// ChatCompletion sends a chat completion request.
// It handles routing, retries, fallback, and caching automatically.
func (c *Client) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages is required")
	}

	// Get plugin context
	pCtx := c.pipeline.GetContext(ctx, generateRequestID())
	defer c.pipeline.PutContext(pCtx)

	// Run PreHooks
	req, sc, _ := c.pipeline.RunPreHooks(pCtx, req)
	if sc != nil {
		// Short-circuit
		if sc.Error != nil {
			if !sc.AllowFallback {
				return nil, sc.Error
			}
			// Fallback logic could go here, but for now just return error
			return nil, sc.Error
		}
		if sc.Response != nil {
			// Run PostHooks even on short-circuit (e.g., for logging)
			finalResp, hookErr := c.pipeline.RunPostHooks(pCtx, sc.Response, nil, c.pipeline.PluginCount())
			if hookErr != nil {
				c.logger.Warn("PostHook error during short-circuit", "error", hookErr)
			}
			return finalResp, nil
		}
	}

	// Check rate limit before processing request
	apiKey := c.rateLimitAPIKey(ctx)
	rateLimitKey := c.buildRateLimitKey(req.Model, req.User, apiKey)
	promptEstimate := tokenizer.EstimatePromptTokens(req.Model, req)
	estimatedTokens := promptEstimate
	if req.MaxTokens > 0 {
		estimatedTokens += req.MaxTokens
	}
	if err := c.checkRateLimit(ctx, rateLimitKey, req.Model, estimatedTokens); err != nil {
		return nil, err
	}

	var resp *ChatResponse
	var err error

	// Check cache for non-streaming requests (if not handled by plugin)
	// Note: Ideally cache should be a plugin, but keeping this for backward compatibility
	// or if built-in cache is preferred.
	if c.cache != nil && !req.Stream {
		if cached, cacheErr := c.getFromCache(ctx, req); cacheErr == nil && cached != nil {
			resp = cached
		}
	}

	if resp != nil {
		provider := ""
		if resp.Usage != nil {
			provider = resp.Usage.Provider
		}
		if pricingErr := c.validatePricing(req.Model, provider); pricingErr != nil {
			return nil, pricingErr
		}
	}

	if resp == nil {
		// Route to deployment
		var deployment *provider.Deployment
		reqCtx := buildRouterRequestContext(req, promptEstimate, req.Stream)
		deployment, err = c.router.PickWithContext(ctx, reqCtx)
		if err != nil {
			err = fmt.Errorf("no available deployment for model %s: %w", req.Model, err)
		} else {
			// Get provider
			c.mu.RLock()
			prov, ok := c.providers[deployment.ProviderName]
			c.mu.RUnlock()
			if !ok {
				err = fmt.Errorf("provider %s not found", deployment.ProviderName)
			} else {
				// Execute with retry
				resp, err = c.executeWithRetry(ctx, prov, deployment, req)
			}
		}
	}

	// Run PostHooks
	// We pass the number of plugins that ran in PreHook phase (all of them if no short-circuit)
	runFrom := c.pipeline.PluginCount()
	finalResp, finalErr := c.pipeline.RunPostHooks(pCtx, resp, err, runFrom)

	if finalErr == nil && finalResp != nil {
		// Store in cache if successful and not streaming
		if c.cache != nil && !req.Stream {
			c.storeInCache(ctx, req, finalResp)
		}
	}

	return finalResp, finalErr
}

// ChatCompletionStream sends a streaming chat completion request.
// Returns a StreamReader that can be used to iterate over response chunks.
//
// Example:
//
//	stream, err := client.ChatCompletionStream(ctx, req)
//	if err != nil {
//	    return err
//	}
//	defer stream.Close()
//
//	for {
//	    chunk, err := stream.Recv()
//	    if err == io.EOF {
//	        break
//	    }
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	}
func (c *Client) ChatCompletionStream(ctx context.Context, req *ChatRequest) (*StreamReader, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages is required")
	}

	req.Stream = true

	// Get plugin context
	pCtx := c.pipeline.GetContext(ctx, generateRequestID())
	pCtx.IsStreaming = true
	pCtx.Model = req.Model

	// Run Stream PreHooks
	req, sc, runFrom := c.pipeline.RunStreamPreHooks(pCtx, req)
	pCtx.Model = req.Model
	if sc != nil {
		if sc.Error != nil {
			_ = c.pipeline.RunStreamPostHooks(pCtx, sc.Error, runFrom)
			c.pipeline.PutContext(pCtx)
			return nil, sc.Error
		}
		if sc.Stream != nil {
			return newStreamReaderFromChannel(ctx, c, req, sc.Stream, c.pipeline, pCtx, runFrom), nil
		}
	}

	// Check rate limit before processing request
	apiKey := c.rateLimitAPIKey(ctx)
	rateLimitKey := c.buildRateLimitKey(req.Model, req.User, apiKey)
	promptEstimate := tokenizer.EstimatePromptTokens(req.Model, req)
	estimatedTokens := promptEstimate
	if req.MaxTokens > 0 {
		estimatedTokens += req.MaxTokens
	}
	if err := c.checkRateLimit(ctx, rateLimitKey, req.Model, estimatedTokens); err != nil {
		c.pipeline.PutContext(pCtx)
		return nil, err
	}

	var lastErr error
	var deployment *provider.Deployment

	// Retry loop
	for attempt := 0; attempt <= c.config.RetryCount; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := c.config.RetryBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				c.pipeline.PutContext(pCtx)
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Route to deployment
		// Pick a new deployment if:
		// 1. It's the first attempt
		// 2. Fallback is enabled (try to find a healthy node)
		// 3. We don't have a deployment yet (e.g. previous pick failed)
		if attempt == 0 || c.config.FallbackEnabled || deployment == nil {
			reqCtx := buildRouterRequestContext(req, promptEstimate, true)
			newDeployment, err := c.router.PickWithContext(ctx, reqCtx)
			if err != nil {
				lastErr = fmt.Errorf("no available deployment for model %s: %w", req.Model, err)
				// If we can't pick a deployment and we don't have one from before, we can't proceed
				if deployment == nil {
					continue
				}
				// If pick failed but we have a previous deployment and fallback is disabled,
				// we might technically retry on the old one, but usually Pick failure means
				// global exhaustion or router issues.
				// For safety, if Pick fails, we count it as a retry failure.
				continue
			}
			deployment = newDeployment
		}

		// Get provider
		c.mu.RLock()
		prov, ok := c.providers[deployment.ProviderName]
		c.mu.RUnlock()
		if !ok {
			lastErr = fmt.Errorf("provider %s not found", deployment.ProviderName)
			continue
		}

		if err := c.validatePricing(req.Model, deployment.ProviderName); err != nil {
			c.pipeline.PutContext(pCtx)
			return nil, err
		}

		// Build and execute request
		httpReq, err := prov.BuildRequest(ctx, sanitizeChatRequestForProvider(req))
		if err != nil {
			c.pipeline.PutContext(pCtx)
			return nil, fmt.Errorf("build request: %w", err)
		}

		c.router.ReportRequestStart(deployment)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			c.router.ReportFailure(deployment, err)
			c.router.ReportRequestEnd(deployment)
			lastErr = fmt.Errorf("execute request: %w", err)
			continue
		}

		if resp.StatusCode >= 500 {
			// Server error, retryable
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			llmErr := prov.MapError(resp.StatusCode, body)
			c.router.ReportFailure(deployment, llmErr)
			c.router.ReportRequestEnd(deployment)
			lastErr = llmErr
			continue
		}

		if resp.StatusCode >= 400 {
			// Client error
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			llmErr := prov.MapError(resp.StatusCode, body)

			// Check if it's a retryable client error (e.g. 429 Rate Limit)
			if llmErr, ok := llmErr.(*LLMError); ok && llmErr.Retryable {
				c.router.ReportFailure(deployment, llmErr)
				c.router.ReportRequestEnd(deployment)
				lastErr = llmErr
				continue
			}

			// Non-retryable error
			c.router.ReportFailure(deployment, llmErr)
			c.router.ReportRequestEnd(deployment)
			c.pipeline.PutContext(pCtx)
			return nil, llmErr
		}

		pCtx.Provider = deployment.ProviderName
		return newStreamReader(ctx, c, req, resp.Body, prov, deployment, c.router, c.pipeline, pCtx, runFrom), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("max retries exceeded")
	}
	c.pipeline.PutContext(pCtx)
	return nil, lastErr
}

// Embedding sends an embedding request.
// It handles routing, retries, and fallback automatically.
func (c *Client) Embedding(ctx context.Context, req *types.EmbeddingRequest) (*types.EmbeddingResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Check rate limit before processing request
	apiKey := c.rateLimitAPIKey(ctx)
	rateLimitKey := c.buildRateLimitKey(req.Model, req.User, apiKey)
	promptEstimate := tokenizer.EstimateEmbeddingTokens(req.Model, req)
	if err := c.checkRateLimit(ctx, rateLimitKey, req.Model, promptEstimate); err != nil {
		return nil, err
	}

	var lastErr error
	var deployment *provider.Deployment

	// Retry loop
	for attempt := 0; attempt <= c.config.RetryCount; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := c.config.RetryBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Route to deployment
		if attempt == 0 || c.config.FallbackEnabled || deployment == nil {
			newDeployment, err := c.router.Pick(ctx, req.Model)
			if err != nil {
				lastErr = fmt.Errorf("no available deployment for model %s: %w", req.Model, err)
				if deployment == nil {
					continue
				}
				continue
			}
			deployment = newDeployment
		}

		// Get provider
		c.mu.RLock()
		prov, ok := c.providers[deployment.ProviderName]
		c.mu.RUnlock()
		if !ok {
			lastErr = fmt.Errorf("provider %s not found", deployment.ProviderName)
			continue
		}

		// Check if provider supports embedding
		if !prov.SupportEmbedding() {
			lastErr = fmt.Errorf("provider %s does not support embeddings", deployment.ProviderName)
			continue
		}

		// Execute request
		resp, err := c.executeEmbeddingOnce(ctx, prov, deployment, req, promptEstimate)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if llmErr, ok := err.(*errors.LLMError); ok && !llmErr.Retryable {
			return nil, err
		}

		// Also check provider-level error mapping if it returns a different error type
		// For now, we assume executeEmbeddingOnce returns standardized errors or wrapped errors
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("max retries exceeded")
	}
	return nil, lastErr
}

func (c *Client) executeEmbeddingOnce(
	ctx context.Context,
	prov provider.Provider,
	deployment *provider.Deployment,
	req *types.EmbeddingRequest,
	promptEstimate int,
) (*types.EmbeddingResponse, error) {
	start := time.Now()

	if err := c.validatePricing(req.Model, deployment.ProviderName); err != nil {
		return nil, err
	}

	httpReq, err := prov.BuildEmbeddingRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	c.router.ReportRequestStart(deployment)
	defer c.router.ReportRequestEnd(deployment)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.router.ReportFailure(deployment, err)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		llmErr := prov.MapError(resp.StatusCode, body)
		c.router.ReportFailure(deployment, llmErr)
		return nil, llmErr
	}

	embResp, err := prov.ParseEmbeddingResponse(resp)
	if err != nil {
		c.router.ReportFailure(deployment, err)
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if embResp.Usage.TotalTokens == 0 && embResp.Usage.PromptTokens == 0 {
		embResp.Usage.PromptTokens = promptEstimate
		embResp.Usage.TotalTokens = promptEstimate
	} else if embResp.Usage.TotalTokens == 0 && embResp.Usage.PromptTokens > 0 {
		embResp.Usage.TotalTokens = embResp.Usage.PromptTokens
	} else if embResp.Usage.PromptTokens == 0 && embResp.Usage.TotalTokens > 0 {
		embResp.Usage.PromptTokens = embResp.Usage.TotalTokens
	}
	if embResp.Usage.Provider == "" {
		embResp.Usage.Provider = deployment.ProviderName
	}

	// Report success metrics
	metrics := &router.ResponseMetrics{
		Latency: latency,
	}
	if embResp.Usage.TotalTokens > 0 {
		metrics.TotalTokens = embResp.Usage.TotalTokens
		metrics.InputTokens = embResp.Usage.PromptTokens
	}
	c.router.ReportSuccess(deployment, metrics)

	return embResp, nil
}

// ListModels returns all available models from registered providers.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var models []Model
	seen := make(map[string]bool)

	for _, prov := range c.providers {
		for _, m := range prov.SupportedModels() {
			if !seen[m] {
				models = append(models, Model{
					ID:       m,
					Provider: prov.Name(),
					Object:   "model",
				})
				seen[m] = true
			}
		}
	}

	return models, nil
}

// AddProvider adds a provider at runtime.
// This is useful for dynamically adding providers without recreating the client.
func (c *Client) AddProvider(name string, prov Provider, models []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.providers[name]; exists {
		return fmt.Errorf("provider %s already exists", name)
	}

	return c.addProviderInstance(name, prov, models)
}

// RemoveProvider removes a provider at runtime.
func (c *Client) RemoveProvider(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// Remove deployments
	for model, deployments := range c.deployments {
		var remaining []*provider.Deployment
		for _, d := range deployments {
			if d.ProviderName == name {
				c.router.RemoveDeployment(d.ID)
			} else {
				remaining = append(remaining, d)
			}
		}
		if len(remaining) == 0 {
			delete(c.deployments, model)
		} else {
			c.deployments[model] = remaining
		}
	}

	delete(c.providers, name)
	c.logger.Info("provider removed", "name", name)
	return nil
}

// GetStats returns routing statistics for a deployment.
func (c *Client) GetStats(deploymentID string) *DeploymentStats {
	return c.router.GetStats(deploymentID)
}

// GetProviders returns the names of all registered providers.
func (c *Client) GetProviders() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.providers))
	for name := range c.providers {
		names = append(names, name)
	}
	return names
}

// Close releases all resources held by the client.
func (c *Client) Close() error {
	if c.cache != nil {
		_ = c.cache.Close()
	}
	c.httpClient.CloseIdleConnections()
	if c.pipeline != nil {
		if err := c.pipeline.Shutdown(); err != nil {
			c.logger.Warn("failed to shutdown plugin pipeline", "error", err)
		}
	}
	c.logger.Info("llmux client closed")
	return nil
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

// RegisterProviderFactory registers a custom provider factory.
// This allows adding support for new provider types.
func (c *Client) RegisterProviderFactory(providerType string, factory ProviderFactory) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.factories[providerType] = factory
}

// Private methods

func (c *Client) buildRateLimitKey(model, user, apiKey string) string {
	defaultKey := "default"
	switch c.rateLimiterConfig.KeyStrategy {
	case RateLimitKeyByAPIKey:
		if apiKey != "" {
			return apiKey
		}
		return defaultKey
	case RateLimitKeyByModel:
		if model != "" {
			return model
		}
		return defaultKey
	case RateLimitKeyByAPIKeyAndModel:
		baseKey := defaultKey
		if apiKey != "" {
			baseKey = apiKey
		}
		if model == "" {
			return baseKey
		}
		return baseKey + ":" + model
	case RateLimitKeyByUser, "":
		if user != "" {
			return user
		}
		return defaultKey
	default:
		if user != "" {
			return user
		}
		return defaultKey
	}
}

func (c *Client) rateLimitAPIKey(ctx context.Context) string {
	apiKey := rateLimitAPIKeyFromContext(ctx)
	if apiKey != "" {
		return apiKey
	}
	if authCtx := auth.GetAuthContext(ctx); authCtx != nil && authCtx.APIKey != nil {
		return authCtx.APIKey.ID
	}
	return ""
}

func (c *Client) checkRateLimit(ctx context.Context, key, model string, estimatedTokens int) error {
	// Skip if rate limiting is disabled or limiter is nil
	if !c.rateLimiterConfig.Enabled || c.rateLimiter == nil {
		return nil
	}

	// Build descriptors for rate limit check
	var descriptors []resilience.Descriptor

	windowSize := c.rateLimiterConfig.WindowSize
	if windowSize == 0 {
		windowSize = time.Minute // Default to 1 minute
	}

	// Add RPM descriptor if limit is set
	if c.rateLimiterConfig.RPMLimit > 0 {
		descriptors = append(descriptors, resilience.Descriptor{
			Key:       key,
			Value:     model,
			Limit:     c.rateLimiterConfig.RPMLimit,
			Type:      resilience.LimitTypeRequests,
			Window:    windowSize,
			Increment: 1,
		})
	}

	// Add TPM descriptor if limit is set
	if c.rateLimiterConfig.TPMLimit > 0 {
		inc := int64(estimatedTokens)
		if inc <= 0 {
			inc = 1
		}
		descriptors = append(descriptors, resilience.Descriptor{
			Key:       key,
			Value:     model,
			Limit:     c.rateLimiterConfig.TPMLimit,
			Type:      resilience.LimitTypeTokens,
			Window:    windowSize,
			Increment: inc,
		})
	}

	if len(descriptors) == 0 {
		return nil // No limits configured
	}

	// Check limits
	results, err := c.rateLimiter.CheckAllow(ctx, descriptors)
	if err != nil || len(results) != len(descriptors) {
		if err == nil {
			err = fmt.Errorf("rate limiter returned %d results, expected %d", len(results), len(descriptors))
		}
		action := "allow"
		if !c.rateLimiterConfig.FailOpen {
			action = "deny"
		}
		metrics.RateLimiterBackendErrors.WithLabelValues("client", action).Inc()
		c.logger.Warn("rate limiter check failed",
			"error", err,
			"fail_open", c.rateLimiterConfig.FailOpen,
			"action", action,
		)
		if c.rateLimiterConfig.FailOpen {
			return nil
		}
		return errors.NewRateLimitError("llmux", model, "rate limiter backend unavailable")
	}

	// Check if any limit was exceeded
	for i, result := range results {
		if !result.Allowed {
			limitType := "requests"
			if descriptors[i].Type == resilience.LimitTypeTokens {
				limitType = "tokens"
			}
			return errors.NewRateLimitError(
				"llmux",
				model,
				fmt.Sprintf("rate limit exceeded: %s per minute limit reached (current: %d, limit: %d, reset at: %d)",
					limitType, result.Current, descriptors[i].Limit, result.ResetAt),
			)
		}
	}

	return nil
}

func (c *Client) executeWithRetry(
	ctx context.Context,
	prov provider.Provider,
	deployment *provider.Deployment,
	req *ChatRequest,
) (*ChatResponse, error) {
	var lastErr error
	promptTokens := tokenizer.EstimatePromptTokens(req.Model, req)

	for attempt := 0; attempt <= c.config.RetryCount; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := c.config.RetryBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.executeOnce(ctx, prov, deployment, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if llmErr, ok := err.(*errors.LLMError); ok && !llmErr.Retryable {
			return nil, err
		}

		// Try fallback if enabled
		if c.config.FallbackEnabled && attempt < c.config.RetryCount {
			reqCtx := buildRouterRequestContext(req, promptTokens, req.Stream)
			newDeployment, pickErr := c.router.PickWithContext(ctx, reqCtx)
			if pickErr == nil && newDeployment.ID != deployment.ID {
				deployment = newDeployment
				c.mu.RLock()
				if newProv, ok := c.providers[deployment.ProviderName]; ok {
					prov = newProv
				}
				c.mu.RUnlock()
				c.logger.Debug("falling back to different deployment",
					"deployment", deployment.ID,
					"attempt", attempt+1,
				)
			}
		}
	}

	return nil, lastErr
}

func (c *Client) executeOnce(
	ctx context.Context,
	prov provider.Provider,
	deployment *provider.Deployment,
	req *ChatRequest,
) (*ChatResponse, error) {
	start := time.Now()

	if err := c.validatePricing(req.Model, deployment.ProviderName); err != nil {
		return nil, err
	}

	httpReq, err := prov.BuildRequest(ctx, sanitizeChatRequestForProvider(req))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	c.router.ReportRequestStart(deployment)
	defer c.router.ReportRequestEnd(deployment)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.router.ReportFailure(deployment, err)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		llmErr := prov.MapError(resp.StatusCode, body)
		c.router.ReportFailure(deployment, llmErr)
		return nil, llmErr
	}

	chatResp, err := prov.ParseResponse(resp)
	if err != nil {
		c.router.ReportFailure(deployment, err)
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Usage == nil || chatResp.Usage.TotalTokens == 0 {
		promptTokens := tokenizer.EstimatePromptTokens(req.Model, req)
		completionTokens := tokenizer.EstimateCompletionTokens(req.Model, chatResp, "")
		chatResp.Usage = &types.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}
	if chatResp.Usage != nil && chatResp.Usage.Provider == "" {
		chatResp.Usage.Provider = deployment.ProviderName
	}

	// Report success metrics
	metrics := &router.ResponseMetrics{
		Latency: latency,
	}
	if chatResp.Usage != nil {
		metrics.InputTokens = chatResp.Usage.PromptTokens
		metrics.OutputTokens = chatResp.Usage.CompletionTokens
		metrics.TotalTokens = chatResp.Usage.TotalTokens
	}
	c.router.ReportSuccess(deployment, metrics)

	return chatResp, nil
}

// CalculateCost computes the usage cost for a given model using loaded pricing data.
// Returns 0 when pricing is unavailable or model not found.
func (c *Client) CalculateCost(model string, usage *types.Usage) float64 {
	if usage == nil || c.pricing == nil {
		return 0
	}

	provider := usage.Provider
	price, ok := c.pricing.GetPrice(model, provider)
	if !ok {
		return 0
	}

	inputCost := float64(usage.PromptTokens) * price.InputCostPerToken
	outputCost := float64(usage.CompletionTokens) * price.OutputCostPerToken
	return inputCost + outputCost
}

func (c *Client) validatePricing(model, provider string) error {
	if c.pricing == nil {
		return errors.NewInternalError(provider, model, "pricing registry unavailable")
	}

	if _, ok := c.pricing.GetPrice(model, provider); !ok {
		return errors.NewInternalError(provider, model, "pricing not configured for model")
	}

	return nil
}

func (c *Client) addProviderFromConfig(cfg ProviderConfig) error {
	factory, ok := c.factories[cfg.Type]
	if !ok {
		return fmt.Errorf("unknown provider type: %s (available: %v)", cfg.Type, c.availableFactories())
	}

	prov, err := factory(cfg)
	if err != nil {
		return err
	}

	return c.addProviderInstance(cfg.Name, prov, cfg.Models)
}

func (c *Client) addProviderInstance(name string, prov provider.Provider, models []string) error {
	c.providers[name] = prov

	// Create deployments for each model
	for _, model := range models {
		deployment := &provider.Deployment{
			ID:           fmt.Sprintf("%s-%s", name, model),
			ProviderName: name,
			ModelName:    model,
		}
		c.deployments[model] = append(c.deployments[model], deployment)

		// If router is already initialized, add deployment
		if c.router != nil {
			c.router.AddDeployment(deployment)
		}
	}

	c.logger.Info("provider registered", "name", name, "models", models)
	return nil
}

func (c *Client) createRouter(strategy Strategy) router.Router {
	// Use the routers package for all strategies
	config := router.DefaultConfig()
	config.Strategy = strategy
	config.CooldownPeriod = c.config.CooldownPeriod
	config.LatencyBuffer = 0.1
	config.MaxLatencyListSize = 10
	config.PricingFile = c.config.PricingFile
	config.DefaultProvider = c.config.DefaultProvider
	r, err := routers.NewWithStore(config, c.config.StatsStore)
	if err != nil {
		// Fallback to shuffle router if strategy is invalid
		return routers.NewShuffleRouter()
	}
	return r
}

func (c *Client) registerBuiltinFactories() {
	// Register all built-in provider factories from the providers package
	for _, name := range providers.List() {
		if factory, ok := providers.Get(name); ok {
			c.factories[name] = factory
		}
	}
}

func (c *Client) availableFactories() []string {
	names := make([]string, 0, len(c.factories))
	for name := range c.factories {
		names = append(names, name)
	}
	return names
}

func (c *Client) getFromCache(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if c.cache == nil {
		return nil, nil
	}

	key, err := c.generateCacheKey(req)
	if err != nil {
		return nil, err
	}

	data, err := c.cache.Get(ctx, key)
	if err != nil || data == nil {
		metrics.CacheMisses.WithLabelValues(c.cacheTypeLabel, req.Model).Inc()
		return nil, err
	}

	var resp ChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		metrics.CacheMisses.WithLabelValues(c.cacheTypeLabel, req.Model).Inc()
		return nil, err
	}

	metrics.CacheHits.WithLabelValues(c.cacheTypeLabel, req.Model).Inc()
	c.logger.Debug("cache hit", "model", req.Model)
	return &resp, nil
}

func (c *Client) storeInCache(ctx context.Context, req *ChatRequest, resp *ChatResponse) {
	if c.cache == nil {
		return
	}

	key, err := c.generateCacheKey(req)
	if err != nil {
		return
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	if err := c.cache.Set(ctx, key, data, c.config.CacheTTL); err != nil {
		c.logger.Debug("cache store failed", "error", err)
	}
}

func (c *Client) generateCacheKey(req *ChatRequest) (string, error) {
	// Simple cache key generation based on model and messages
	messages, err := json.Marshal(req.Messages)
	if err != nil {
		return "", err
	}

	// Include relevant parameters in key
	keyData := struct {
		Model       string          `json:"model"`
		Messages    json.RawMessage `json:"messages"`
		Temperature *float64        `json:"temperature,omitempty"`
		MaxTokens   int             `json:"max_tokens,omitempty"`
	}{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	data, err := json.Marshal(keyData)
	if err != nil {
		return "", err
	}

	// Use simple hash for key
	return fmt.Sprintf("llmux:%x", hashBytes(data)), nil
}

// hashBytes returns a simple hash of the input bytes.
func hashBytes(data []byte) uint64 {
	var h uint64 = 14695981039346656037 // FNV offset basis
	for _, b := range data {
		h ^= uint64(b)
		h *= 1099511628211 // FNV prime
	}
	return h
}
