// Package confluent is the Confluent Cloud plugin: the kafka source under
// Confluent's own metadata, with the settings Confluent Cloud fixes hidden.
// It is a package rather than only a main so a host can compile it in.
package confluent

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kafka/kafka"
)

// Source discovers topics from a Confluent Cloud cluster.
type Source = kafka.Source

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	spec := pluginsdk.DeriveSpec(kafka.Config{},
		pluginsdk.Hide(
			"tls",
			"consumer_config",
			"authentication.type",
			"authentication.mechanism",
		),
		pluginsdk.Override("bootstrap_servers",
			pluginsdk.Placeholder("pkc-xxxxx.us-west-2.aws.confluent.cloud:9092"),
		),
	)

	return pluginsdk.Meta{
		ID:            "confluent",
		MarmotVersion: ">=0.13.0",
		Name:          "Confluent Cloud",
		Description:   "Discover Kafka topics from Confluent Cloud clusters",
		Icon:          "confluent",
		Category:      "streaming",
		ConfigSpec:    spec,
	}
}
