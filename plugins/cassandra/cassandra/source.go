// Package cassandra discovers keyspaces, tables and materialized views
// from Apache Cassandra clusters.
package cassandra

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string, shared with the OpenMetadata
// projection so both routes land a Cassandra table on the same asset.
const provider = "Cassandra"

// Config for the Cassandra plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Hosts      []string `json:"hosts" description:"Contact points, as host or host:port" validate:"required,min=1,dive,required"`
	Port       int      `json:"port" description:"Port for contact points given without one" default:"9042" validate:"omitempty,min=1,max=65535"`
	Username   string   `json:"username" description:"Username for password authentication"`
	Password   string   `json:"password" description:"Password for password authentication" sensitive:"true"`
	Datacenter string   `json:"datacenter" description:"Local datacenter; only its nodes are queried when set"`

	SSL           bool   `json:"ssl" label:"SSL" description:"Connect over TLS" default:"false"`
	SSLCACert     string `json:"ssl_ca_cert" label:"SSL CA Certificate" description:"Path to the CA certificate that signed the server certificate"`
	SSLSkipVerify bool   `json:"ssl_skip_verify" label:"SSL Skip Verify" description:"Skip verification of the server certificate" default:"false"`

	ConnectTimeoutSeconds int `json:"connect_timeout_seconds" description:"Seconds to wait for a connection" default:"10" validate:"omitempty,min=1,max=300"`

	Keyspaces              []string `json:"keyspaces" description:"Keyspaces to discover; all non-system keyspaces when empty"`
	ExcludeSystemKeyspaces bool     `json:"exclude_system_keyspaces" description:"Whether to skip Cassandra's own keyspaces (system, system_schema, system_auth, ...)" default:"true"`
	IncludeColumns         bool     `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeViews           bool     `json:"include_views" description:"Whether to include materialized views" default:"true"`
	IncludeIndexes         bool     `json:"include_indexes" description:"Whether to include secondary index information" default:"true"`
	IncludeStatistics      bool     `json:"include_statistics" description:"Whether to include column and index counts" default:"true"`
}

// Example configuration for the plugin
var _ = `
hosts:
  - "cassandra-1.internal:9042"
  - "cassandra-2.internal"
port: 9042
username: "marmot_reader"
password: "cassandra_secure_pass"
datacenter: "dc1"
keyspaces:
  - "shop"
tags:
  - "cassandra"
  - "shop"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "cassandra",
		Name:        "Apache Cassandra",
		Description: "Discover keyspaces, tables and materialized views from Apache Cassandra clusters",
		Icon:        "cassandra",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(CassandraKeyspaceFields{}, "Keyspace",
				"The metadata fields the Cassandra plugin emits for keyspace assets."),
			pluginsdk.AssetSchemaOf(CassandraTableFields{}, "Table",
				"The metadata fields emitted for table and materialized view assets."),
			pluginsdk.AssetSchemaOf(CassandraColumnFields{}, "Column",
				"The per-column fields embedded in an asset's schema."),
		},
	}
}

