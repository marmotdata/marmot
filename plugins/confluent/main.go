package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kafka/kafka"
)

func main() {
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

	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "confluent",
			Name:        "Confluent Cloud",
			Description: "Discover Kafka topics from Confluent Cloud clusters",
			Icon:        "confluent",
			Category:    "streaming",
			ConfigSpec:  spec,
		},
		Source: &kafka.Source{},
	})
}
