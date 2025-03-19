// +marmot:name=Kafka
// +marmot:description=This plugin discovers Kafka topics from Kafka clusters.
// +marmot:status=experimental
package kafka

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Config for Kafka plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection configuration
	BootstrapServers string            `json:"bootstrap_servers" yaml:"bootstrap_servers" description:"Comma-separated list of bootstrap servers"`
	ClientID         string            `json:"client_id" yaml:"client_id" description:"Client ID for the consumer"`
	Authentication   *AuthConfig       `json:"authentication,omitempty" yaml:"authentication,omitempty" description:"Authentication configuration"`
	ConsumerConfig   map[string]string `json:"consumer_config,omitempty" yaml:"consumer_config,omitempty" description:"Additional consumer configuration"`
	ClientTimeout    int               `json:"client_timeout_seconds" yaml:"client_timeout_seconds" description:"Request timeout in seconds"`

	// Schema Registry configuration (optional)
	SchemaRegistry *SchemaRegistryConfig `json:"schema_registry,omitempty" yaml:"schema_registry,omitempty" description:"Schema Registry configuration"`

	// Topic patterns for filtering
	TopicFilter *plugin.Filter `json:"topic_filter,omitempty" yaml:"topic_filter,omitempty" description:"Filter configuration for topics"`

	// Metadata extraction
	IncludePartitionInfo bool `json:"include_partition_info" yaml:"include_partition_info" description:"Whether to include partition information in metadata"`
	IncludeTopicConfig   bool `json:"include_topic_config" yaml:"include_topic_config" description:"Whether to include topic configuration in metadata"`
}

// Authentication configuration
type AuthConfig struct {
	Type          string `json:"type" yaml:"type" description:"Authentication type: none, sasl_plaintext, sasl_ssl, ssl"`
	Username      string `json:"username,omitempty" yaml:"username,omitempty" description:"SASL username"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty" description:"SASL password"`
	Mechanism     string `json:"mechanism,omitempty" yaml:"mechanism,omitempty" description:"SASL mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512"`
	TLSCertPath   string `json:"tls_cert_path,omitempty" yaml:"tls_cert_path,omitempty" description:"Path to TLS certificate file"`
	TLSKeyPath    string `json:"tls_key_path,omitempty" yaml:"tls_key_path,omitempty" description:"Path to TLS key file"`
	TLSCACertPath string `json:"tls_ca_cert_path,omitempty" yaml:"tls_ca_cert_path,omitempty" description:"Path to TLS CA certificate file"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty" yaml:"tls_skip_verify,omitempty" description:"Skip TLS verification"`
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

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	log.Debug().Interface("raw_config", rawConfig).Msg("Starting Kafka config validation")

	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.BootstrapServers == "" {
		return fmt.Errorf("bootstrap_servers is required")
	}

	s.config = config
	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	if err := s.Validate(pluginConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	// Initialize Kafka client
	if err := s.initClient(ctx); err != nil {
		return nil, fmt.Errorf("initializing Kafka client: %w", err)
	}
	defer s.closeClient()

	// Initialize Schema Registry client if configured
	if s.config.SchemaRegistry != nil && s.config.SchemaRegistry.Enabled {
		if err := s.initSchemaRegistry(); err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Schema Registry client")
		}
	}

	// Discover topics
	topics, err := s.discoverTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering topics: %w", err)
	}

	var assets []asset.Asset
	for _, topic := range topics {
		// Apply filter if configured
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

func (s *Source) initClient(ctx context.Context) error {
	// Setup client options
	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(s.config.BootstrapServers, ",")...),
	}

	if s.config.ClientID != "" {
		opts = append(opts, kgo.ClientID(s.config.ClientID))
	}

	if s.config.ClientTimeout > 0 {
		timeout := time.Duration(s.config.ClientTimeout) * time.Second
		opts = append(opts, kgo.RequestTimeoutOverhead(timeout))
	}

	// Configure authentication
	if s.config.Authentication != nil {
		authOpts, err := s.configureAuthentication()
		if err != nil {
			return fmt.Errorf("configuring authentication: %w", err)
		}
		opts = append(opts, authOpts...)
	}

	// Create Kafka client
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("creating Kafka client: %w", err)
	}
	s.client = client

	// Create admin client
	s.admin = kadm.NewClient(client)

	return nil
}

