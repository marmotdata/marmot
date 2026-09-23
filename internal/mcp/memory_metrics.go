package mcp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Query text is never a label: its cardinality is unbounded and it is closer
// to memory content than to telemetry.
var (
	memoryLookups = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_memory_lookups_total",
		Help: "Single-entity MCP lookups by the memory they carried: empty, complete, or truncated when older memories were left out",
	}, []string{"entity_type", "result"})

	memoryRecalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "marmot_memory_recalls_total",
		Help: "MCP recall calls by scope (entity or all), mode (search or list) and whether anything came back",
	}, []string{"scope", "mode", "result"})
)

func recallResult(n int) string {
	if n == 0 {
		return "empty"
	}
	return "found"
}

func lookupResult(left int) string {
	if left == 0 {
		return "complete"
	}
	return "truncated"
}
