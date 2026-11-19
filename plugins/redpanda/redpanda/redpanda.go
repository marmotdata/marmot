// Package redpanda is the Redpanda plugin: the kafka source under Redpanda's
// own metadata. It is a package rather than only a main so a host can
// compile it in.
package redpanda

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kafka/kafka"
)

// Source discovers topics from a Redpanda cluster.
type Source = kafka.Source

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	spec := pluginsdk.DeriveSpec(kafka.Config{},
		pluginsdk.Override("bootstrap_servers",
			pluginsdk.Placeholder("seed-xxxxx.cloud.redpanda.com:9092"),
		),
	)

	return pluginsdk.Meta{
		ID:            "redpanda",
		MarmotVersion: ">=0.13.0",
		Name:          "Redpanda",
		Description:   "Discover topics from Redpanda clusters",
		Icon:          "redpanda",
		Category:      "streaming",
		ConfigSpec:    spec,
	}
}
