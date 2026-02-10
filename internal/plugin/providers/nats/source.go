// +marmot:name=NATS
// +marmot:description=Discovers JetStream streams from NATS servers.
// +marmot:status=experimental
// +marmot:features=Assets
package nats

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

// Config for NATS plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection options
	Host            string `json:"host" description:"NATS server hostname or IP address" validate:"required"`
	Port            int    `json:"port,omitempty" description:"NATS server port" default:"4222" validate:"omitempty,min=1,max=65535"`
	Token           string `json:"token,omitempty" description:"Authentication token" sensitive:"true"`
	Username        string `json:"username,omitempty" description:"Username for authentication"`
	Password        string `json:"password,omitempty" description:"Password for authentication" sensitive:"true"`
	CredentialsFile string `json:"credentials_file,omitempty" description:"Path to NATS credentials file (.creds)"`
	TLS             bool   `json:"tls,omitempty" description:"Enable TLS connection"`
	TLSInsecure     bool   `json:"tls_insecure,omitempty" description:"Skip TLS certificate verification"`

}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "localhost"
port: 4222
token: "s3cr3t"
filter:
  include:
    - "^ORDERS"
tags:
  - "nats"
  - "messaging"
`

type Source struct {
	config *Config
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Port == 0 {
		config.Port = 4222
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, _ plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	nc, err := s.connect()
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS: %w", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("creating JetStream context: %w", err)
	}

	var assets []asset.Asset

	streams := js.ListStreams(ctx)
	for info := range streams.Info() {
		a := s.createStreamAsset(info)
		assets = append(assets, a)
	}
	if err := streams.Err(); err != nil {
		return nil, fmt.Errorf("listing streams: %w", err)
	}

	return &plugin.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) connect() (*nats.Conn, error) {
	addr := fmt.Sprintf("nats://%s:%d", s.config.Host, s.config.Port)

	opts := []nats.Option{
		nats.Timeout(10 * time.Second),
		nats.Name("marmot-discovery"),
	}

	if s.config.Token != "" {
		opts = append(opts, nats.Token(s.config.Token))
	}

	if s.config.Username != "" && s.config.Password != "" {
		opts = append(opts, nats.UserInfo(s.config.Username, s.config.Password))
	}

	if s.config.CredentialsFile != "" {
		opts = append(opts, nats.UserCredentials(s.config.CredentialsFile))
	}

	if s.config.TLS {
		opts = append(opts, nats.Secure(&tls.Config{
			InsecureSkipVerify: s.config.TLSInsecure,
		}))
	}

	return nats.Connect(addr, opts...)
}

func (s *Source) createStreamAsset(info *jetstream.StreamInfo) asset.Asset {
	metadata := make(map[string]interface{})

	metadata["stream_name"] = info.Config.Name
	metadata["subjects"] = strings.Join(info.Config.Subjects, ", ")
	metadata["retention_policy"] = info.Config.Retention.String()
	metadata["max_bytes"] = info.Config.MaxBytes
	metadata["max_msgs"] = info.Config.MaxMsgs
	metadata["max_age"] = info.Config.MaxAge.String()
	metadata["max_msg_size"] = int64(info.Config.MaxMsgSize)
	metadata["storage_type"] = info.Config.Storage.String()
	metadata["num_replicas"] = info.Config.Replicas
	metadata["duplicate_window"] = info.Config.Duplicates.String()
	metadata["discard_policy"] = info.Config.Discard.String()

	metadata["messages"] = info.State.Msgs
	metadata["bytes"] = info.State.Bytes
	metadata["consumer_count"] = info.State.Consumers
	metadata["first_seq"] = info.State.FirstSeq
	metadata["last_seq"] = info.State.LastSeq

	metadata["host"] = s.config.Host
	metadata["port"] = s.config.Port

	streamName := info.Config.Name
	mrnValue := mrn.New("Stream", "NATS", streamName)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:      &streamName,
		MRN:       &mrnValue,
		Type:      "Stream",
		Providers: []string{"NATS"},
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "NATS",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "nats",
		Name:        "NATS",
		Description: "Discover JetStream streams from NATS servers",
		Icon:        "nats",
		Category:    "messaging",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register NATS plugin")
	}
}
