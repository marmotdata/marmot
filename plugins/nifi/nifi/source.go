// Package nifi discovers process groups, processors and the connections
// between them from Apache NiFi, and links processors to the buckets,
// topics, tables and indexes they read or write.
package nifi

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string on every NiFi asset.
const provider = "NiFi"

// Config for the NiFi plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"NiFi base URL (e.g. https://nifi.example.com:8443)" validate:"required,url"`
	Username string `json:"username,omitempty" description:"Username for single-user or LDAP login. Leave all credentials empty for an unsecured HTTP install"`
	Password string `json:"password,omitempty" description:"Password for the username" sensitive:"true"`
	Token    string `json:"token,omitempty" description:"Bearer token to use instead of a username and password" sensitive:"true"`

	ClientCert string `json:"client_cert,omitempty" label:"Client Certificate" description:"Path to a PEM client certificate for mutual TLS (used instead of a login)"`
	ClientKey  string `json:"client_key,omitempty" label:"Client Key" description:"Path to the PEM private key of the client certificate"`
	CACert     string `json:"ca_cert,omitempty" label:"CA Certificate" description:"Path to a PEM CA certificate to trust"`
	VerifySSL  bool   `json:"verify_ssl" label:"Verify SSL" description:"Verify the NiFi TLS certificate. Set to false for the self-signed certificate NiFi ships with" default:"true"`

	IncludeProcessors bool   `json:"include_processors" description:"Discover processors as Task assets" default:"true"`
	IncludePorts      bool   `json:"include_ports" description:"Discover input and output ports as Task assets" default:"false"`
	DiscoverLineage   bool   `json:"discover_lineage" description:"Link well-known processors to the buckets, topics, tables and indexes they read or write" default:"true"`
	RootProcessGroup  string `json:"root_process_group,omitempty" description:"Id of the process group to start from (defaults to the root group)"`
}

// Example configuration for the plugin
var _ = `
host: "https://nifi.example.com:8443"
username: "marmot"
password: "marmot-password"
verify_ssl: false
include_processors: true
include_ports: false
discover_lineage: true
tags:
  - "nifi"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "nifi",
		Name:        "Apache NiFi",
		Description: "Discover process groups, processors and data flow lineage from Apache NiFi",
		Icon:        "nifi",
		Category:    "orchestration",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source implements the NiFi plugin.
type Source struct {
	config *Config
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)
	config.Host = strings.TrimSuffix(config.Host, "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	// The url rule accepts "nifi.example.com:8443" (scheme "nifi.example.com"),
	// which would only fail later with a confusing transport error.
	if u, err := url.Parse(config.Host); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("host must start with http:// or https://")
	}

	if (config.Username == "") != (config.Password == "") {
		return nil, fmt.Errorf("username and password must be set together")
	}
	if (config.ClientCert == "") != (config.ClientKey == "") {
		return nil, fmt.Errorf("client_cert and client_key must be set together")
	}

	s.config = config
	return rawConfig, nil
}

// Discover walks the process group tree and emits pipelines, tasks and
// lineage.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	c, err := newClient(s.config)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	// A configured token or client certificate already authenticates every
	// request; only a username and password need exchanging for a token.
	if s.config.Token == "" && s.config.Username != "" {
		if err := c.login(ctx, s.config.Username, s.config.Password); err != nil {
			return nil, err
		}
	}

	d := newDiscovery(s.config, c)
	if err := d.run(ctx); err != nil {
		return nil, err
	}

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Msg("NiFi discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  d.assets,
		Lineage: d.lineage,
	}, nil
}

// assetMRN is the single place a NiFi MRN is built, so pipelines, tasks
// and every edge between them address an asset the same way. The name
// is the path from the root group, which is what keeps two processors
// called "LogAttribute" in different groups apart.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// nativeMRN addresses an asset another plugin owns (a bucket, topic or
// table) by that plugin's provider and name shape, so the edge lands on
// the asset it catalogued rather than on a NiFi-flavoured duplicate.
func nativeMRN(assetType, nativeProvider, name string) string {
	return mrn.New(assetType, nativeProvider, name)
}