func (s *Source) initSchemaRegistry() error {
	if s.config.SchemaRegistry.URL == "" {
		return fmt.Errorf("schema registry URL is required")
	}

	// Create configuration for schema registry client
	conf := schemaregistry.NewConfig(s.config.SchemaRegistry.URL)

	// Apply custom schema registry config for authentication
	if userInfo, ok := s.config.SchemaRegistry.Config["basic.auth.user.info"]; ok {
		conf.BasicAuthUserInfo = userInfo
	}

	// Apply other custom configs
	if timeout, ok := s.config.SchemaRegistry.Config["request.timeout.ms"]; ok {
		if val, err := strconv.Atoi(timeout); err == nil {
			conf.RequestTimeoutMs = val
		}
	}

	if cacheCapacity, ok := s.config.SchemaRegistry.Config["cache.capacity"]; ok {
		if val, err := strconv.Atoi(cacheCapacity); err == nil {
			conf.CacheCapacity = val
		}
	}

	// Create Schema Registry client
	client, err := schemaregistry.NewClient(conf)
	if err != nil {
		return fmt.Errorf("creating Schema Registry client: %w", err)
	}

	s.schemaRegistry = client
	return nil
}

func (s *Source) configureAuthentication() ([]kgo.Opt, error) {
	var opts []kgo.Opt

	switch s.config.Authentication.Type {
	case "sasl_plaintext":
		var mechanism sasl.Mechanism

		switch s.config.Authentication.Mechanism {
		case "PLAIN":
			mechanism = plain.Auth{
				User: s.config.Authentication.Username,
				Pass: s.config.Authentication.Password,
			}.AsMechanism()
		case "SCRAM-SHA-256":
			mechanism = scram.Auth{
				User: s.config.Authentication.Username,
				Pass: s.config.Authentication.Password,
			}.AsSha256Mechanism()
		case "SCRAM-SHA-512":
			mechanism = scram.Auth{
				User: s.config.Authentication.Username,
				Pass: s.config.Authentication.Password,
			}.AsSha512Mechanism()
		default:
			return nil, fmt.Errorf("unsupported SASL mechanism: %s", s.config.Authentication.Mechanism)
		}

		opts = append(opts, kgo.SASL(mechanism))

	case "sasl_ssl":
		var mechanism sasl.Mechanism

		switch s.config.Authentication.Mechanism {
		case "PLAIN":
			mechanism = plain.Auth{
				User: s.config.Authentication.Username,
				Pass: s.config.Authentication.Password,
			}.AsMechanism()
		case "SCRAM-SHA-256":
			mechanism = scram.Auth{
				User: s.config.Authentication.Username,
				Pass: s.config.Authentication.Password,
			}.AsSha256Mechanism()
		case "SCRAM-SHA-512":
			mechanism = scram.Auth{
				User: s.config.Authentication.Username,
				Pass: s.config.Authentication.Password,
			}.AsSha512Mechanism()
		default:
			return nil, fmt.Errorf("unsupported SASL mechanism: %s", s.config.Authentication.Mechanism)
		}

		opts = append(opts, kgo.SASL(mechanism))

		// Setup TLS
		tlsConfig, err := s.configureTLS()
		if err != nil {
			return nil, fmt.Errorf("configuring TLS: %w", err)
		}
		opts = append(opts, kgo.DialTLSConfig(tlsConfig))

	case "ssl":
		// Setup TLS
		tlsConfig, err := s.configureTLS()
		if err != nil {
			return nil, fmt.Errorf("configuring TLS: %w", err)
		}
		opts = append(opts, kgo.DialTLSConfig(tlsConfig))

	case "none", "":
		// No authentication

	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", s.config.Authentication.Type)
	}

	return opts, nil
}

func (s *Source) configureTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if s.config.Authentication.TLSSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	// Configure CA certificate
	if s.config.Authentication.TLSCACertPath != "" {
		caCert, err := os.ReadFile(s.config.Authentication.TLSCACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert file: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert to pool")
		}
		tlsConfig.RootCAs = certPool
	}

	// Configure client certificate if provided
	if s.config.Authentication.TLSCertPath != "" && s.config.Authentication.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(
			s.config.Authentication.TLSCertPath,
			s.config.Authentication.TLSKeyPath,
		)
		if err != nil {
			return nil, fmt.Errorf("loading client cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func (s *Source) closeClient() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *Source) discoverTopics(ctx context.Context) ([]string, error) {
	metadata, err := s.admin.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}

	var topics []string
	for topic := range metadata {
		topics = append(topics, topic)
	}

	return topics, nil
}

