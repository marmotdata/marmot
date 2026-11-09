package kafka

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

func TestTopicMRN(t *testing.T) {
	s := &Source{config: &Config{}}
	if got, want := s.topicMRN("orders.v1"), "mrn://asset/topic/kafka/orders.v1"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	s.config.Zone = "prod-eu"
	if got, want := s.topicMRN("orders.v1"), "mrn://asset/topic/kafka@prod-eu/orders.v1"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestZoneBindings(t *testing.T) {
	s := &Source{config: &Config{BootstrapServers: "Broker-1:9093, broker-2, broker-1:9093"}}
	s.config.Zone = "prod-eu"

	want := []pluginsdk.ZoneBinding{
		{Provider: "Kafka", Host: "broker-1:9093", Zone: "prod-eu"},
		{Provider: "Kafka", Host: "broker-2:9092", Zone: "prod-eu"},
	}
	got := s.zoneBindings()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("binding %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
