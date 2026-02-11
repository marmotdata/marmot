// +marmot:name=Redpanda
// +marmot:description=Discover topics from Redpanda clusters.
// +marmot:status=experimental
// +marmot:features=Assets
// +marmot:config-source=../kafka
package redpanda

//go:generate go run ../../../docgen/cmd/main.go

import (
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	"github.com/rs/zerolog/log"
)

func init() {
	spec := plugin.ApplyConfigOverrides(
		plugin.CloneConfigSpec(plugin.GenerateConfigSpec(kafka.Config{})),
		map[string]plugin.ConfigOverride{
			"bootstrap_servers": {Placeholder: "seed-xxxxx.cloud.redpanda.com:9092"},
		},
	)

	meta := plugin.PluginMeta{
		ID:          "redpanda",
		Name:        "Redpanda",
		Description: "Discover topics from Redpanda clusters",
		Icon:        "redpanda",
		Category:    "streaming",
		ConfigSpec:  spec,
	}

	if err := plugin.GetRegistry().Register(meta, &kafka.Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Redpanda plugin")
	}
}
