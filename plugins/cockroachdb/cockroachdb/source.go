// Package cockroachdb discovers databases, tables, views and foreign key
// relationships from CockroachDB clusters.
package cockroachdb

import (
	"context"
	"fmt"
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

	Database         string   `json:"database,omitempty" description:"Discover only this database. Leave empty to discover every database"`
	ExcludeDatabases []string `json:"exclude_databases,omitempty" description:"Databases to skip when discovering every database" default:"[\"system\"]"`

	SSLMode     string `json:"ssl_mode" label:"SSL Mode" description:"SSL mode (disable, require, verify-ca, verify-full)" default:"disable" validate:"omitempty,oneof=disable require verify-ca verify-full"`
	SSLRootCert string `json:"ssl_root_cert,omitempty" label:"SSL Root Certificate" description:"Path to the CA certificate that signed the node certificates"`
	SSLCert     string `json:"ssl_cert,omitempty" label:"SSL Client Certificate" description:"Path to the client certificate, for certificate authentication"`
	SSLKey      string `json:"ssl_key,omitempty" label:"SSL Client Key" description:"Path to the client certificate key"`

	IncludeColumns      bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeViews        bool `json:"include_views" description:"Whether to discover views and materialized views" default:"true"`
	DiscoverForeignKeys bool `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	IncludeStatistics   bool `json:"include_statistics" description:"Whether to include row counts, column counts and table sizes" default:"true"`
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
		ID:          "cockroachdb",
		Name:        "CockroachDB",
		Description: "Discover databases, tables, views and foreign key relationships from CockroachDB clusters",
		Icon:        "cockroachdb",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the CockroachDB plugin.
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

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
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
	mrnValue := assetMRN("Database", name)

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

	name := qualifiedName(dbName, r.Schema, r.Name)
	assetType := r.assetType()
	mrnValue := assetMRN(assetType, name)

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

	if r.Comment != "" {
		comment := r.Comment
		asset.Description = &comment
	}

	if details.columns != nil {
		if err := pluginsdk.SetColumns(&asset, details.columns); err != nil {
			log.Warn().Err(err).Str("table", name).Msg("Failed to set columns")
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

// qualifiedName is the Name of a table or view: database.schema.table. One
// cluster holds many databases and a connection can address any of them, so
// the database is part of the identity. This is the shape the OpenMetadata
// plugin projects Cockroach tables onto.
func qualifiedName(database, schema, table string) string {
	return database + "." + schema + "." + table
}

// assetMRN is the single place a CockroachDB MRN is built, so the asset pass
// and every lineage pass address an object the same way.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
