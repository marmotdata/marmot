package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kafka/kafka"
)

func main() {
	spec := pluginsdk.DeriveSpec(kafka.Config{},
		pluginsdk.Override("bootstrap_servers",
			pluginsdk.Placeholder("seed-xxxxx.cloud.redpanda.com:9092"),
		),
	)

	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "redpanda",
			Name:        "Redpanda",
			Description: "Discover topics from Redpanda clusters",
			Icon:        "redpanda",
			Category:    "streaming",
			ConfigSpec:  spec,
		},
		Source: &kafka.Source{},
	})
}