// Source represents the Cassandra plugin.
type Source struct {
	config  *Config
	session *gocql.Session
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

	hosts, err := parseHosts(config.Hosts, config.Port)
	if err != nil {
		return nil, fmt.Errorf("parsing hosts: %w", err)
	}
	config.Hosts = hosts

	if config.Password != "" && config.Username == "" {
		return nil, fmt.Errorf("password requires username")
	}
	if config.SSLCACert != "" && !config.SSL {
		return nil, fmt.Errorf("ssl_ca_cert requires ssl to be true")
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Cassandra keyspaces, tables and materialized views.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	session, err := connect(s.config)
	if err != nil {
		return nil, fmt.Errorf("connecting to Cassandra: %w", err)
	}
	defer session.Close()
	s.session = session

	log.Debug().Strs("hosts", s.config.Hosts).Msg("Starting Cassandra discovery")

	cluster, err := s.clusterInfo(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read cluster information")
	}

	allKeyspaces, err := s.listKeyspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing keyspaces: %w", err)
	}
	keyspaces := s.selectKeyspaces(allKeyspaces)

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	for _, ks := range keyspaces {
		result, err := s.discoverKeyspace(ctx, ks, cluster)
		if err != nil {
			log.Warn().Err(err).Str("keyspace", ks.Name).Msg("Failed to discover keyspace")
			continue
		}
		assets = append(assets, result.Assets...)
		lineages = append(lineages, result.Lineage...)
		statistics = append(statistics, result.Statistics...)
	}

	log.Info().
		Int("keyspaces", len(keyspaces)).
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("Cassandra discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// systemKeyspaces are the keyspaces Cassandra manages itself. The virtual
// ones (system_views, system_virtual_schema) never appear in
// system_schema.keyspaces, but are listed so an explicit exclusion covers
// every version.
var systemKeyspaces = map[string]struct{}{
	"system":                {},
	"system_schema":         {},
	"system_auth":           {},
	"system_distributed":    {},
	"system_traces":         {},
	"system_views":          {},
	"system_virtual_schema": {},
}

// selectKeyspaces narrows the cluster's keyspaces to the ones this run
// should discover. An explicit keyspaces list wins over the system
// exclusion, so a user who names "system" gets it.
func (s *Source) selectKeyspaces(all []keyspaceInfo) []keyspaceInfo {
	byName := make(map[string]keyspaceInfo, len(all))
	for _, ks := range all {
		byName[ks.Name] = ks
	}

	var selected []keyspaceInfo

	if len(s.config.Keyspaces) > 0 {
		for _, name := range s.config.Keyspaces {
			ks, ok := byName[name]
			if !ok {
				log.Warn().Str("keyspace", name).Msg("Configured keyspace not found")
				continue
			}
			selected = append(selected, ks)
		}
		return selected
	}

	for _, ks := range all {
		if _, system := systemKeyspaces[ks.Name]; system && s.config.ExcludeSystemKeyspaces {
			log.Debug().Str("keyspace", ks.Name).Msg("Skipping system keyspace")
			continue
		}
		selected = append(selected, ks)
	}
	return selected
}

// discoverKeyspace reads one keyspace's schema and turns it into assets.
// Only the table list is fatal for the keyspace; columns, views, indexes
// and types degrade to a warning so a permissions gap on one system table
// does not hide the whole keyspace.
func (s *Source) discoverKeyspace(ctx context.Context, ks keyspaceInfo, cluster clusterInfo) (*pluginsdk.DiscoveryResult, error) {
	tables, err := s.listTables(ctx, ks.Name)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}

	columns, err := s.listColumns(ctx, ks.Name)
	if err != nil {
		log.Warn().Err(err).Str("keyspace", ks.Name).Msg("Failed to list columns")
		columns = nil
	}

	var views []viewInfo
	if s.config.IncludeViews {
		views, err = s.listViews(ctx, ks.Name)
		if err != nil {
			log.Warn().Err(err).Str("keyspace", ks.Name).Msg("Failed to list materialized views")
		}
	}

	indexes := map[string][]indexInfo{}
	if s.config.IncludeIndexes {
		indexes, err = s.listIndexes(ctx, ks.Name)
		if err != nil {
			log.Warn().Err(err).Str("keyspace", ks.Name).Msg("Failed to list indexes")
		}
	}

	userTypes, err := s.listUserTypes(ctx, ks.Name)
	if err != nil {
		log.Warn().Err(err).Str("keyspace", ks.Name).Msg("Failed to list user-defined types")
	}

	result := &pluginsdk.DiscoveryResult{}

	keyspaceAsset := s.keyspaceAsset(ks, cluster, len(tables), len(views), userTypes)
	result.Assets = append(result.Assets, keyspaceAsset)

	for _, t := range tables {
		cols, hasColumns := columns[t.Name]
		asset := s.tableAsset(ks.Name, t, cols, indexes[t.Name])
		result.Assets = append(result.Assets, asset)
		result.Lineage = append(result.Lineage, pluginsdk.LineageEdge{
			Source: *keyspaceAsset.MRN,
			Target: *asset.MRN,
			Type:   "CONTAINS",
		})
		if s.config.IncludeStatistics {
			result.Statistics = append(result.Statistics, objectStatistics(*asset.MRN, cols, hasColumns, len(indexes[t.Name]))...)
		}
	}

	tableMRNs := make(map[string]string, len(tables))
	for _, t := range tables {
		tableMRNs[t.Name] = assetMRN("Table", objectName(ks.Name, t.Name))
	}

	for _, v := range views {
		cols, hasColumns := columns[v.Name]
		asset := s.viewAsset(ks.Name, v, cols)
		result.Assets = append(result.Assets, asset)
		result.Lineage = append(result.Lineage, pluginsdk.LineageEdge{
			Source: *keyspaceAsset.MRN,
			Target: *asset.MRN,
			Type:   "CONTAINS",
		})
		// A materialized view is always built from a table in its own
		// keyspace, so the base table was discovered in this same pass.
		if baseMRN, ok := tableMRNs[v.BaseTable]; ok {
			result.Lineage = append(result.Lineage, pluginsdk.LineageEdge{
				Source: baseMRN,
				Target: *asset.MRN,
				Type:   "VIEW_OF",
			})
		}
		if s.config.IncludeStatistics {
			result.Statistics = append(result.Statistics, objectStatistics(*asset.MRN, cols, hasColumns, 0)...)
		}
	}

	log.Debug().
		Str("keyspace", ks.Name).
		Int("tables", len(tables)).
		Int("views", len(views)).
		Msg("Discovered keyspace")

	return result, nil
}

func (s *Source) keyspaceAsset(ks keyspaceInfo, cluster clusterInfo, tableCount, viewCount int, userTypes []userType) pluginsdk.Asset {
	name := ks.Name
	metadata := map[string]interface{}{
		"keyspace":       ks.Name,
		"durable_writes": ks.DurableWrites,
		"table_count":    tableCount,
		"view_count":     viewCount,
	}

	rep := parseReplication(ks.Replication)
	if rep.Class != "" {
		metadata["replication_class"] = rep.Class
	}
	if rep.Factor > 0 {
		metadata["replication_factor"] = rep.Factor
	}
	if len(rep.PerDatacenter) > 0 {
		metadata["replication_factors"] = rep.PerDatacenter
	}

	if len(userTypes) > 0 {
		names := make([]string, 0, len(userTypes))
		for _, t := range userTypes {
			names = append(names, t.Name)
		}
		sort.Strings(names)
		metadata["user_types"] = names
	}

	if cluster.Name != "" {
		metadata["cluster_name"] = cluster.Name
	}
	if cluster.Version != "" {
		metadata["cassandra_version"] = cluster.Version
	}
	if cluster.Datacenter != "" {
		metadata["datacenter"] = cluster.Datacenter
	}

	return s.newAsset("Keyspace", name, metadata)
}

func (s *Source) tableAsset(keyspace string, t tableInfo, cols []columnInfo, indexes []indexInfo) pluginsdk.Asset {
	name := objectName(keyspace, t.Name)
	metadata := map[string]interface{}{
		"keyspace":               keyspace,
		"table_name":             t.Name,
		"object_type":            "table",
		"id":                     t.ID.String(),
		"default_ttl":            t.DefaultTTL,
		"gc_grace_seconds":       t.GCGraceSeconds,
		"bloom_filter_fp_chance": t.BloomFilterFPChance,
		"is_counter":             hasFlag(t.Flags, "counter"),
	}
	if t.Comment != "" {
		metadata["comment"] = t.Comment
	}
	if class := shortClassName(t.Compaction["class"]); class != "" {
		metadata["compaction_class"] = class
	}
	if class := shortClassName(t.Compression["class"]); class != "" {
		metadata["compression_class"] = class
	}
	if len(t.Caching) > 0 {
		metadata["caching"] = t.Caching
	}
	if len(t.Flags) > 0 {
		flags := append([]string(nil), t.Flags...)
		sort.Strings(flags)
		metadata["flags"] = flags
	}

	addKeyMetadata(metadata, cols)

	if len(indexes) > 0 {
		names := make([]string, 0, len(indexes))
		for _, idx := range indexes {
			names = append(names, idx.Name)
		}
		sort.Strings(names)
		metadata["indexes"] = names
		metadata["index_count"] = len(indexes)
	}

	asset := s.newAsset("Table", name, metadata)
	if t.Comment != "" {
		comment := t.Comment
		asset.Description = &comment
	}
	s.attachColumns(&asset, cols)
	return asset
}

func (s *Source) viewAsset(keyspace string, v viewInfo, cols []columnInfo) pluginsdk.Asset {
	name := objectName(keyspace, v.Name)
	metadata := map[string]interface{}{
		"keyspace":            keyspace,
		"table_name":          v.Name,
		"object_type":         "view",
		"id":                  v.ID.String(),
		"base_table":          v.BaseTable,
		"include_all_columns": v.IncludeAllColumns,
	}
	if v.WhereClause != "" {
		metadata["where_clause"] = v.WhereClause
	}
	if v.Comment != "" {
		metadata["comment"] = v.Comment
	}

	addKeyMetadata(metadata, cols)

	asset := s.newAsset("View", name, metadata)
	if v.Comment != "" {
		comment := v.Comment
		asset.Description = &comment
	}

	query := viewQuery(keyspace, v, cols)
	language := "CQL"
	asset.Query = &query
	asset.QueryLanguage = &language

	s.attachColumns(&asset, cols)
	return asset
}

// newAsset builds the parts every Cassandra asset shares.
func (s *Source) newAsset(assetType, name string, metadata map[string]interface{}) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)

	return pluginsdk.Asset{
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
}

