package main

import (
	"log/slog"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

type jobRunner interface {
	Start()
	Stop()
}

func startJobRunner(cfg *config.Config, store auth.Store, logger *slog.Logger, newRunner func(*auth.JobRunnerConfig) jobRunner) jobRunner {
	if cfg == nil || !cfg.Governance.Enabled || store == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if newRunner == nil {
		newRunner = func(cfg *auth.JobRunnerConfig) jobRunner {
			return auth.NewJobRunner(cfg)
		}
	}
	runner := newRunner(&auth.JobRunnerConfig{
		Store:    store,
		Logger:   logger,
		Interval: time.Hour,
	})
	if runner == nil {
		return nil
	}
	runner.Start()
	return runner
}
