// +marmot:name=Kafka
// +marmot:description=This plugin discovers Kafka topics from Kafka clusters.
// +marmot:status=experimental
package kafka

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Config for Kafka plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	BootstrapServers string            `json:"bootstrap_servers" description:"Comma-separated list of bootstrap servers"`
	ClientID         string            `json:"client_id" description:"Client ID for the consumer"`
	Authentication   *AuthConfig       `json:"authentication,omitempty" description:"Authentication configuration"`
	ConsumerConfig   map[string]string `json:"consumer_config,omitempty" description:"Additional consumer configuration"`
	ClientTimeout    int               `json:"client_timeout_seconds" description:"Request timeout in seconds"`
	TLS              *TLSConfig        `json:"tls,omitempty" description:"TLS configuration"`

	SchemaRegistry *SchemaRegistryConfig `json:"schema_registry,omitempty" description:"Schema Registry configuration"`

	TopicFilter *plugin.Filter `json:"topic_filter,omitempty" description:"Filter configuration for topics"`

	IncludePartitionInfo bool `json:"include_partition_info" description:"Whether to include partition information in metadata" default:"true"`

	IncludeTopicConfig bool `json:"include_topic_config" description:"Whether to include topic configuration in metadata" default:"true"`
}

// Authentication configuration
type AuthConfig struct {
	Type      string `json:"type" description:"Authentication type: none, sasl_plaintext, sasl_ssl, ssl"`
	Username  string `json:"username,omitempty" description:"SASL username"`
	Password  string `json:"password,omitempty" description:"SASL password"`
	Mechanism string `json:"mechanism,omitempty" description:"SASL mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512"`
}

// TLS configuration
type TLSConfig struct {
	Enabled    bool   `json:"enabled" description:"Whether to enable TLS"`
	CertPath   string `json:"cert_path,omitempty" description:"Path to TLS certificate file"`
	KeyPath    string `json:"key_path,omitempty" description:"Path to TLS key file"`
	CACertPath string `json:"ca_cert_path,omitempty" description:"Path to TLS CA certificate file"`
	SkipVerify bool   `json:"skip_verify,omitempty" description:"Skip TLS verification"`
}

// Schema Registry configuration
type SchemaRegistryConfig struct {
	URL     string            `json:"url" description:"Schema Registry URL"`
	Config  map[string]string `json:"config,omitempty" description:"Additional Schema Registry configuration"`
	Enabled bool              `json:"enabled" description:"Whether to use Schema Registry"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
bootstrap_servers: "kafka-1.prod.com:9092,kafka-2.prod.com:9092"
client_id: "marmot-discovery"
authentication:
  type: "sasl_ssl"
  username: "marmot-service"
  password: "kafka_secret_789"
  mechanism: "PLAIN"
tls:
  enabled: true
tags:
  - "kafka"
  - "streaming"
`

type Source struct {
	config         *Config
	client         *kgo.Client
	admin          *kadm.Client
	schemaRegistry schemaregistry.Client
}

func (c *Config) ApplyDefaults() {
	c.IncludePartitionInfo = true
	c.IncludeTopicConfig = true

	if c.TLS == nil {
		c.TLS = &TLSConfig{
			Enabled: true,
		}
	}
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	config.ApplyDefaults()

	if config.BootstrapServers == "" {
		return fmt.Errorf("bootstrap_servers is required")
	}

	if config.Authentication != nil {
		sanitizedConfig := rawConfig.MaskSensitiveInfo(config.Authentication.Password)
		log.Debug().Interface("raw_config", sanitizedConfig).Msg("Starting Kafka config validation")

		authType := config.Authentication.Type
		switch authType {
		case "sasl_plaintext", "sasl_ssl", "ssl":
			if authType == "sasl_plaintext" || authType == "sasl_ssl" {
				if config.Authentication.Username == "" {
					return fmt.Errorf("username is required for %s authentication", authType)
				}
				if config.Authentication.Password == "" {
					return fmt.Errorf("password is required for %s authentication", authType)
				}
				if config.Authentication.Mechanism == "" {
					return fmt.Errorf("mechanism is required for %s authentication", authType)
				}
			}
		case "none", "":
		default:
			return fmt.Errorf("unsupported authentication type: %s. Valid types are: sasl_plaintext, sasl_ssl, ssl, none", authType)
		}
	}

	s.config = config
	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	if err := s.Validate(pluginConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	if err := s.initClient(ctx); err != nil {
		return nil, fmt.Errorf("initializing Kafka client: %w", err)
	}
	defer s.closeClient()

	if s.config.SchemaRegistry != nil && s.config.SchemaRegistry.Enabled {
		if err := s.initSchemaRegistry(); err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Schema Registry client")
		}
	}

	topics, err := s.discoverTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering topics: %w", err)
	}

	var assets []asset.Asset
	for _, topic := range topics {
		if s.config.TopicFilter != nil && !plugin.ShouldIncludeResource(topic, *s.config.TopicFilter) {
			log.Debug().Str("topic", topic).Msg("Skipping topic due to filter")
			continue
		}

		asset, err := s.createTopicAsset(ctx, topic)
		if err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("Failed to create asset for topic")
			continue
		}
		assets = append(assets, asset)
	}

	return &plugin.DiscoveryResult{
		Assets: assets,
	}, nil
}
