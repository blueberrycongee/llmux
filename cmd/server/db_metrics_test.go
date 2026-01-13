package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/metrics"
)

type fakeDBStatsProvider struct {
	stats sql.DBStats
}

func (f fakeDBStatsProvider) DBStats() sql.DBStats {
	return f.stats
}

func TestStartDBPoolMetrics_UpdatesImmediately(t *testing.T) {
	provider := fakeDBStatsProvider{
		stats: sql.DBStats{
			InUse:              2,
			Idle:               4,
			MaxOpenConnections: 8,
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startDBPoolMetrics(ctx, provider, nil, time.Hour)
	require.NotNil(t, stop)

	require.Equal(t, 2.0, testutil.ToFloat64(metrics.DBConnectionPoolSize.WithLabelValues("active")))
	require.Equal(t, 4.0, testutil.ToFloat64(metrics.DBConnectionPoolSize.WithLabelValues("idle")))
	require.Equal(t, 8.0, testutil.ToFloat64(metrics.DBConnectionPoolSize.WithLabelValues("max")))

	stop()
}

func TestStartDBPoolMetrics_SkipsWhenNil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startDBPoolMetrics(ctx, nil, nil, time.Hour)
	require.Nil(t, stop)
}
