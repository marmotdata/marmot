// Package kafka discovers topics from Apache Kafka clusters.
package kafka

import (
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Config for the Kafka plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	BootstrapServers string            `json:"bootstrap_servers" description:"Comma-separated list of bootstrap servers" validate:"required"`
	ClientID         string            `json:"client_id" label:"Client ID" description:"Client ID for the consumer"`
	Authentication   *AuthConfig       `json:"authentication,omitempty" description:"Authentication configuration"`
	ConsumerConfig   map[string]string `json:"consumer_config,omitempty" description:"Additional consumer configuration"`
	ClientTimeout    int               `json:"client_timeout_seconds" description:"Request timeout in seconds" validate:"omitempty,min=1,max=300"`
	TLS              *TLSConfig        `json:"tls,omitempty" description:"TLS configuration"`

	SchemaRegistry *SchemaRegistryConfig `json:"schema_registry,omitempty" description:"Schema Registry configuration"`

	IncludePartitionInfo bool `json:"include_partition_info" description:"Whether to include partition information in metadata" default:"true"`
	IncludeTopicConfig   bool `json:"include_topic_config" description:"Whether to include topic configuration in metadata" default:"true"`
}

// AuthConfig defines Kafka client authentication.
type AuthConfig struct {
	Type      string `json:"type" description:"Authentication type: none, sasl_plaintext, sasl_ssl, ssl" validate:"omitempty,oneof=none sasl_plaintext sasl_ssl ssl"`
	Username  string `json:"username,omitempty" description:"SASL username"`
	Password  string `json:"password,omitempty" description:"SASL password" sensitive:"true"`
	Mechanism string `json:"mechanism,omitempty" description:"SASL mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512" validate:"omitempty,oneof=PLAIN SCRAM-SHA-256 SCRAM-SHA-512"`
}

// TLSConfig defines TLS options for the Kafka client.
type TLSConfig struct {
	Enabled    bool   `json:"enabled" description:"Whether to enable TLS"`
	CertPath   string `json:"cert_path,omitempty" description:"Path to TLS certificate file"`
	KeyPath    string `json:"key_path,omitempty" description:"Path to TLS key file"`
	CACertPath string `json:"ca_cert_path,omitempty" description:"Path to TLS CA certificate file"`
	SkipVerify bool   `json:"skip_verify,omitempty" description:"Skip TLS verification"`
}

// SchemaRegistryConfig defines the optional Schema Registry connection.
type SchemaRegistryConfig struct {
	URL        string            `json:"url" description:"Schema Registry URL" validate:"omitempty,url"`
	Config     map[string]string `json:"config,omitempty" description:"Additional Schema Registry configuration"`
	Enabled    bool              `json:"enabled" description:"Whether to use Schema Registry"`
	SkipVerify bool              `json:"skip_verify,omitempty" description:"Skip TLS certificate verification"`
}

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "kafka",
		Name:        "Kafka",
		Description: "Discover Kafka topics from Kafka clusters",
		Icon:        "kafka",
		Category:    "streaming",
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source is the Kafka discovery source. It is exported so alias plugins
// (Confluent Cloud, Redpanda) can reuse it verbatim.
type Source struct {
	config         *Config
	client         *kgo.Client
	admin          *kadm.Client
	schemaRegistry schemaregistry.Client
}

// ApplyDefaults sets default values on the config. The default:"" struct
// tags are documentation only; runtime defaults must be set explicitly
// because Go zero-values collide with real-world false/0.
func (c *Config) ApplyDefaults() {
	c.IncludePartitionInfo = true
	c.IncludeTopicConfig = true

	if c.TLS == nil {
		c.TLS = &TLSConfig{
			Enabled: true,
		}
	}
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	config.ApplyDefaults()

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	if config.Authentication != nil {
		authType := config.Authentication.Type
		if authType == "sasl_plaintext" || authType == "sasl_ssl" {
			if config.Authentication.Username == "" {
				return nil, fmt.Errorf("username is required for %s authentication", authType)
			}
			if config.Authentication.Password == "" {
				return nil, fmt.Errorf("password is required for %s authentication", authType)
			}
			if config.Authentication.Mechanism == "" {
				return nil, fmt.Errorf("mechanism is required for %s authentication", authType)
			}
		}
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	if _, err := s.Validate(rawConfig); err != nil {
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

	var assets []pluginsdk.Asset
	for _, topic := range topics {
		asset, err := s.createTopicAsset(ctx, topic)
		if err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("Failed to create asset for topic")
			continue
		}
		assets = append(assets, asset)
	}

	return &pluginsdk.DiscoveryResult{
		Assets: assets,
	}, nil
}
