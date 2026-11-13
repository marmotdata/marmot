package sqs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSource() *Source { return &Source{config: &Config{}} }

// Both ends of a DLQ edge must be queues this run discovered, or the
// server silently drops the edge and the lineage disappears.
func TestDLQEdge_BothQueuesDiscovered(t *testing.T) {
	discovered := map[string]string{
		"orders":     "arn:aws:sqs:eu-west-1:1:orders",
		"orders-dlq": "arn:aws:sqs:eu-west-1:1:orders-dlq",
	}

	edge, ok := newSource().dlqEdge("orders", "orders-dlq", discovered)

	require.True(t, ok)
	assert.Equal(t, "mrn://asset/queue/sqs/orders", edge.Source)
	assert.Equal(t, "mrn://asset/queue/sqs/orders-dlq", edge.Target)
	assert.Equal(t, "DLQ", edge.Type)
}

// A centralised DLQ living in another account or region is never
// discovered by this run, so the edge must be dropped.
func TestDLQEdge_TargetNotDiscovered(t *testing.T) {
	discovered := map[string]string{"orders": "arn:aws:sqs:eu-west-1:1:orders"}

	_, ok := newSource().dlqEdge("orders", "central-dlq", discovered)

	assert.False(t, ok, "the DLQ has no asset behind it")
}

func TestDLQEdge_SourceNotDiscovered(t *testing.T) {
	discovered := map[string]string{"orders-dlq": "arn:aws:sqs:eu-west-1:1:orders-dlq"}

	_, ok := newSource().dlqEdge("orders", "orders-dlq", discovered)

	assert.False(t, ok, "the source queue has no asset behind it")
}

func TestDLQEdge_CarriesTheZone(t *testing.T) {
	s := newSource()
	s.config.Zone = "prod"
	discovered := map[string]string{"orders": "a", "orders-dlq": "b"}

	edge, ok := s.dlqEdge("orders", "orders-dlq", discovered)

	require.True(t, ok)
	assert.Equal(t, "mrn://asset/queue/sqs@prod/orders", edge.Source)
	assert.Equal(t, "mrn://asset/queue/sqs@prod/orders-dlq", edge.Target)
}
