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

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/cache"
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
	providers   map[string]provider.Provider
	deployments map[string][]*provider.Deployment // model -> deployments
	router      router.Router
	cache       cache.Cache
	httpClient  *http.Client
	logger      *slog.Logger
	config      *ClientConfig
	pipeline    *plugin.Pipeline

	// Provider factories for creating providers from config
	factories map[string]provider.Factory

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

	// Initialize providers from config
	for _, pcfg := range cfg.Providers {
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
	}

	c.logger.Info("llmux client initialized",
		"providers", len(c.providers),
		"strategy", cfg.RouterStrategy,
		"cache_enabled", cfg.CacheEnabled,
	)

	// Initialize plugin pipeline
	pipelineConfig := plugin.DefaultPipelineConfig()
	if cfg.PluginConfig != nil {
		pipelineConfig = *cfg.PluginConfig
	}
	c.pipeline = plugin.NewPipeline(c.logger, pipelineConfig)

	// Register plugins
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
			finalResp, _ := c.pipeline.RunPostHooks(pCtx, sc.Response, nil, c.pipeline.PluginCount())
			return finalResp, nil
		}
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

	if resp == nil {
		// Route to deployment
		var deployment *provider.Deployment
		deployment, err = c.router.Pick(ctx, req.Model)
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

	// Route to deployment
	deployment, err := c.router.Pick(ctx, req.Model)
	if err != nil {
		return nil, fmt.Errorf("no available deployment for model %s: %w", req.Model, err)
	}

	// Get provider
	c.mu.RLock()
	prov, ok := c.providers[deployment.ProviderName]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider %s not found", deployment.ProviderName)
	}

	// Build and execute request
	httpReq, err := prov.BuildRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	c.router.ReportRequestStart(deployment)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.router.ReportRequestEnd(deployment)
		c.router.ReportFailure(deployment, err)
		return nil, fmt.Errorf("execute request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.router.ReportRequestEnd(deployment)
		llmErr := prov.MapError(resp.StatusCode, body)
		c.router.ReportFailure(deployment, llmErr)
		return nil, llmErr
	}

	return newStreamReader(resp.Body, prov, deployment, c.router), nil
}

// Embedding sends an embedding request (future support).
func (c *Client) Embedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, fmt.Errorf("embedding not yet implemented")
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
		c.cache.Close()
	}
	c.httpClient.CloseIdleConnections()
	if c.pipeline != nil {
		c.pipeline.Shutdown()
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

func (c *Client) executeWithRetry(
	ctx context.Context,
	prov provider.Provider,
	deployment *provider.Deployment,
	req *ChatRequest,
) (*ChatResponse, error) {
	var lastErr error

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
		if llmErr, ok := err.(*LLMError); ok && !llmErr.Retryable {
			return nil, err
		}

		// Try fallback if enabled
		if c.config.FallbackEnabled && attempt < c.config.RetryCount {
			newDeployment, pickErr := c.router.Pick(ctx, req.Model)
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

	httpReq, err := prov.BuildRequest(ctx, req)
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
	defer resp.Body.Close()

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
	config := router.Config{
		Strategy:           strategy,
		CooldownPeriod:     c.config.CooldownPeriod,
		LatencyBuffer:      0.1,
		MaxLatencyListSize: 10,
	}
	r, err := routers.New(config)
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
		return nil, err
	}

	var resp ChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

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
