// Package observability provides unified configuration for all observability integrations.
package observability

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ObservabilityConfig contains configuration for all observability integrations.
type ObservabilityConfig struct {
	// Callbacks to enable (comma-separated: "prometheus,otel,langfuse,s3,slack")
	EnabledCallbacks []string `yaml:"enabled_callbacks" json:"enabled_callbacks"`

	// Prometheus configuration
	Prometheus struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
	} `yaml:"prometheus" json:"prometheus"`

	// OpenTelemetry configuration
	OpenTelemetry TracingConfig `yaml:"opentelemetry" json:"opentelemetry"`

	// Langfuse configuration
	Langfuse LangfuseConfig `yaml:"langfuse" json:"langfuse"`

	// S3 logging configuration
	S3 S3Config `yaml:"s3" json:"s3"`

	// Slack alerting configuration
	Slack SlackConfig `yaml:"slack" json:"slack"`

	// Content filtering
	ContentFilter struct {
		FilterBase64      bool     `yaml:"filter_base64" json:"filter_base64"`
		MaxContentLength  int      `yaml:"max_content_length" json:"max_content_length"`
		RedactPatterns    []string `yaml:"redact_patterns" json:"redact_patterns"`
	} `yaml:"content_filter" json:"content_filter"`

	// Label filtering for metrics
	MetricsLabelConfig []MetricsLabelConfig `yaml:"metrics_label_config" json:"metrics_label_config"`
}

// DefaultObservabilityConfig returns configuration from environment variables.
func DefaultObservabilityConfig() ObservabilityConfig {
	cfg := ObservabilityConfig{}

	// Parse enabled callbacks from environment
	if callbacks := os.Getenv("LLMUX_CALLBACKS"); callbacks != "" {
		cfg.EnabledCallbacks = strings.Split(callbacks, ",")
		for i := range cfg.EnabledCallbacks {
			cfg.EnabledCallbacks[i] = strings.TrimSpace(cfg.EnabledCallbacks[i])
		}
	}

	// Prometheus is enabled by default
	cfg.Prometheus.Enabled = os.Getenv("LLMUX_PROMETHEUS_ENABLED") != "false"

	// OpenTelemetry
	cfg.OpenTelemetry = DefaultTracingConfig()

	// Langfuse
	cfg.Langfuse = DefaultLangfuseConfig()

	// S3
	cfg.S3 = DefaultS3Config()

	// Slack
	cfg.Slack = DefaultSlackConfig()

	// Content filter defaults
	cfg.ContentFilter.FilterBase64 = os.Getenv("LLMUX_FILTER_BASE64") == "true"
	cfg.ContentFilter.MaxContentLength = 10000

	return cfg
}

// ObservabilityManager manages all observability integrations.
type ObservabilityManager struct {
	config          ObservabilityConfig
	callbackManager *CallbackManager
	tracerProvider  *TracerProvider
	contentFilter   *ContentFilter
	labelFilter     *LabelFilterManager
}

// NewObservabilityManager creates a new observability manager.
func NewObservabilityManager(cfg ObservabilityConfig) (*ObservabilityManager, error) {
	mgr := &ObservabilityManager{
		config:          cfg,
		callbackManager: NewCallbackManager(),
	}

	// Initialize content filter
	mgr.contentFilter = &ContentFilter{
		FilterBase64:      cfg.ContentFilter.FilterBase64,
		Base64Placeholder: "[base64_content_filtered]",
		MaxContentLength:  cfg.ContentFilter.MaxContentLength,
		RedactPlaceholder: "[REDACTED]",
	}

	// Initialize label filter
	mgr.labelFilter = NewLabelFilterManager(cfg.MetricsLabelConfig)

	// Initialize enabled callbacks
	for _, name := range cfg.EnabledCallbacks {
		if err := mgr.enableCallback(name); err != nil {
			return nil, fmt.Errorf("failed to enable callback %s: %w", name, err)
		}
	}

	// Always enable Prometheus if configured
	if cfg.Prometheus.Enabled {
		mgr.callbackManager.Register(NewPrometheusCallback())
	}

	return mgr, nil
}

// enableCallback enables a specific callback by name.
func (m *ObservabilityManager) enableCallback(name string) error {
	switch strings.ToLower(name) {
	case "prometheus":
		// Already handled separately
		return nil

	case "otel", "opentelemetry":
		if m.config.OpenTelemetry.Enabled {
			tp, err := InitTracing(context.Background(), m.config.OpenTelemetry)
			if err != nil {
				return err
			}
			m.tracerProvider = tp
			m.callbackManager.Register(NewOTelCallback(OTelCallbackConfig{
				Tracer: tp.Tracer(),
			}))
		}

	case "langfuse":
		if m.config.Langfuse.PublicKey != "" {
			cb, err := NewLangfuseCallback(m.config.Langfuse)
			if err != nil {
				return err
			}
			m.callbackManager.Register(cb)
		}

	case "s3":
		if m.config.S3.BucketName != "" {
			cb, err := NewS3Callback(m.config.S3)
			if err != nil {
				return err
			}
			m.callbackManager.Register(cb)
		}

	case "slack":
		if m.config.Slack.WebhookURL != "" {
			cb, err := NewSlackCallback(m.config.Slack)
			if err != nil {
				return err
			}
			m.callbackManager.Register(cb)
		}

	default:
		return fmt.Errorf("unknown callback: %s", name)
	}

	return nil
}

// CallbackManager returns the callback manager.
func (m *ObservabilityManager) CallbackManager() *CallbackManager {
	return m.callbackManager
}

// TracerProvider returns the tracer provider.
func (m *ObservabilityManager) TracerProvider() *TracerProvider {
	return m.tracerProvider
}

// ContentFilter returns the content filter.
func (m *ObservabilityManager) ContentFilter() *ContentFilter {
	return m.contentFilter
}

// LabelFilter returns the label filter manager.
func (m *ObservabilityManager) LabelFilter() *LabelFilterManager {
	return m.labelFilter
}

// LogSuccess logs a successful request through all callbacks.
func (m *ObservabilityManager) LogSuccess(ctx context.Context, payload *StandardLoggingPayload) {
	// Apply content filtering
	filtered := m.contentFilter.FilterPayload(payload)
	m.callbackManager.LogSuccessEvent(ctx, filtered)
}

// LogFailure logs a failed request through all callbacks.
func (m *ObservabilityManager) LogFailure(ctx context.Context, payload *StandardLoggingPayload, err error) {
	// Apply content filtering
	filtered := m.contentFilter.FilterPayload(payload)
	m.callbackManager.LogFailureEvent(ctx, filtered, err)
}

// LogFallback logs a fallback event through all callbacks.
func (m *ObservabilityManager) LogFallback(ctx context.Context, originalModel, fallbackModel string, err error, success bool) {
	m.callbackManager.LogFallbackEvent(ctx, originalModel, fallbackModel, err, success)
}

// Shutdown gracefully shuts down all integrations.
func (m *ObservabilityManager) Shutdown(ctx context.Context) error {
	// Shutdown callbacks
	if err := m.callbackManager.Shutdown(ctx); err != nil {
		return err
	}

	// Shutdown tracer provider
	if m.tracerProvider != nil {
		if err := m.tracerProvider.Shutdown(ctx); err != nil {
			return err
		}
	}

	return nil
}
