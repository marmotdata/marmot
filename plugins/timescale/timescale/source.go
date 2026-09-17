// Package timescale discovers databases, tables, views, hypertables and
// continuous aggregates from TimescaleDB instances.
package timescale

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the service every asset is filed under. TimescaleDB is a
// PostgreSQL extension, so a TimescaleDB server is a PostgreSQL server and its
// tables belong in the same catalog entries the PostgreSQL plugin creates. A
// separate provider string would split one database across two catalogs.
const provider = "PostgreSQL"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "timescale",
		Name:        "TimescaleDB",
		Description: "Discover databases, tables, hypertables and continuous aggregates from TimescaleDB instances",
		Icon:        "postgresql",
		Category:    "database",
		Status:      "experimental",
		// Discover emits CONTAINS, FOREIGN_KEY and VIEW_OF edges, so the
		// manifest declares Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(TimescaleFields{}, "TimescaleDB",
				"The metadata every discovered object carries."),
			pluginsdk.AssetSchemaOf(TimescaleHypertableFields{}, "Hypertable",
				"The metadata a table carries when TimescaleDB partitions it into chunks."),
			pluginsdk.AssetSchemaOf(TimescaleAggregateFields{}, "Aggregate",
				"The metadata a view carries when it is a continuous aggregate."),
			pluginsdk.AssetSchemaOf(TimescaleColumnFields{}, "Column",
				"The per-column metadata attached to an asset's schema."),
		},
	}
}

// Config for the TimescaleDB plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	// Connection configuration
	Host     string `json:"host" description:"TimescaleDB server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"TimescaleDB server port" default:"5432" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Username for authentication" validate:"required"`
	Password string `json:"password" description:"Password for authentication" sensitive:"true"`
	Database string `json:"database" description:"Database to discover. When empty, every database on the server is discovered"`
	SSLMode  string `json:"ssl_mode" label:"SSL Mode" description:"SSL mode (disable, require, verify-ca, verify-full)" default:"disable" validate:"omitempty,oneof=disable require verify-ca verify-full"`

	// Discovery configuration
	IncludeColumns       bool     `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeStatistics    bool     `json:"include_statistics" description:"Whether to collect row, column and size statistics" default:"true"`
	DiscoverForeignKeys  bool     `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	ExcludeSystemSchemas bool     `json:"exclude_system_schemas" description:"Whether to exclude PostgreSQL and TimescaleDB internal schemas" default:"true"`
	ExcludeDatabases     []string `json:"exclude_databases" description:"Databases to skip"`

	// TimescaleDB configuration
	IncludeHypertables          bool `json:"include_hypertables" description:"Whether to record hypertable dimensions, chunk counts and policies" default:"true"`
	IncludeContinuousAggregates bool `json:"include_continuous_aggregates" description:"Whether to record continuous aggregates and their source hypertable lineage" default:"true"`
	IncludeCompression          bool `json:"include_compression" description:"Whether to record compression segment and order columns" default:"true"`
	IncludeChunks               bool `json:"include_chunks" description:"Whether to read the oldest and newest chunk range. Off by default because a busy hypertable has thousands of chunks" default:"false"`
}

// Example configuration for the plugin
var _ = `
host: "timescale.company.com"
port: 5432
user: "marmot_reader"
password: "secure_password_123"
database: "metrics"
ssl_mode: "require"
include_chunks: true
tags:
  - "timescale"
  - "production"
`

