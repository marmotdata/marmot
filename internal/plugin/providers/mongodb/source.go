// +marmot:name=MongoDB
// +marmot:description=This plugin discovers databases and collections from MongoDB instances.
// +marmot:status=experimental
package mongodb

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

// Config for MongoDB plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	ConnectionURI string `json:"connection_uri" description:"MongoDB connection URI (overrides host/port/user/password)"`
	Host          string `json:"host" description:"MongoDB server hostname or IP address"`
	Port          int    `json:"port" description:"MongoDB server port (default: 27017)"`
	User          string `json:"user" description:"Username for authentication"`
	Password      string `json:"password" description:"Password for authentication" sensitive:"true"`
	AuthSource    string `json:"auth_source" description:"Authentication database name"`
	TLS           bool   `json:"tls" description:"Enable TLS/SSL for connection"`
	TLSInsecure   bool   `json:"tls_insecure" description:"Skip verification of server certificate"`

	IncludeDatabases   bool           `json:"include_databases" description:"Whether to discover databases" default:"true"`
	IncludeCollections bool           `json:"include_collections" description:"Whether to discover collections" default:"true"`
	IncludeViews       bool           `json:"include_views" description:"Whether to include views" default:"true"`
	IncludeIndexes     bool           `json:"include_indexes" description:"Whether to include index information" default:"true"`
	SampleSchema       bool           `json:"sample_schema" description:"Sample documents to infer schema" default:"true"`
	SampleSize         int            `json:"sample_size" description:"Number of documents to sample (default: 1000, -1 for entire collection)" default:"1000"`
	UseRandomSampling  bool           `json:"use_random_sampling" description:"Use random sampling for schema inference" default:"true"`
	DatabaseFilter     *plugin.Filter `json:"database_filter,omitempty" description:"Filter configuration for databases"`
	CollectionFilter   *plugin.Filter `json:"collection_filter,omitempty" description:"Filter configuration for collections"`
	ExcludeSystemDbs   bool           `json:"exclude_system_dbs" description:"Whether to exclude system databases (admin, config, local)" default:"true"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "mongo-cluster.company.com"
port: 27017
user: "analytics_reader"
password: "mongo_pass_456"
auth_source: "admin"
tls: true
tags:
  - "mongodb"
  - "analytics"
`

type Source struct {
	config     *Config
	client     *mongo.Client
	timeout    time.Duration
	sampleSize int32
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	log.Debug().Interface("raw_config", rawConfig).Msg("Starting MongoDB config validation")

	if config.ConnectionURI == "" {
		if config.Host == "" {
			return nil, fmt.Errorf("either host or connection_uri is required")
		}
		if config.Port == 0 {
			config.Port = 27017
		}
	}
	if !config.IncludeDatabases {
		config.IncludeDatabases = true
	}
	if !config.IncludeCollections {
		config.IncludeCollections = true
	}
	if config.SampleSize == -1 {
		s.sampleSize = 0
	} else {
		s.sampleSize = int32(config.SampleSize)
	}
	s.config = config
	s.timeout = 2 * time.Minute
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to MongoDB: %w", err)
	}
	defer s.disconnect(ctx)

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	if s.config.IncludeDatabases {
		databaseAssets, err := s.discoverDatabases(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover databases")
		} else {
			assets = append(assets, databaseAssets...)
			log.Debug().Int("count", len(databaseAssets)).Msg("Discovered databases")
		}

		for _, dbAsset := range databaseAssets {
			if dbAsset.Type != "Database" {
				continue
			}

			dbName := *dbAsset.Name
			if s.config.DatabaseFilter != nil && !plugin.ShouldIncludeResource(dbName, *s.config.DatabaseFilter) {
				log.Debug().Str("database", dbName).Msg("Skipping database due to filter")
				continue
			}

			if s.config.ExcludeSystemDbs && (dbName == "admin" || dbName == "config" || dbName == "local") {
				log.Debug().Str("database", dbName).Msg("Skipping system database")
				continue
			}

			if s.config.IncludeCollections {
				collectionAssets, err := s.discoverCollections(ctx, dbName)
				if err != nil {
					log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover collections")
					continue
				}

				assets = append(assets, collectionAssets...)
				log.Debug().Str("database", dbName).Int("count", len(collectionAssets)).Msg("Discovered collections")

				for _, collAsset := range collectionAssets {
					lineages = append(lineages, lineage.LineageEdge{
						Source: *dbAsset.MRN,
						Target: *collAsset.MRN,
						Type:   "CONTAINS",
					})

					if collAsset.Type == "View" {
						viewOn, ok := collAsset.Metadata["view_on"].(string)
						if ok && viewOn != "" {
							sourceCollMRN := mrn.New("Collection", "MongoDB", viewOn)
							lineages = append(lineages, lineage.LineageEdge{
								Source: sourceCollMRN,
								Target: *collAsset.MRN,
								Type:   "VIEW_OF",
							})
						}
					}
				}
			}
		}
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}
