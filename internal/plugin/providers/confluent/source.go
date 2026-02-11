// +marmot:name=Confluent Cloud
// +marmot:description=Discover Kafka topics from Confluent Cloud clusters.
// +marmot:status=experimental
// +marmot:features=Assets
// +marmot:config-source=../kafka
package confluent

//go:generate go run ../../../docgen/cmd/main.go

import (
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	"github.com/rs/zerolog/log"
)

func init() {
	baseSpec := plugin.GenerateConfigSpec(kafka.Config{})

	spec := plugin.RemoveConfigFields(plugin.CloneConfigSpec(baseSpec), []string{
		"tls",
		"consumer_config",
		"authentication.type",
		"authentication.mechanism",
	})
	spec = plugin.ApplyConfigOverrides(spec, map[string]plugin.ConfigOverride{
		"bootstrap_servers": {Placeholder: "pkc-xxxxx.us-west-2.aws.confluent.cloud:9092"},
	})

	meta := plugin.PluginMeta{
		ID:          "confluent",
		Name:        "Confluent Cloud",
		Description: "Discover Kafka topics from Confluent Cloud clusters",
		Icon:        "confluent",
		Category:    "streaming",
		ConfigSpec:  spec,
	}

	if err := plugin.GetRegistry().Register(meta, &kafka.Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Confluent Cloud plugin")
	}
}