// Source represents the TimescaleDB plugin.
type Source struct {
	config *Config
	pool   *pgxpool.Pool
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

// Discover discovers databases, tables, views and TimescaleDB objects.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// The database list lives in a shared catalog, so any database can be
	// asked for it. Prefer the configured one so a server that only grants
	// access to that database still works.
	bootstrap := s.config.Database
	if bootstrap == "" {
		bootstrap = "postgres"
	}
	if err := s.connect(ctx, bootstrap); err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", bootstrap, err)
	}
	defer s.disconnect()

	databaseAssets, err := s.discoverDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering databases: %w", err)
	}

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	assets = append(assets, databaseAssets...)

	for _, dbAsset := range databaseAssets {
		dbName := *dbAsset.Name

		dbCtx, dbCancel := context.WithTimeout(ctx, 5*time.Minute)
		result, err := s.discoverDatabase(dbCtx, dbName, *dbAsset.MRN)
		dbCancel()
		if err != nil {
			// One unreachable database must not lose the rest of the server.
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover database")
			continue
		}

		assets = append(assets, result.Assets...)
		lineages = append(lineages, result.Lineage...)
		statistics = append(statistics, result.Statistics...)
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("TimescaleDB discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// discoverDatabase reads one database: its objects, their columns, the
// TimescaleDB catalog on top of them, and the edges between them.
func (s *Source) discoverDatabase(ctx context.Context, dbName, dbMRN string) (*pluginsdk.DiscoveryResult, error) {
	if err := s.connect(ctx, dbName); err != nil {
		return nil, fmt.Errorf("connecting: %w", err)
	}

	log.Debug().Str("database", dbName).Msg("Starting object discovery")

	objects, err := s.discoverObjects(ctx, dbName)
	if err != nil {
		return nil, fmt.Errorf("discovering tables and views: %w", err)
	}

	// TimescaleDB is an extension, so an instance can hold databases that do
	// not have it installed. Those are discovered as plain PostgreSQL.
	ts, err := s.readTimescale(ctx)
	if err != nil {
		log.Warn().Err(err).Str("database", dbName).Msg("Failed to read the TimescaleDB catalog, continuing with PostgreSQL metadata only")
		ts = nil
	}

	assets := s.buildAssets(objects, ts)

	var lineages []pluginsdk.LineageEdge
	for i := range assets {
		lineages = append(lineages, pluginsdk.LineageEdge{
			Source: dbMRN,
			Target: *assets[i].MRN,
			Type:   "CONTAINS",
		})
	}

	lineages = append(lineages, viewLineage(objects, ts)...)

	if s.config.DiscoverForeignKeys {
		fkLineages, err := s.discoverForeignKeys(ctx)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover foreign key relationships")
		} else {
			lineages = append(lineages, fkLineages...)
			log.Debug().Int("count", len(fkLineages)).Msg("Discovered foreign key relationships")
		}
	}

	var statistics []pluginsdk.Statistic
	if s.config.IncludeStatistics {
		statistics = s.collectStatistics(ctx, objects, ts)
	}

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// buildAssets turns the discovered objects into assets, layering the
// TimescaleDB catalog on top of the PostgreSQL metadata where it applies.
func (s *Source) buildAssets(objects []object, ts *timescaleInfo) []pluginsdk.Asset {
	assets := make([]pluginsdk.Asset, 0, len(objects))

	for _, obj := range objects {
		metadata := obj.metadata()

		assetType := "Table"
		if obj.Kind != kindTable {
			assetType = "View"
		}

		query := obj.Definition
		if ts != nil {
			if cagg, ok := ts.aggregates[obj.key()]; ok {
				// The rewritten definition PostgreSQL reports for a
				// continuous aggregate points at the internal
				// materialization hypertable, so the catalog's copy of the
				// original query is the one worth showing.
				query = cagg.Definition
				applyAggregateMetadata(metadata, cagg, ts)
			}
			if ht, ok := ts.hypertables[obj.key()]; ok {
				applyHypertableMetadata(metadata, ht, ts, s.config.IncludeChunks)
			}
		}

		mrnValue := assetMRN(assetType, obj.Name)
		description := fmt.Sprintf("%s %s.%s in database %s", describeKind(metadata), obj.Schema, obj.Name, obj.Database)

		asset := pluginsdk.Asset{
			Name:        &obj.Name,
			MRN:         &mrnValue,
			Type:        assetType,
			Providers:   []string{provider},
			Description: &description,
			Metadata:    metadata,
			Schema:      make(map[string]string),
			Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		}

		if query != "" {
			language := "sql"
			asset.Query = &query
			asset.QueryLanguage = &language
		}

		if len(obj.Columns) > 0 {
			if err := pluginsdk.SetColumns(&asset, obj.Columns); err != nil {
				log.Warn().Err(err).Str("table", obj.key()).Msg("Failed to set columns")
			}
		}

		assets = append(assets, asset)
	}

	return assets
}

// describeKind names the object in prose for the asset description.
func describeKind(metadata map[string]interface{}) string {
	if metadata["continuous_aggregate"] == true {
		return "TimescaleDB continuous aggregate"
	}
	if metadata["hypertable"] == true {
		return "TimescaleDB hypertable"
	}
	switch metadata["object_type"] {
	case "view":
		return "PostgreSQL view"
	case "materialized_view":
		return "PostgreSQL materialized view"
	default:
		return "PostgreSQL table"
	}
}

// assetMRN is the single place a TimescaleDB MRN is built. Object discovery,
// foreign keys and view lineage all go through it so the three can never
// drift into addressing the same table differently. The name is the bare
// object name under the PostgreSQL provider, byte for byte what the
// PostgreSQL plugin produces, so a run of either plugin against the same
// server updates the same assets instead of duplicating them.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