func (s *Source) createTopicAsset(ctx context.Context, topic string) (asset.Asset, error) {
	// Initialize metadata map
	metadata := make(map[string]interface{})
	metadata["topic_name"] = topic

	// Get topic details
	topicDetails, err := s.getTopicDetails(ctx, topic)
	if err != nil {
		return asset.Asset{}, fmt.Errorf("getting topic details: %w", err)
	}

	// Add partition information
	if s.config.IncludePartitionInfo {
		metadata["partition_count"] = topicDetails.partitionCount
		metadata["replication_factor"] = topicDetails.replicationFactor
	}

	// Get topic configuration
	if s.config.IncludeTopicConfig {
		topicConfig, err := s.getTopicConfig(ctx, topic)
		if err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("Failed to get topic config")
		} else {
			for key, value := range topicConfig {
				metadata[key] = value
			}
		}
	}

	// Add schema information if Schema Registry is enabled
	if s.schemaRegistry != nil {
		if err := s.enrichWithSchemaRegistry(topic, metadata); err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("Failed to get schema information")
		}
	}

	description := fmt.Sprintf("Kafka topic %s", topic)
	mrnValue := mrn.New("Topic", "Kafka", topic)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &topic,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Kafka"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Kafka",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

// TopicDetails holds information about a Kafka topic
type TopicDetails struct {
	partitionCount    int
	replicationFactor int
}

func (s *Source) getTopicDetails(ctx context.Context, topic string) (*TopicDetails, error) {
	metadata, err := s.admin.ListTopics(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("describing topic: %w", err)
	}

	topicMetadata, exists := metadata[topic]
	if !exists {
		return nil, fmt.Errorf("topic %s not found in metadata", topic)
	}

	partitionCount := len(topicMetadata.Partitions)

	// Get replication factor from first partition (should be the same for all)
	var replicationFactor int
	if partitionCount > 0 {
		replicationFactor = len(topicMetadata.Partitions[0].Replicas)
	}

	return &TopicDetails{
		partitionCount:    partitionCount,
		replicationFactor: replicationFactor,
	}, nil
}

func (s *Source) getTopicConfig(ctx context.Context, topic string) (map[string]string, error) {
	configs, err := s.admin.DescribeTopicConfigs(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("describing topic configs: %w", err)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no config found for topic %s", topic)
	}

	configMap := make(map[string]string)
	for _, config := range configs[0].Configs {
		if config.Source == kmsg.ConfigSourceDefaultConfig {
			continue
		}
		if config.Value != nil {
			configMap[config.Key] = *config.Value
		}
	}

	return configMap, nil
}

// ConsumerGroupDetails holds information about a Kafka consumer group
type ConsumerGroupDetails struct {
	State        string
	Protocol     string
	ProtocolType string
	Members      []ConsumerGroupMember
}

// ConsumerGroupMember holds information about a member of a consumer group
type ConsumerGroupMember struct {
	ClientID        string
	ClientHost      string
	TopicPartitions map[string][]int
}

func (s *Source) enrichWithSchemaRegistry(topic string, metadata map[string]interface{}) error {
	// Check for value schema
	valueSubject := topic + "-value"
	valueMetadata, err := s.schemaRegistry.GetLatestSchemaMetadata(valueSubject)
	if err == nil {
		metadata["value_schema_id"] = valueMetadata.ID
		metadata["value_schema_version"] = valueMetadata.Version
		metadata["value_schema_type"] = valueMetadata.SchemaType
		metadata["value_schema"] = valueMetadata.Schema
	}

	// Check for key schema
	keySubject := topic + "-key"
	keyMetadata, err := s.schemaRegistry.GetLatestSchemaMetadata(keySubject)
	if err == nil {
		metadata["key_schema_id"] = keyMetadata.ID
		metadata["key_schema_version"] = keyMetadata.Version
		metadata["key_schema_type"] = keyMetadata.SchemaType
		metadata["key_schema"] = keyMetadata.Schema
	}

	return nil
}

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
