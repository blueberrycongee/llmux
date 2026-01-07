package mcp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// mcpToolExecutions tracks total MCP tool executions.
	mcpToolExecutions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llmux_mcp_tool_executions_total",
			Help: "Total number of MCP tool executions",
		},
		[]string{"client_id", "tool_name", "status"},
	)

	// mcpToolLatency tracks MCP tool execution latency.
	mcpToolLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llmux_mcp_tool_latency_seconds",
			Help:    "MCP tool execution latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"client_id", "tool_name"},
	)

	// mcpClientConnections tracks MCP client connection states.
	mcpClientConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llmux_mcp_client_connections",
			Help: "Current MCP client connection states (1=connected, 0=disconnected)",
		},
		[]string{"client_id", "client_name", "type"},
	)

	// mcpToolsAvailable tracks the number of available tools per client.
	mcpToolsAvailable = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llmux_mcp_tools_available",
			Help: "Number of available MCP tools per client",
		},
		[]string{"client_id", "client_name"},
	)

	// mcpAgentIterations tracks agentic loop iterations.
	mcpAgentIterations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llmux_mcp_agent_iterations",
			Help:    "Number of iterations in agentic tool execution loops",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		[]string{"status"},
	)
)

// RecordToolExecution records a tool execution metric.
func RecordToolExecution(clientID, toolName, status string, latencySeconds float64) {
	mcpToolExecutions.WithLabelValues(clientID, toolName, status).Inc()
	mcpToolLatency.WithLabelValues(clientID, toolName).Observe(latencySeconds)
}

// RecordClientConnection records a client connection state.
func RecordClientConnection(clientID, clientName, connType string, connected bool) {
	value := 0.0
	if connected {
		value = 1.0
	}
	mcpClientConnections.WithLabelValues(clientID, clientName, connType).Set(value)
}

// RecordToolsAvailable records the number of available tools for a client.
func RecordToolsAvailable(clientID, clientName string, count int) {
	mcpToolsAvailable.WithLabelValues(clientID, clientName).Set(float64(count))
}

// RecordAgentIterations records the number of iterations in an agentic loop.
func RecordAgentIterations(iterations int, status string) {
	mcpAgentIterations.WithLabelValues(status).Observe(float64(iterations))
}
