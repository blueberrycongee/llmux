// Example: Using LLMux Plugin System
//
// This example demonstrates how to use the LLMux plugin system
// for request interception, modification, and short-circuiting.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/internal/plugin/builtin"
)

func main() {
	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create built-in plugins
	loggingPlugin := builtin.NewLoggingPlugin(logger,
		builtin.WithLogRequestBody(true),
		builtin.WithLogResponseBody(true),
	)

	rateLimitPlugin := builtin.NewRateLimitPlugin(100.0, 50,
		builtin.WithRateLimitLogger(logger),
	)

	metricsPlugin := builtin.NewMetricsPlugin(
		builtin.WithMetricsCallback(func(m *builtin.RequestMetrics) {
			logger.Info("request completed",
				"request_id", m.RequestID,
				"model", m.Model,
				"latency_ms", m.LatencyMs,
				"success", m.Success,
			)
		}),
	)

	// Create in-memory cache backend
	cacheBackend := builtin.NewMemoryCacheBackend(
		builtin.WithMemoryCacheMaxSize(1000),
		builtin.WithMemoryCacheCleanupInterval(5*time.Minute),
	)
	cachePlugin := builtin.NewCachePlugin(cacheBackend,
		builtin.WithCacheTTL(time.Hour),
		builtin.WithCacheLogger(logger),
	)

	// Create a custom plugin
	customPlugin := &CustomPlugin{
		logger: logger,
	}

	// Create LLMux client with plugins
	client, err := llmux.New(
		llmux.WithProvider(llmux.ProviderConfig{
			Name:   "openai",
			Type:   "openai",
			APIKey: os.Getenv("OPENAI_API_KEY"),
			Models: []string{"gpt-4o", "gpt-4o-mini"},
		}),
		llmux.WithLogger(logger),

		// Register plugins (executed in priority order)
		llmux.WithPlugin(rateLimitPlugin),  // Priority: 5
		llmux.WithPlugin(cachePlugin),      // Priority: 10
		llmux.WithPlugin(loggingPlugin),    // Priority: 1000
		llmux.WithPlugin(metricsPlugin),    // Priority: 999
		llmux.WithPlugin(customPlugin),     // Priority: 50

		// Optional: configure plugin pipeline
		llmux.WithPluginConfig(plugin.PipelineConfig{
			PreHookTimeout:  5 * time.Second,
			PostHookTimeout: 5 * time.Second,
			PropagateErrors: false,
		}),
	)
	if err != nil {
		logger.Error("failed to create client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	// Make a request - plugins will automatically handle:
	// 1. Rate limiting check (RateLimitPlugin)
	// 2. Cache lookup (CachePlugin)
	// 3. Custom pre-processing (CustomPlugin)
	// 4. Logging (LoggingPlugin)
	// 5. Metrics collection (MetricsPlugin)
	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, &llmux.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []llmux.ChatMessage{
			{Role: "user", Content: []byte(`"Hello, how are you?"`)},
		},
	})

	if err != nil {
		logger.Error("request failed", "error", err)
	} else {
		logger.Info("received response",
			"id", resp.ID,
			"model", resp.Model,
		)
		if len(resp.Choices) > 0 {
			fmt.Println("Assistant:", string(resp.Choices[0].Message.Content))
		}
	}

	// Get metrics snapshot
	snapshot := metricsPlugin.GetSnapshot()
	logger.Info("metrics snapshot",
		"total_requests", snapshot.TotalRequests,
		"successful", snapshot.SuccessfulRequests,
		"failed", snapshot.FailedRequests,
		"cache_hits", snapshot.CacheHits,
		"avg_latency_ms", snapshot.AvgLatencyMs,
	)
}

// CustomPlugin demonstrates how to implement a custom plugin.
type CustomPlugin struct {
	logger *slog.Logger
}

func (p *CustomPlugin) Name() string     { return "custom-plugin" }
func (p *CustomPlugin) Priority() int    { return 50 }

func (p *CustomPlugin) PreHook(ctx *plugin.Context, req *llmux.ChatRequest) (*llmux.ChatRequest, *plugin.ShortCircuit, error) {
	p.logger.Info("custom plugin: request received",
		"request_id", ctx.RequestID,
		"model", req.Model,
	)

	// Example: Add a system message
	// systemMsg := llmux.ChatMessage{
	// 	Role:    "system",
	// 	Content: []byte(`"You are a helpful assistant."`),
	// }
	// req.Messages = append([]llmux.ChatMessage{systemMsg}, req.Messages...)

	return req, nil, nil
}

func (p *CustomPlugin) PostHook(ctx *plugin.Context, resp *llmux.ChatResponse, err error) (*llmux.ChatResponse, error, error) {
	if err != nil {
		p.logger.Warn("custom plugin: request failed",
			"request_id", ctx.RequestID,
			"error", err,
		)
		// Example: Error recovery
		// return &llmux.ChatResponse{...}, nil, nil
	} else if resp != nil {
		p.logger.Info("custom plugin: request succeeded",
			"request_id", ctx.RequestID,
			"tokens", resp.Usage.TotalTokens,
		)
	}
	return resp, err, nil
}

func (p *CustomPlugin) Cleanup() error {
	p.logger.Info("custom plugin: cleanup")
	return nil
}
