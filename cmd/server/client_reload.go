package main

import (
	"log/slog"
	"sync/atomic"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/config"
)

type clientReloader struct {
	logger     *slog.Logger
	swapper    *api.ClientSwapper
	build      func(*config.Config) (*llmux.Client, error)
	inProgress atomic.Bool
}

func newClientReloader(logger *slog.Logger, swapper *api.ClientSwapper, build func(*config.Config) (*llmux.Client, error)) *clientReloader {
	if logger == nil {
		logger = slog.Default()
	}
	return &clientReloader{
		logger:  logger,
		swapper: swapper,
		build:   build,
	}
}

func (r *clientReloader) Reload(cfg *config.Config) {
	if !r.inProgress.CompareAndSwap(false, true) {
		r.logger.Warn("client reload already in progress")
		return
	}
	defer r.inProgress.Store(false)

	next, err := r.build(cfg)
	if err != nil {
		r.logger.Error("failed to rebuild llmux client", "error", err)
		return
	}
	if next == nil {
		r.logger.Error("failed to rebuild llmux client", "error", "nil client")
		return
	}

	r.swapper.Swap(next)

	r.logger.Info("llmux client reloaded",
		"providers", len(cfg.Providers),
		"routing_strategy", cfg.Routing.Strategy,
	)
}
