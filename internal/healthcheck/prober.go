// Package healthcheck provides proactive deployment probing.
package healthcheck

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	defaultProbeInterval = 30 * time.Second
	defaultProbeTimeout  = 10 * time.Second
)

// Config controls the proactive health checker behavior.
type Config struct {
	Enabled        bool
	Interval       time.Duration
	Timeout        time.Duration
	CooldownPeriod time.Duration
}

// ClientProvider supplies the current llmux client.
type ClientProvider interface {
	Acquire() (*llmux.Client, func())
}

// StaticClientProvider wraps a fixed client for probing.
type StaticClientProvider struct {
	Client *llmux.Client
}

// Acquire returns the configured client without reference tracking.
func (p StaticClientProvider) Acquire() (*llmux.Client, func()) {
	if p.Client == nil {
		return nil, func() {}
	}
	return p.Client, func() {}
}

// Prober periodically checks deployment health and updates router cooldowns.
type Prober struct {
	cfg      Config
	provider ClientProvider
	logger   *slog.Logger
	client   *http.Client
	started  atomic.Bool
}

// NewProber creates a new health checker.
func NewProber(cfg Config, provider ClientProvider, logger *slog.Logger) *Prober {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultProbeInterval
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultProbeTimeout
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Prober{
		cfg:      cfg,
		provider: provider,
		logger:   logger,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Start begins the probe loop until the context is canceled.
func (p *Prober) Start(ctx context.Context) {
	if p == nil || !p.cfg.Enabled {
		return
	}
	if p.provider == nil {
		p.logger.Warn("healthcheck prober missing client provider")
		return
	}
	if !p.started.CompareAndSwap(false, true) {
		return
	}

	go p.run(ctx)
}

func (p *Prober) run(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	p.runOnce(ctx)

	for {
		select {
		case <-ticker.C:
			p.runOnce(ctx)
		case <-ctx.Done():
			p.logger.Info("healthcheck prober stopped")
			return
		}
	}
}

func (p *Prober) runOnce(ctx context.Context) {
	client, release := p.provider.Acquire()
	if client == nil {
		return
	}
	defer release()

	deployments := client.ListDeployments()
	if len(deployments) == 0 {
		return
	}

	for _, deployment := range deployments {
		if ctx.Err() != nil {
			return
		}
		prov, ok := client.GetProvider(deployment.ProviderName)
		if !ok {
			p.logger.Warn("healthcheck provider missing",
				"provider", deployment.ProviderName,
				"deployment_id", deployment.ID,
			)
			continue
		}
		if err := p.probeDeployment(ctx, prov, deployment); err != nil {
			p.handleFailure(client, deployment, err)
			continue
		}
		p.handleSuccess(client, deployment)
	}
}

func (p *Prober) probeDeployment(ctx context.Context, prov provider.Provider, deployment *provider.Deployment) error {
	probeCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	req := buildProbeRequest(deployment.ModelName)
	httpReq, err := prov.BuildRequest(probeCtx, req)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			return errors.New("healthcheck probe failed")
		}
		return prov.MapError(resp.StatusCode, body)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (p *Prober) handleFailure(client *llmux.Client, deployment *provider.Deployment, err error) {
	if p.cfg.CooldownPeriod <= 0 {
		p.logger.Warn("healthcheck probe failed", "deployment_id", deployment.ID, "error", err)
		return
	}

	until := time.Now().Add(p.cfg.CooldownPeriod)
	if setErr := client.SetCooldown(deployment.ID, until); setErr != nil {
		p.logger.Warn("healthcheck cooldown update failed",
			"deployment_id", deployment.ID,
			"error", setErr,
		)
	}
	p.logger.Warn("healthcheck probe failed",
		"deployment_id", deployment.ID,
		"provider", deployment.ProviderName,
		"model", deployment.ModelName,
		"cooldown_until", until,
		"error", err,
	)
}

func (p *Prober) handleSuccess(client *llmux.Client, deployment *provider.Deployment) {
	if p.cfg.CooldownPeriod <= 0 {
		return
	}
	stats := client.GetStats(deployment.ID)
	if stats == nil || !time.Now().Before(stats.CooldownUntil) {
		return
	}
	if clearErr := client.SetCooldown(deployment.ID, time.Time{}); clearErr != nil {
		p.logger.Warn("healthcheck cooldown clear failed",
			"deployment_id", deployment.ID,
			"error", clearErr,
		)
	}
}

func buildProbeRequest(model string) *types.ChatRequest {
	return &types.ChatRequest{
		Model: model,
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`"healthcheck"`),
			},
		},
		MaxTokens: 1,
	}
}
