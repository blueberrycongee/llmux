package metrics

import "database/sql"

// UpdateDBPoolStats updates database connection pool metrics from sql.DBStats.
func UpdateDBPoolStats(stats sql.DBStats) {
	DBConnectionPoolSize.WithLabelValues("active").Set(float64(stats.InUse))
	DBConnectionPoolSize.WithLabelValues("idle").Set(float64(stats.Idle))
	DBConnectionPoolSize.WithLabelValues("max").Set(float64(stats.MaxOpenConnections))
}
