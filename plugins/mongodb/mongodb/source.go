// Package mongodb discovers databases and collections from MongoDB
// instances.
package mongodb

import (
	"context"
	"fmt"
	"math"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "mongodb",
		Name:        "MongoDB",
		Description: "Discover databases and collections from MongoDB instances",
		Icon:        "mongodb",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for MongoDB plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ConnectionURI string `json:"connection_uri" label:"Connection URI" description:"MongoDB connection URI (overrides host/port/user/password)" validate:"omitempty,uri"`
	Host          string `json:"host" description:"MongoDB server hostname or IP address" validate:"required_without=ConnectionURI"`
	Port          int    `json:"port" description:"MongoDB server port" default:"27017" validate:"omitempty,min=1,max=65535"`
	User          string `json:"user" description:"Username for authentication"`
	Password      string `json:"password" description:"Password for authentication" sensitive:"true"`
	AuthSource    string `json:"auth_source" description:"Authentication database name" default:"admin"`
	TLS           bool   `json:"tls" description:"Enable TLS/SSL for connection" default:"false"`
	TLSInsecure   bool   `json:"tls_insecure" label:"TLS Insecure" description:"Skip verification of server certificate" default:"false"`

	IncludeDatabases   bool `json:"include_databases" description:"Whether to discover databases" default:"true"`
	IncludeCollections bool `json:"include_collections" description:"Whether to discover collections" default:"true"`
	IncludeViews       bool `json:"include_views" description:"Whether to include views" default:"true"`
	IncludeIndexes     bool `json:"include_indexes" description:"Whether to include index information" default:"true"`
	SampleSchema       bool `json:"sample_schema" description:"Sample documents to infer schema" default:"true"`
	SampleSize         int  `json:"sample_size" description:"Number of documents to sample (-1 for entire collection)" default:"1000" validate:"omitempty,min=-1"`
	UseRandomSampling  bool `json:"use_random_sampling" description:"Use random sampling for schema inference" default:"true"`
	ExcludeSystemDbs   bool `json:"exclude_system_dbs" description:"Whether to exclude system databases (admin, config, local)" default:"true"`
}

// Example configuration for the plugin
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

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Port == 0 {
		config.Port = 27017
	}
	if config.AuthSource == "" {
		config.AuthSource = "admin"
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	if config.ConnectionURI == "" && config.Host == "" {
		return nil, fmt.Errorf("either host or connection_uri is required")
	}

	switch {
	case config.SampleSize == -1:
		s.sampleSize = 0
	case config.SampleSize > math.MaxInt32:
		s.sampleSize = math.MaxInt32
	default:
		s.sampleSize = int32(config.SampleSize) //nolint:gosec // G115: bounds checked above
	}

	s.config = config
	s.timeout = 2 * time.Minute
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover
	// cannot rely on state set by an earlier Validate call.
	if _, err := s.Validate(pluginConfig); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to MongoDB: %w", err)
	}
	defer s.disconnect(ctx)

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge

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
					lineages = append(lineages, pluginsdk.LineageEdge{
						Source: *dbAsset.MRN,
						Target: *collAsset.MRN,
						Type:   "CONTAINS",
					})

					if collAsset.Type == "View" {
						viewOn, ok := collAsset.Metadata["view_on"].(string)
						if ok && viewOn != "" {
							sourceCollMRN := mrn.New("Collection", "MongoDB", viewOn)
							lineages = append(lineages, pluginsdk.LineageEdge{
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

	return &pluginsdk.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}
