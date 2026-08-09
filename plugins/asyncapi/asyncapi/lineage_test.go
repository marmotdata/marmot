package asyncapi

import (
	"encoding/json"
	"testing"

	asyncapi "github.com/charlie-haley/asyncapi-go"
	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/ibmmq"
	"github.com/charlie-haley/asyncapi-go/bindings/solace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseSolaceBinding parses the binding exactly the way discovery does, so
// the test cannot accidentally feed the two paths different input.
func parseSolaceBinding(t *testing.T, channel *asyncapi3.Channel) (*solace.OperationBinding, error) {
	t.Helper()
	return asyncapi.ParseBindings[solace.OperationBinding](channel.Bindings, "solace")
}

func parseIBMMQBinding(t *testing.T, channel *asyncapi3.Channel) (*ibmmq.ChannelBinding, error) {
	t.Helper()
	return asyncapi.ParseBindings[ibmmq.ChannelBinding](channel.Bindings, "ibmmq")
}

// channelWithBinding builds a channel carrying one raw protocol binding,
// the same shape the parser sees when reading a real spec.
func channelWithBinding(t *testing.T, address, protocol string, binding interface{}) *asyncapi3.Channel {
	t.Helper()

	raw, err := json.Marshal(binding)
	require.NoError(t, err)

	var decoded any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	return &asyncapi3.Channel{
		Address:  address,
		Bindings: map[string]any{protocol: decoded},
	}
}

// testDoc is the minimum document the asset builders dereference.
func testDoc() *asyncapi3.Document {
	return &asyncapi3.Document{
		AsyncAPI: "3.0.0",
		Info:     &asyncapi3.Info{Title: "orders-service", Version: "1.0.0"},
	}
}

// assertChannelMRNsMatchCreatedAssets is the real guard: the MRNs the
// lineage pass hands out must be exactly the MRNs the asset pass created.
// These are two separate code paths over the same binding, and when their
// fallback conditions drift the asset is created with no edge pointing at
// it, so the lineage silently disappears.
func assertChannelMRNsMatchCreatedAssets(t *testing.T, s *Source, channelName string, channel *asyncapi3.Channel, created []string) {
	t.Helper()

	got := s.getChannelAssetMRNs(channelName, channel)
	assert.ElementsMatch(t, created, got,
		"the lineage MRNs and the created assets have drifted apart")
}

// A Solace destination that matches neither the queue branch nor the topic
// branch still produces a generic topic asset, so the MRN list has to
// produce that same generic topic.
func TestSolace_DestinationMatchingNoBranchStillGetsItsMRN(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := testDoc()

	// destinationType "queue" but no queue object: neither branch fires.
	channel := channelWithBinding(t, "orders.stream", "solace", map[string]interface{}{
		"destinations": []map[string]interface{}{
			{"destinationType": "queue"},
		},
	})

	binding, _ := parseSolaceBinding(t, channel)
	assets := s.createSolaceAssets(doc, "orders", channel, binding)
	require.Len(t, assets, 1, "the generic fallback asset should be created")

	var created []string
	for _, a := range assets {
		created = append(created, *a.MRN)
	}

	assertChannelMRNsMatchCreatedAssets(t, s, "orders", channel, created)
}

// A Solace binding with a real queue destination.
func TestSolace_QueueDestinationMatches(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := testDoc()

	channel := channelWithBinding(t, "orders.stream", "solace", map[string]interface{}{
		"destinations": []map[string]interface{}{
			{"destinationType": "queue", "queue": map[string]interface{}{"name": "order-queue"}},
		},
	})

	binding, _ := parseSolaceBinding(t, channel)
	assets := s.createSolaceAssets(doc, "orders", channel, binding)

	var created []string
	for _, a := range assets {
		created = append(created, *a.MRN)
	}
	assert.Equal(t, []string{"mrn://queue/solace/order-queue"}, created)

	assertChannelMRNsMatchCreatedAssets(t, s, "orders", channel, created)
}

// An IBM MQ binding with a Queue object whose ObjectName is empty reaches
// the generic fallback in the asset pass, so the MRN pass must too.
func TestIBMMQ_EmptyQueueObjectNameStillGetsItsMRN(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := testDoc()

	channel := channelWithBinding(t, "payments.in", "ibmmq", map[string]interface{}{
		"destinationType": "queue",
		"queue":           map[string]interface{}{"objectName": ""},
	})

	binding, _ := parseIBMMQBinding(t, channel)
	assets := s.createIBMMQAssets(doc, "payments", channel, binding)
	require.Len(t, assets, 1, "the generic fallback asset should be created")

	var created []string
	for _, a := range assets {
		created = append(created, *a.MRN)
	}

	assertChannelMRNsMatchCreatedAssets(t, s, "payments", channel, created)
}

// An IBM MQ binding naming a real queue.
func TestIBMMQ_NamedQueueMatches(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := testDoc()

	channel := channelWithBinding(t, "payments.in", "ibmmq", map[string]interface{}{
		"destinationType": "queue",
		"queue":           map[string]interface{}{"objectName": "PAY.IN"},
	})

	binding, _ := parseIBMMQBinding(t, channel)
	assets := s.createIBMMQAssets(doc, "payments", channel, binding)

	var created []string
	for _, a := range assets {
		created = append(created, *a.MRN)
	}
	assert.Equal(t, []string{"mrn://queue/ibmmq/pay.in"}, created)

	assertChannelMRNsMatchCreatedAssets(t, s, "payments", channel, created)
}
