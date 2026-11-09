package asyncapi

import (
	"testing"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kafkaDoc(servers map[string]*asyncapi3.Server) *asyncapi3.Document {
	doc := testDoc()
	doc.Servers = servers
	return doc
}

func TestKafkaTopicMRN_CarriesTheServerHostAsAHint(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := kafkaDoc(map[string]*asyncapi3.Server{
		"prod": {Host: "Broker-1.Example.com:9093", Protocol: "kafka-secure"},
	})
	channel := channelWithBinding(t, "orders", "kafka", map[string]interface{}{"topic": "orders.v1"})

	binding := &kafka.ChannelBinding{Topic: "orders.v1"}
	asset := s.createKafkaTopic(doc, "orders", channel, binding)

	assert.Equal(t, "mrn://asset/topic/kafka@host=broker-1.example.com:9093/orders.v1", *asset.MRN)
	assert.Equal(t, []string{*asset.MRN}, s.getChannelAssetMRNs(doc, "orders", channel))
}

func TestKafkaTopicMRN_AddsTheDefaultPort(t *testing.T) {
	doc := kafkaDoc(map[string]*asyncapi3.Server{
		"prod": {Host: "broker-1.example.com", Protocol: "kafka"},
	})
	assert.Equal(t, "broker-1.example.com:9092", serverHost(doc, &asyncapi3.Channel{}, "Kafka"))
}

func TestKafkaTopicMRN_HasNoHintWhenServersDisagree(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := kafkaDoc(map[string]*asyncapi3.Server{
		"prod":    {Host: "prod.example.com:9092", Protocol: "kafka"},
		"staging": {Host: "staging.example.com:9092", Protocol: "kafka"},
	})
	channel := channelWithBinding(t, "orders", "kafka", map[string]interface{}{"topic": "orders"})

	got := s.getChannelAssetMRNs(doc, "orders", channel)
	assert.Equal(t, []string{"mrn://asset/topic/kafka/orders"}, got)
}

func TestKafkaTopicMRN_ChannelServersNarrowTheHint(t *testing.T) {
	s := &Source{config: &Config{}}
	doc := kafkaDoc(map[string]*asyncapi3.Server{
		"prod":    {Host: "prod.example.com:9092", Protocol: "kafka"},
		"staging": {Host: "staging.example.com:9092", Protocol: "kafka"},
	})
	channel := channelWithBinding(t, "orders", "kafka", map[string]interface{}{"topic": "orders"})
	channel.Servers = []*asyncapi3.Reference{{Ref: "#/servers/staging"}}

	got := s.getChannelAssetMRNs(doc, "orders", channel)
	assert.Equal(t, []string{"mrn://asset/topic/kafka@host=staging.example.com:9092/orders"}, got)
}

func TestNATSSubjectMRN_CarriesTheServerHostAsAHint(t *testing.T) {
	doc := kafkaDoc(map[string]*asyncapi3.Server{
		"main": {Host: "nats.example.com", Protocol: "nats"},
	})
	assert.Equal(t, "nats.example.com:4222", serverHost(doc, &asyncapi3.Channel{}, "NATS"))
}

func TestOwnMRNs_CarryTheZone(t *testing.T) {
	s := &Source{config: &Config{}}
	s.config.Zone = "platform"
	doc := testDoc()

	service := s.createServiceAsset(doc)
	assert.Equal(t, "mrn://asset/service/asyncapi@platform/orders-service", *service.MRN)

	channel := s.createGenericChannelAsset(doc, "userCreated", &asyncapi3.Channel{})
	assert.Equal(t, "mrn://asset/channel/asyncapi@platform/orders-service.usercreated", *channel.MRN)
}

func TestGenericChannelMRN_IsQualifiedByItsService(t *testing.T) {
	s := &Source{config: &Config{}}

	first := s.createGenericChannelAsset(testDoc(), "userCreated", &asyncapi3.Channel{})
	other := testDoc()
	other.Info.Title = "billing-service"
	second := s.createGenericChannelAsset(other, "userCreated", &asyncapi3.Channel{})

	require.NotEqual(t, *first.MRN, *second.MRN)
	assert.Equal(t, []string{*first.MRN}, s.getChannelAssetMRNs(testDoc(), "userCreated", &asyncapi3.Channel{}))
}
