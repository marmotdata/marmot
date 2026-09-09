// Package amundsen catalogues the contents of an Amundsen metadata
// graph in Marmot.
//
// Amundsen is itself a catalog, so everything in it describes something
// that lives somewhere else: a table under its "postgres" database is a
// Postgres table. This plugin reads Amundsen's Neo4j graph over Bolt and
// projects every entry onto the Marmot provider and MRN its native
// Marmot plugin would use, so a catalog imported from Amundsen looks the
// same as one built by Marmot's own plugins, and running both merges
// rather than duplicates. See projection.go for that mapping.
package amundsen

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "amundsen",
		Name:        "Amundsen",
		Description: "Import tables, columns, owners, usage, dashboards and lineage from an Amundsen metadata graph",
		// iconloader.ts has no amundsen entry, so this borrows the
		// generic catalog icon the OpenMetadata import already uses.
		Icon:       "openmetadata",
		Category:   "catalog",
		Status:     "experimental",
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(AmundsenTableFields{}, "Table",
				"The metadata fields the plugin emits on a table or view asset."),
			pluginsdk.AssetSchemaOf(AmundsenColumnFields{}, "Column",
				"The per-column fields embedded in a table asset's schema."),
			pluginsdk.AssetSchemaOf(AmundsenDashboardFields{}, "Dashboard",
				"The metadata fields the plugin emits on a dashboard asset."),
			pluginsdk.AssetSchemaOf(AmundsenChartFields{}, "Chart",
				"The metadata fields the plugin emits on a chart asset."),
		},
	}
}

// Config for the Amundsen plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	// Connection
	URI      string `json:"uri" label:"URI" description:"Bolt address of Amundsen's Neo4j, for example bolt://neo4j.company.com:7687" validate:"required"`
	Username string `json:"username" description:"Neo4j username" validate:"required"`
	Password string `json:"password" description:"Neo4j password" sensitive:"true" validate:"required"`
	Database string `json:"database" description:"Neo4j database holding the Amundsen graph" default:"neo4j"`
	// Neo4j decides encryption from the URI scheme, so these two rewrite
	// it rather than being passed to the driver.
	Encrypted            bool `json:"encrypted" description:"Connect over TLS" default:"false"`
	TrustAllCertificates bool `json:"trust_all_certificates" description:"Accept any TLS certificate, including self signed ones" default:"false"`

	// Links
	AmundsenURL string `json:"amundsen_url" label:"Amundsen URL" description:"Address of the Amundsen web app, used to link each asset back to its page" validate:"omitempty,url"`

	// What to import
	IncludeUsers        bool `json:"include_users" description:"Record table owners from Amundsen" default:"true"`
	IncludeDashboards   bool `json:"include_dashboards" description:"Import dashboards and their charts" default:"true"`
	IncludeTags         bool `json:"include_tags" description:"Copy Amundsen tags onto assets" default:"true"`
	IncludeDescriptions bool `json:"include_descriptions" description:"Copy Amundsen descriptions onto assets and columns" default:"true"`
	IncludeUsage        bool `json:"include_usage" description:"Import Amundsen read counts as statistics" default:"true"`

	// Performance
	QueryTimeoutSeconds int `json:"query_timeout_seconds" description:"Per-query timeout" default:"120" validate:"min=1"`
	PageSize            int `json:"page_size" description:"Records per query. Every query is paged, so a large graph does not have to fit in memory" default:"1000" validate:"min=1,max=100000"`
}

// Example configuration for the plugin
var _ = `
uri: "bolt://neo4j.company.com:7687"
username: "neo4j"
password: "secret"
amundsen_url: "https://amundsen.company.com"
include_dashboards: true
tags:
  - "amundsen"
`

// Source represents the Amundsen plugin.
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

	config.URI = strings.TrimSpace(config.URI)
	config.AmundsenURL = strings.TrimRight(strings.TrimSpace(config.AmundsenURL), "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	// The scheme decides whether the connection is routed and encrypted,
	// so a typo here fails as a confusing connection error much later.
	if _, err := boltURI(config.URI, config.Encrypted, config.TrustAllCertificates); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover reads the Amundsen graph and returns the assets, lineage and
// usage statistics it describes.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	client, err := newClient(ctx, s.config)
	if err != nil {
		return nil, err
	}
	defer client.close(ctx)

	c := newCollector(s.config)
	if err := c.collect(ctx, client); err != nil {
		return nil, err
	}

	log.Info().
		Int("assets", len(c.assets)).
		Int("lineage", len(c.lineage)).
		Int("statistics", len(c.statistics)).
		Msg("Amundsen discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     c.assets,
		Lineage:    c.lineage,
		Statistics: c.statistics,
	}, nil
}

// collect runs each reader in turn. Owners come first because a table
// asset carries them, and the two lineage passes come last because they
// resolve Amundsen keys against the assets the earlier passes produced.
func (c *collector) collect(ctx context.Context, r reader) error {
	if c.config.IncludeUsers {
		if err := c.discoverOwners(ctx, r); err != nil {
			return err
		}
	}

	if err := c.discoverTables(ctx, r); err != nil {
		return err
	}

	if err := c.discoverTableLineage(ctx, r); err != nil {
		return err
	}

	if c.config.IncludeDashboards {
		if err := c.discoverDashboards(ctx, r); err != nil {
			return err
		}
		if err := c.discoverDashboardLineage(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

// boltURI returns the address to hand the Neo4j driver. The Go driver
// takes encryption from the URI scheme rather than from a config field,
// so the encrypted and trust_all_certificates settings are applied by
// upgrading the scheme: bolt becomes bolt+s, or bolt+ssc when any
// certificate is acceptable. A URI that already names an encrypted
// scheme is left alone.
func boltURI(raw string, encrypted, trustAll bool) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing uri: %w", err)
	}

	if parsed.User != nil {
		return "", fmt.Errorf("uri must not carry credentials: put them in the username and password fields")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("uri %q has no host: expected something like bolt://host:7687", raw)
	}

	switch parsed.Scheme {
	case "bolt+s", "bolt+ssc", "neo4j+s", "neo4j+ssc":
		return parsed.String(), nil
	case "bolt", "neo4j":
		if !encrypted {
			return parsed.String(), nil
		}
		if trustAll {
			parsed.Scheme += "+ssc"
		} else {
			parsed.Scheme += "+s"
		}
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("uri scheme %q is not a Neo4j scheme: expected bolt, bolt+s, bolt+ssc, neo4j, neo4j+s or neo4j+ssc", parsed.Scheme)
	}
}
