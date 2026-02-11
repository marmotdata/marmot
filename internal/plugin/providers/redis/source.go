// +marmot:name=Redis
// +marmot:description=Discovers databases from Redis instances.
// +marmot:status=experimental
// +marmot:features=Assets
package redis

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Config for Redis plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection options
	Host     string `json:"host" description:"Redis server hostname or IP address" validate:"required"`
	Port     int    `json:"port,omitempty" description:"Redis server port" default:"6379" validate:"omitempty,min=1,max=65535"`
	Password string `json:"password,omitempty" description:"Password for authentication" sensitive:"true"`
	Username string `json:"username,omitempty" description:"Username for ACL authentication"`
	DB          int    `json:"db,omitempty" description:"Default database number" default:"0" validate:"omitempty,min=0,max=15"`
	TLS         bool   `json:"tls,omitempty" description:"Enable TLS connection"`
	TLSInsecure bool   `json:"tls_insecure,omitempty" label:"TLS Insecure" description:"Skip TLS certificate verification"`

	// Discovery options
	DiscoverAllDatabases bool `json:"discover_all_databases" description:"Discover all databases with keys (db0-db15)" default:"true"`

}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "localhost"
port: 6379
password: "secret"
discover_all_databases: true
filter:
  include:
    - "^db[0-3]$"
tags:
  - "redis"
  - "cache"
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
		config.Port = 6379
	}

	// Default discover_all_databases to true unless explicitly set to false
	if _, ok := rawConfig["discover_all_databases"]; !ok {
		config.DiscoverAllDatabases = true
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	if s.config.Port == 0 {
		s.config.Port = 6379
	}

	client, err := s.createClient()
	if err != nil {
		return nil, fmt.Errorf("creating Redis client: %w", err)
	}
	defer client.Close()

	// Ping to verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to Redis: %w", err)
	}

	// Get server info
	infoResult, err := client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("getting Redis INFO: %w", err)
	}

	serverInfo := parseInfoSection(infoResult, "Server")
	memoryInfo := parseInfoSection(infoResult, "Memory")
	clientsInfo := parseInfoSection(infoResult, "Clients")
	replicationInfo := parseInfoSection(infoResult, "Replication")
	keyspaceInfo := parseInfoSection(infoResult, "Keyspace")

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	host := s.config.Host
	port := s.config.Port

	if s.config.DiscoverAllDatabases {
		// Discover databases from keyspace info
		for dbName, dbStats := range keyspaceInfo {
			if !strings.HasPrefix(dbName, "db") {
				continue
			}

			keyspace := parseKeyspaceEntry(dbStats)
			a := s.createDatabaseAsset(host, port, dbName, keyspace, serverInfo, memoryInfo, clientsInfo, replicationInfo)
			assets = append(assets, a)
		}
	} else {
		// Only discover the configured database
		dbName := fmt.Sprintf("db%d", s.config.DB)
		dbStats, exists := keyspaceInfo[dbName]
		keyspace := make(map[string]string)
		if exists {
			keyspace = parseKeyspaceEntry(dbStats)
		}
		a := s.createDatabaseAsset(host, port, dbName, keyspace, serverInfo, memoryInfo, clientsInfo, replicationInfo)
		assets = append(assets, a)
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) createClient() (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Password: s.config.Password,
		Username: s.config.Username,
		DB:       s.config.DB,
	}

	if s.config.TLS {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: s.config.TLSInsecure,
		}
	}

	return redis.NewClient(opts), nil
}

func parseInfoSection(info string, section string) map[string]string {
	result := make(map[string]string)
	inSection := false

	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			if inSection {
				break
			}
			continue
		}

		if strings.HasPrefix(line, "# "+section) {
			inSection = true
			continue
		}

		if inSection {
			if strings.HasPrefix(line, "#") {
				break
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

func parseKeyspaceEntry(entry string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(entry, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func (s *Source) createDatabaseAsset(host string, port int, dbName string, keyspace map[string]string, serverInfo, memoryInfo, clientsInfo, replicationInfo map[string]string) asset.Asset {
	metadata := make(map[string]interface{})
	metadata["host"] = host
	metadata["port"] = port
	metadata["database"] = dbName

	if v, ok := serverInfo["redis_version"]; ok {
		metadata["redis_version"] = v
	}
	if v, ok := serverInfo["uptime_in_seconds"]; ok {
		metadata["uptime_seconds"] = v
	}
	if v, ok := replicationInfo["role"]; ok {
		metadata["role"] = v
	}
	if v, ok := clientsInfo["connected_clients"]; ok {
		metadata["connected_clients"] = v
	}
	if v, ok := memoryInfo["used_memory_human"]; ok {
		metadata["used_memory_human"] = v
	}
	if v, ok := memoryInfo["maxmemory_policy"]; ok {
		metadata["maxmemory_policy"] = v
	}

	if v, ok := keyspace["keys"]; ok {
		if count, err := strconv.ParseInt(v, 10, 64); err == nil {
			metadata["key_count"] = count
		}
	}
	if v, ok := keyspace["expires"]; ok {
		if count, err := strconv.ParseInt(v, 10, 64); err == nil {
			metadata["expires_count"] = count
		}
	}
	if v, ok := keyspace["avg_ttl"]; ok {
		if ttl, err := strconv.ParseInt(v, 10, 64); err == nil {
			metadata["avg_ttl_ms"] = ttl
		}
	}

	resourceName := fmt.Sprintf("%s:%d-%s", host, port, dbName)
	mrnValue := mrn.New("Database", "Redis", resourceName)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:      &dbName,
		MRN:       &mrnValue,
		Type:      "Database",
		Providers: []string{"Redis"},
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Redis",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "redis",
		Name:        "Redis",
		Description: "Discover databases from Redis instances",
		Icon:        "redis",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Redis plugin")
	}
}