func (s *Source) attachColumns(asset *pluginsdk.Asset, cols []columnInfo) {
	if !s.config.IncludeColumns || len(cols) == 0 {
		return
	}
	if err := pluginsdk.SetColumns(asset, schemaColumns(cols)); err != nil {
		log.Warn().Err(err).Str("asset", *asset.Name).Msg("Failed to set columns")
	}
}

// addKeyMetadata records the primary key layout: partition key columns and
// clustering columns in key order, plus each clustering column's sort
// direction.
func addKeyMetadata(metadata map[string]interface{}, cols []columnInfo) {
	partition, clustering, order := keyColumns(cols)
	if len(partition) > 0 {
		metadata["partition_key"] = partition
	}
	if len(clustering) > 0 {
		metadata["clustering_columns"] = clustering
		metadata["clustering_order"] = order
	}
}

// objectStatistics emits the column count for a table or view and, for
// tables with secondary indexes, the index count. A row count is
// deliberately absent: Cassandra keeps no row estimate and COUNT(*) is a
// full cluster scan.
func objectStatistics(assetMRN string, cols []columnInfo, hasColumns bool, indexCount int) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	if hasColumns {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   assetMRN,
			MetricName: "asset.column_count",
			Value:      float64(len(cols)),
		})
	}
	if indexCount > 0 {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   assetMRN,
			MetricName: "asset.index_count",
			Value:      float64(indexCount),
		})
	}

	return statistics
}

// objectName qualifies a table or view by its keyspace. Cassandra has no
// database layer above the keyspace and a table name is only unique within
// one, so keyspace.table is the identity, matching the OpenMetadata
// projection for Cassandra services.
func objectName(keyspace, name string) string {
	return keyspace + "." + name
}

// assetMRN is the single place a Cassandra MRN is built. Assets, lineage
// and statistics all go through it so they can never drift into addressing
// the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

// shortClassName strips the Java package from a strategy or compressor
// class, so metadata reads SizeTieredCompactionStrategy rather than the
// full org.apache.cassandra path.
func shortClassName(class string) string {
	if i := strings.LastIndex(class, "."); i >= 0 {
		return class[i+1:]
	}
	return class
}
