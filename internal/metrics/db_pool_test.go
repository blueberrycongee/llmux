package metrics

import (
	"database/sql"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestUpdateDBPoolStats(t *testing.T) {
	stats := sql.DBStats{
		InUse:              3,
		Idle:               7,
		MaxOpenConnections: 10,
	}

	UpdateDBPoolStats(stats)

	require.Equal(t, 3.0, testutil.ToFloat64(DBConnectionPoolSize.WithLabelValues("active")))
	require.Equal(t, 7.0, testutil.ToFloat64(DBConnectionPoolSize.WithLabelValues("idle")))
	require.Equal(t, 10.0, testutil.ToFloat64(DBConnectionPoolSize.WithLabelValues("max")))
}
