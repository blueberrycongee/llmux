package main

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/metrics"
)

type dbStatsProvider interface {
	DBStats() sql.DBStats
}

func startDBPoolMetrics(ctx context.Context, provider dbStatsProvider, logger *slog.Logger, interval time.Duration) func() {
	if provider == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}

	metrics.UpdateDBPoolStats(provider.DBStats())

	ticker := time.NewTicker(interval)
	stopCh := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() { close(stopCh) })
	}

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				metrics.UpdateDBPoolStats(provider.DBStats())
			case <-ctx.Done():
				stop()
				return
			case <-stopCh:
				return
			}
		}
	}()

	logger.Debug("db pool metrics updater started", "interval", interval.String())
	return stop
}
