// Package cockroachdb discovers databases, tables, views and foreign key
// relationships from CockroachDB clusters.
package cockroachdb

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact string the Marmot server files these assets under.
// The OpenMetadata plugin uses the same word for its Cockroach services so
// both runs land on one asset.
const provider = "CockroachDB"

// Config for the CockroachDB plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"CockroachDB node hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"SQL port" default:"26257" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"SQL user" validate:"required"`
	Password string `json:"password" description:"Password for the SQL user. Leave empty on insecure clusters or with certificate authentication" sensitive:"true"`

	Database         string   `json:"database,omitempty" group:"Discovery" description:"Discover only this database. Leave empty to discover every database"`
	ExcludeDatabases []string `json:"exclude_databases,omitempty" group:"Discovery" description:"Databases to skip when discovering every database" default:"[\"system\"]"`

	SSLMode     string `json:"ssl_mode" group:"Authentication" label:"SSL Mode" description:"SSL mode (disable, require, verify-ca, verify-full)" default:"disable" validate:"omitempty,oneof=disable require verify-ca verify-full"`
	SSLRootCert string `json:"ssl_root_cert,omitempty" group:"Authentication" label:"SSL Root Certificate" description:"Path to the CA certificate that signed the node certificates"`
	SSLCert     string `json:"ssl_cert,omitempty" group:"Authentication" label:"SSL Client Certificate" description:"Path to the client certificate, for certificate authentication"`
	SSLKey      string `json:"ssl_key,omitempty" group:"Authentication" label:"SSL Client Key" description:"Path to the client certificate key"`

	IncludeColumns      bool `json:"include_columns" group:"Discovery" description:"Whether to include column information in table metadata" default:"true"`
	IncludeViews        bool `json:"include_views" group:"Discovery" description:"Whether to discover views and materialized views" default:"true"`
	DiscoverForeignKeys bool `json:"discover_foreign_keys" group:"Discovery" description:"Whether to discover foreign key relationships" default:"true"`
	IncludeStatistics   bool `json:"include_statistics" group:"Advanced" description:"Whether to include row counts, column counts and table sizes" default:"true"`

	// QueryDatabase is the database data queries connect to. It defaults to
	// Database when discovery is scoped to one database, or to defaultdb when
	// discovery spans the whole cluster, since that default mirrors the
	// bootstrap database listDatabases itself falls back to.
	QueryDatabase string `json:"query_database,omitempty" group:"Discovery" label:"Query Database" description:"Database that data queries run against when this source answers queries. Defaults to the configured database, or defaultdb when every database is discovered" default:"defaultdb"`
}

