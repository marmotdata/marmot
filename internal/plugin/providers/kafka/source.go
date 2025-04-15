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

	BootstrapServers string            `json:"bootstrap_servers" yaml:"bootstrap_servers" description:"Comma-separated list of bootstrap servers"`
	ClientID         string            `json:"client_id" yaml:"client_id" description:"Client ID for the consumer"`
	Authentication   *AuthConfig       `json:"authentication,omitempty" yaml:"authentication,omitempty" description:"Authentication configuration"`
	ConsumerConfig   map[string]string `json:"consumer_config,omitempty" yaml:"consumer_config,omitempty" description:"Additional consumer configuration"`
	ClientTimeout    int               `json:"client_timeout_seconds" yaml:"client_timeout_seconds" description:"Request timeout in seconds"`
	TLS              *TLSConfig        `json:"tls,omitempty" yaml:"tls,omitempty" description:"TLS configuration"`

	SchemaRegistry *SchemaRegistryConfig `json:"schema_registry,omitempty" yaml:"schema_registry,omitempty" description:"Schema Registry configuration"`

	TopicFilter *plugin.Filter `json:"topic_filter,omitempty" yaml:"topic_filter,omitempty" description:"Filter configuration for topics"`

	IncludePartitionInfo bool `json:"include_partition_info" yaml:"include_partition_info" description:"Whether to include partition information in metadata" default:"true"`

	IncludeTopicConfig bool `json:"include_topic_config" yaml:"include_topic_config" description:"Whether to include topic configuration in metadata" default:"true"`
}

// Authentication configuration
type AuthConfig struct {
	Type      string `json:"type" yaml:"type" description:"Authentication type: none, sasl_plaintext, sasl_ssl, ssl"`
	Username  string `json:"username,omitempty" yaml:"username,omitempty" description:"SASL username"`
	Password  string `json:"password,omitempty" yaml:"password,omitempty" description:"SASL password"`
	Mechanism string `json:"mechanism,omitempty" yaml:"mechanism,omitempty" description:"SASL mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512"`
}

// TLS configuration
type TLSConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled" description:"Whether to enable TLS"`
	CertPath   string `json:"cert_path,omitempty" yaml:"cert_path,omitempty" description:"Path to TLS certificate file"`
	KeyPath    string `json:"key_path,omitempty" yaml:"key_path,omitempty" description:"Path to TLS key file"`
	CACertPath string `json:"ca_cert_path,omitempty" yaml:"ca_cert_path,omitempty" description:"Path to TLS CA certificate file"`
	SkipVerify bool   `json:"skip_verify,omitempty" yaml:"skip_verify,omitempty" description:"Skip TLS verification"`
}

// Schema Registry configuration
type SchemaRegistryConfig struct {
	URL     string            `json:"url" yaml:"url" description:"Schema Registry URL"`
	Config  map[string]string `json:"config,omitempty" yaml:"config,omitempty" description:"Additional Schema Registry configuration"`
	Enabled bool              `json:"enabled" yaml:"enabled" description:"Whether to use Schema Registry"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
bootstrap_servers: "localhost:9092"
client_id: "marmot-kafka-plugin"
client_timeout_seconds: 60
authentication:
  type: "sasl_plaintext"
  username: "username"
  password: "password"
  mechanism: "PLAIN"
tls:
  enabled: true
  cert_path: "/path/to/cert.pem"
  key_path: "/path/to/key.pem"
  ca_cert_path: "/path/to/ca.pem"
  skip_verify: false
schema_registry:
  url: "http://localhost:8081"
  enabled: true
  config:
    basic.auth.user.info: "username:password"
topic_filter:
  include:
    - "^prod-.*"
    - "^staging-.*"
  exclude:
    - ".*-test$"
    - ".*-dev$"
include_partition_info: true
include_topic_config: true
tags:
  - "kafka"
  - "messaging"
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