// Example configuration for the plugin
var _ = `
host: "crdb-prod.internal"
port: 26257
user: "marmot_reader"
password: "secure_password_123"
ssl_mode: "verify-full"
ssl_root_cert: "/etc/marmot/certs/ca.crt"
exclude_databases:
  - "system"
  - "defaultdb"
tags:
  - "cockroachdb"
  - "production"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:            "cockroachdb",
		MarmotVersion: ">=0.13.0",
		Name:          "CockroachDB",
		Description:   "Discover databases, tables, views and foreign key relationships from CockroachDB clusters",
		Icon:          "cockroachdb",
		Category:      "database",
		Status:        "experimental",
		Features:      []string{"Assets", "Lineage"},
		ConfigSpec:    pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(CockroachDBDatabaseFields{}, "Database",
				"The metadata fields the plugin emits for database assets."),
			pluginsdk.AssetSchemaOf(CockroachDBTableFields{}, "Table",
				"The metadata fields the plugin emits for table and view assets."),
			pluginsdk.AssetSchemaOf(CockroachDBColumnFields{}, "Column",
				"The per-column fields embedded in an asset's schema."),
		},
	}
}

// Source represents the CockroachDB plugin.
type Source struct {
	config *Config

	// query is the connection pool data queries share across calls.
	query queryPools
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := parseConfig(rawConfig)
	if err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// parseConfig decodes, defaults and validates a raw config without touching
// the Source, so concurrent data query calls can each work from their own
// copy rather than the config the host last validated through Validate.
func parseConfig(rawConfig pluginsdk.RawConfig) (*Config, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// The static default tag cannot express "fall back to Database", so a
	// single-database setup is handled here once the tag has already filled
	// in defaultdb for the common, cluster-wide case.
	if _, ok := rawConfig["query_database"]; !ok && config.Database != "" {
		config.QueryDatabase = config.Database
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	return config, nil
}

// Discover discovers databases, tables, views and their relationships.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	log.Debug().Str("host", s.config.Host).Int("port", s.config.Port).Msg("Starting CockroachDB discovery")

	databases, serverVersion, err := s.listDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	for _, db := range databases {
		dbAsset := s.databaseAsset(db, serverVersion)
		assets = append(assets, dbAsset)

		result, err := s.discoverDatabase(ctx, db.Name)
		if err != nil {
			log.Warn().Err(err).Str("database", db.Name).Msg("Failed to discover database")
			continue
		}

		assets = append(assets, result.assets...)
		for _, a := range result.assets {
			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: *dbAsset.MRN,
				Target: *a.MRN,
				Type:   "CONTAINS",
			})
		}
		lineages = append(lineages, result.lineage...)
		statistics = append(statistics, result.statistics...)

		log.Debug().
			Str("database", db.Name).
			Int("objects", len(result.assets)).
			Int("lineages", len(result.lineage)).
			Msg("Discovered database")
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("CockroachDB discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
		ZoneBindings: []pluginsdk.ZoneBinding{{
			Provider: provider,
			Host:     strings.ToLower(net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))),
			Zone:     s.config.Zone,
		}},
	}, nil
}

// selectDatabases narrows the cluster's databases to the ones discovery
// should visit. An explicit database wins over the exclusion list, and asking
// for one that does not exist is an error rather than an empty run.
func selectDatabases(all []database, only string, exclude []string) ([]database, error) {
	if only != "" {
		for _, db := range all {
			if db.Name == only {
				return []database{db}, nil
			}
		}
		return nil, fmt.Errorf("database %q not found", only)
	}

	excluded := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		excluded[name] = struct{}{}
	}

	var selected []database
	for _, db := range all {
		if _, skip := excluded[db.Name]; skip {
			continue
		}
		selected = append(selected, db)
	}
	return selected, nil
}

func (s *Source) databaseAsset(db database, serverVersion string) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"host":     s.config.Host,
		"port":     s.config.Port,
		"database": db.Name,
	}
	if db.Owner != "" {
		metadata["owner"] = db.Owner
	}
	if serverVersion != "" {
		metadata["server_version"] = serverVersion
	}

	name := db.Name
	mrnValue := assetMRN(s.config, "Database", name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Database",
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// relationDetails is everything discovery learned about a relation beyond
// the pg_class row: gathered first, then folded into one asset.
type relationDetails struct {
	// columns is nil when column discovery is off, and empty when the
	// relation has no visible columns.
	columns    []column
	definition string
	partition  *partition
	rowCount   *int64
}

func (s *Source) relationAsset(dbName string, r relation, details relationDetails) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"database":    dbName,
		"schema":      r.Schema,
		"table_name":  r.Name,
		"object_type": r.objectType(),
	}
	if r.Owner != "" {
		metadata["owner"] = r.Owner
	}
	if r.Comment != "" {
		metadata["comment"] = r.Comment
	}
	if r.Kind == relkindMaterializedView {
		metadata["materialized"] = true
	}
	if r.Kind == relkindPartitionedTable || details.partition != nil {
		metadata["partitioned"] = true
	}
	if details.partition != nil && len(details.partition.Columns) > 0 {
		metadata["partition_columns"] = strings.Join(details.partition.Columns, ", ")
	}
	if details.rowCount != nil {
		metadata["estimated_row_count"] = *details.rowCount
	}

	name := r.Name
	assetType := r.assetType()
	mrnValue := assetMRN(s.config, assetType, dbName, r.Schema, r.Name)
	previous := mrn.New(assetType, provider, qualifiedName(dbName, r.Schema, r.Name))

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	// Earlier releases named the asset database.schema.table and had no zone, so the old MRN only differs once a zone is set.
	if previous != mrnValue {
		asset.PreviousMRNs = []string{previous}
	}

	if r.Comment != "" {
		comment := r.Comment
		asset.Description = &comment
	}

	if details.columns != nil {
		if err := pluginsdk.SetColumns(&asset, details.columns); err != nil {
			log.Warn().Err(err).Str("table", qualifiedName(dbName, r.Schema, r.Name)).Msg("Failed to set columns")
		}
	}

	if details.definition != "" {
		definition := details.definition
		language := "SQL"
		asset.Query = &definition
		asset.QueryLanguage = &language
	}

	return asset
}

// qualifiedName is the database.schema.table form earlier releases used as an asset name, kept to build the previous MRN and for log messages.
func qualifiedName(database, schema, table string) string {
	return database + "." + schema + "." + table
}

// assetMRN is the single place a CockroachDB MRN is built, so the asset pass and every lineage pass address an object the same way. The path runs from the database down.
func assetMRN(cfg *Config, assetType string, path ...string) string {
	return cfg.AssetMRN(assetType, provider, path...)
}
