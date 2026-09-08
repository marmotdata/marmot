// Package couchbase discovers buckets, scopes and collections from
// Couchbase Server and Capella clusters.
package couchbase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string every Couchbase asset carries. It
// has to match the OpenMetadata projection for Couchbase so both routes
// land on one asset.
const provider = "Couchbase"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "couchbase",
		Name:        "Couchbase",
		Description: "Discover buckets, scopes and collections from Couchbase clusters",
		Icon:        "couchbase",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Couchbase plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ConnectionString string `json:"connection_string" label:"Connection String" description:"Cluster connection string: couchbase://host, or couchbases://host for TLS (Capella)" validate:"required"`
	Username         string `json:"username" description:"Username for authentication" validate:"required"`
	Password         string `json:"password" description:"Password for authentication" sensitive:"true" validate:"required"`

	Bucket         string   `json:"bucket" description:"Only discover this bucket (all buckets when empty)"`
	ExcludeBuckets []string `json:"exclude_buckets" description:"Bucket names to skip" default:"[]"`

	IncludeSystemScopes   bool `json:"include_system_scopes" description:"Whether to include the _system scope" default:"false"`
	IncludeColumns        bool `json:"include_columns" description:"Whether to infer a document schema by sampling each collection" default:"true"`
	SampleSize            int  `json:"sample_size" description:"Number of documents to sample per collection for schema inference" default:"100" validate:"min=1,max=10000"`
	IncludeIndexes        bool `json:"include_indexes" description:"Whether to include query index information" default:"true"`
	IncludeStatistics     bool `json:"include_statistics" description:"Whether to collect document counts and bucket sizes" default:"true"`
	ConnectTimeoutSeconds int  `json:"connect_timeout_seconds" description:"Seconds to wait for the cluster connection" default:"10" validate:"min=1,max=300"`
	SSLSkipVerify         bool `json:"ssl_skip_verify" label:"SSL Skip Verify" description:"Skip TLS certificate verification for couchbases:// connections" default:"false"`
}

// Example configuration for the plugin
var _ = `
connection_string: "couchbases://cb.abc123.cloud.couchbase.com"
username: "marmot_reader"
password: "couchbase_pass_789"
bucket: "travel-sample"
include_columns: true
sample_size: 100
include_indexes: true
include_statistics: true
tags:
  - "couchbase"
  - "travel"
`

// Source represents the Couchbase plugin.
type Source struct {
	config  *Config
	cluster *gocb.Cluster
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	config.ConnectionString = normaliseConnectionString(config.ConnectionString)
	if config.ConnectionString != "" && !hasSupportedScheme(config.ConnectionString) {
		return nil, fmt.Errorf("connection_string must start with couchbase:// or couchbases://")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// normaliseConnectionString trims whitespace and adds the plain scheme when
// the user gave a bare host, which is what the SDK itself would assume.
func normaliseConnectionString(cs string) string {
	cs = strings.TrimSpace(cs)
	if cs == "" || strings.Contains(cs, "://") {
		return cs
	}
	return "couchbase://" + cs
}

func hasSupportedScheme(cs string) bool {
	return strings.HasPrefix(cs, "couchbase://") || strings.HasPrefix(cs, "couchbases://")
}

// Discover discovers Couchbase buckets and collections.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to Couchbase: %w", err)
	}
	defer s.close()

	buckets, err := s.listBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}

	// The management REST API is the only place the cluster version and
	// bucket usage figures live. It is reached over plain HTTP on the
	// management port, which may be closed where the SDK ports are open
	// (Capella), so a failure only costs the figures, not the run.
	mgmt := newManagementClient(s.config)
	clusterVersion, err := mgmt.clusterVersion(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read the cluster version from the management API")
	}

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	for _, bucket := range buckets {
		log.Debug().Str("bucket", bucket.Name).Msg("Discovering bucket")

		var details *bucketDetails
		if s.config.IncludeStatistics {
			details, err = mgmt.bucket(ctx, bucket.Name)
			if err != nil {
				log.Warn().Err(err).Str("bucket", bucket.Name).Msg("Failed to read bucket statistics from the management API")
			}
		}

		scopes, err := s.listScopes(ctx, bucket.Name)
		if err != nil {
			log.Warn().Err(err).Str("bucket", bucket.Name).Msg("Failed to list scopes")
		}

		indexes := map[string][]gocb.QueryIndex{}
		if s.config.IncludeIndexes {
			indexes, err = s.listIndexes(ctx, bucket.Name)
			if err != nil {
				log.Warn().Err(err).Str("bucket", bucket.Name).Msg("Failed to list query indexes")
			}
		}

		bucketAsset := s.bucketAsset(bucket, details, scopes, clusterVersion)
		assets = append(assets, bucketAsset)
		if details != nil {
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   *bucketAsset.MRN,
				MetricName: "asset.size_bytes",
				Value:      float64(details.BasicStats.DataUsed),
			})
		}

		for _, scope := range scopes {
			for _, coll := range scope.Collections {
				asset, stats := s.collectionAsset(ctx, bucket.Name, scope.Name, coll, indexes, details, countCollections(scopes))
				assets = append(assets, asset)
				statistics = append(statistics, stats...)
				lineages = append(lineages, pluginsdk.LineageEdge{
					Source: *bucketAsset.MRN,
					Target: *asset.MRN,
					Type:   "CONTAINS",
				})
			}
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("Couchbase discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// listBuckets returns the buckets to discover, sorted by name so output is
// stable between runs.
func (s *Source) listBuckets(ctx context.Context) ([]gocb.BucketSettings, error) {
	all, err := s.cluster.Buckets().GetAllBuckets(&gocb.GetAllBucketsOptions{
		Context: ctx,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	if s.config.Bucket != "" {
		settings, ok := all[s.config.Bucket]
		if !ok {
			return nil, fmt.Errorf("bucket %q not found", s.config.Bucket)
		}
		return []gocb.BucketSettings{settings}, nil
	}

	excluded := make(map[string]struct{}, len(s.config.ExcludeBuckets))
	for _, name := range s.config.ExcludeBuckets {
		excluded[name] = struct{}{}
	}

	buckets := make([]gocb.BucketSettings, 0, len(all))
	for name, settings := range all {
		if _, skip := excluded[name]; skip {
			log.Debug().Str("bucket", name).Msg("Skipping excluded bucket")
			continue
		}
		buckets = append(buckets, settings)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Name < buckets[j].Name })

	return buckets, nil
}

// listScopes returns a bucket's scopes and their collections, dropping the
// _system scope unless asked to keep it. Output is sorted by scope and
// collection name.
func (s *Source) listScopes(ctx context.Context, bucket string) ([]gocb.ScopeSpec, error) {
	all, err := s.cluster.Bucket(bucket).CollectionsV2().GetAllScopes(&gocb.GetAllScopesOptions{
		Context: ctx,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	scopes := make([]gocb.ScopeSpec, 0, len(all))
	for _, scope := range all {
		if scope.Name == "_system" && !s.config.IncludeSystemScopes {
			continue
		}
		sort.Slice(scope.Collections, func(i, j int) bool {
			return scope.Collections[i].Name < scope.Collections[j].Name
		})
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Name < scopes[j].Name })

	return scopes, nil
}

// listIndexes reads every GSI index in a bucket in one query and groups
// them by "scope.collection".
func (s *Source) listIndexes(ctx context.Context, bucket string) (map[string][]gocb.QueryIndex, error) {
	indexes, err := s.cluster.QueryIndexes().GetAllIndexes(bucket, &gocb.GetAllQueryIndexesOptions{
		Context: ctx,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return groupIndexes(indexes), nil
}

func countCollections(scopes []gocb.ScopeSpec) int {
	count := 0
	for _, scope := range scopes {
		count += len(scope.Collections)
	}
	return count
}

func (s *Source) bucketAsset(bucket gocb.BucketSettings, details *bucketDetails, scopes []gocb.ScopeSpec, clusterVersion string) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"bucket_type":      bucketTypeName(bucket.BucketType),
		"ram_quota_mb":     bucket.RAMQuotaMB,
		"replicas":         bucket.NumReplicas,
		"flush_enabled":    bucket.FlushEnabled,
		"scope_count":      len(scopes),
		"collection_count": countCollections(scopes),
	}
	if bucket.StorageBackend != "" {
		metadata["storage_backend"] = string(bucket.StorageBackend)
	}
	if bucket.EvictionPolicy != "" {
		metadata["eviction_policy"] = string(bucket.EvictionPolicy)
	}
	if bucket.MaxExpiry > 0 {
		metadata["max_ttl"] = int64(bucket.MaxExpiry / time.Second)
	}
	if level := durabilityLevelName(bucket.MinimumDurabilityLevel); level != "" {
		metadata["durability_min_level"] = level
	}
	if clusterVersion != "" {
		metadata["cluster_version"] = clusterVersion
	}
	if details != nil {
		if details.ConflictResolutionType != "" {
			metadata["conflict_resolution"] = details.ConflictResolutionType
		}
		metadata["item_count"] = details.BasicStats.ItemCount
		metadata["disk_used_bytes"] = details.BasicStats.DiskUsed
		metadata["data_used_bytes"] = details.BasicStats.DataUsed
		metadata["mem_used_bytes"] = details.BasicStats.MemUsed
	}

	name := bucket.Name
	mrnValue := assetMRN("Bucket", name)
	description := fmt.Sprintf("Couchbase bucket %s", name)

	return pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Bucket",
		Providers:   []string{provider},
		Description: &description,
		Metadata:    metadata,
		Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) collectionAsset(ctx context.Context, bucket, scope string, coll gocb.CollectionSpec, indexes map[string][]gocb.QueryIndex, details *bucketDetails, collectionsInBucket int) (pluginsdk.Asset, []pluginsdk.Statistic) {
	name := collectionName(bucket, scope, coll.Name)
	mrnValue := assetMRN("Collection", name)
	keyspace := quoteKeyspace(bucket, scope, coll.Name)

	metadata := map[string]interface{}{
		"bucket":     bucket,
		"scope":      scope,
		"collection": coll.Name,
	}
	if coll.MaxExpiry > 0 {
		metadata["max_ttl"] = int64(coll.MaxExpiry / time.Second)
	}
	if coll.History != nil {
		metadata["history"] = coll.History.Enabled
	}

	if s.config.IncludeIndexes {
		collIndexes := indexes[scope+"."+coll.Name]
		metadata["index_count"] = len(collIndexes)
		metadata["primary_index"] = hasPrimaryIndex(collIndexes)
		if names := indexNames(collIndexes); len(names) > 0 {
			metadata["indexes"] = names
		}
	}

	var statistics []pluginsdk.Statistic

	if s.config.IncludeStatistics {
		count, ok := s.documentCount(ctx, keyspace, scope, coll.Name, details, collectionsInBucket)
		if ok {
			metadata["document_count"] = count
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   mrnValue,
				MetricName: "asset.row_count",
				Value:      float64(count),
			})
		}
	}

	description := fmt.Sprintf("Couchbase collection %s", name)
	asset := pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Collection",
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

	if s.config.IncludeColumns {
		columns := s.inferColumns(ctx, keyspace)
		if len(columns) > 0 {
			if err := pluginsdk.SetColumns(&asset, columns); err != nil {
				log.Warn().Err(err).Str("collection", name).Msg("Failed to set columns")
			} else if s.config.IncludeStatistics {
				statistics = append(statistics, pluginsdk.Statistic{
					AssetMRN:   mrnValue,
					MetricName: "asset.column_count",
					Value:      float64(len(columns)),
				})
			}
		}
	}

	return asset, statistics
}

// documentCount counts a collection's documents. COUNT(*) needs an index
// the query service can scan (any primary or secondary index, or a
// sequential scan on 7.6+). When that is missing the bucket's item count
// from the management API stands in, but only when the bucket holds a
// single collection, because that count spans the whole bucket.
func (s *Source) documentCount(ctx context.Context, keyspace, scope, collection string, details *bucketDetails, collectionsInBucket int) (int64, bool) {
	rows, err := s.query(ctx, "SELECT RAW COUNT(*) FROM "+keyspace, nil)
	if err == nil && len(rows) == 1 {
		var count int64
		if err := decodeRow(rows[0], &count); err == nil {
			return count, true
		}
	}
	if err != nil {
		if isIndexError(err) {
			log.Debug().Str("keyspace", keyspace).Msg("Skipping document count, no index available")
		} else {
			log.Warn().Err(err).Str("keyspace", keyspace).Msg("Failed to count documents")
		}
	}

	if details != nil && scope == "_default" && collection == "_default" && collectionsInBucket == 1 {
		return details.BasicStats.ItemCount, true
	}
	return 0, false
}

// inferColumns samples a collection's documents and merges them into a
// column list. Sampling reads the documents through the query service,
// which needs an index (or a 7.6+ sequential scan); without one INFER
// still works because it samples through the data service directly.
func (s *Source) inferColumns(ctx context.Context, keyspace string) []documentColumn {
	statement := fmt.Sprintf("SELECT META().id AS _id, d.* FROM %s AS d LIMIT %d", keyspace, s.config.SampleSize)
	rows, err := s.query(ctx, statement, nil)
	if err == nil {
		docs := make([]map[string]interface{}, 0, len(rows))
		for _, row := range rows {
			var doc map[string]interface{}
			if err := decodeRow(row, &doc); err != nil {
				log.Warn().Err(err).Str("keyspace", keyspace).Msg("Failed to decode sampled document")
				continue
			}
			delete(doc, "_id")
			docs = append(docs, doc)
		}
		return columnsFromDocuments(docs)
	}

	if isIndexError(err) {
		log.Debug().Str("keyspace", keyspace).Msg("No index for sampling, falling back to INFER")
	} else {
		log.Warn().Err(err).Str("keyspace", keyspace).Msg("Failed to sample documents, falling back to INFER")
	}

	statement = fmt.Sprintf("INFER %s WITH {\"sample_size\": %d}", keyspace, s.config.SampleSize)
	rows, err = s.query(ctx, statement, nil)
	if err != nil {
		log.Debug().Err(err).Str("keyspace", keyspace).Msg("INFER failed, emitting no columns")
		return nil
	}

	columns, err := columnsFromInfer(rows)
	if err != nil {
		log.Warn().Err(err).Str("keyspace", keyspace).Msg("Failed to parse INFER output")
		return nil
	}
	return columns
}

// FetchSampleData returns up to 20 documents from a collection for the
// asset preview.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}
	if a.Type != "Collection" {
		return nil, nil, fmt.Errorf("sample data is only available for collections, not %s", a.Type)
	}

	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	bucket, _ := a.Metadata["bucket"].(string)
	scope, _ := a.Metadata["scope"].(string)
	collection, _ := a.Metadata["collection"].(string)
	if bucket == "" || scope == "" || collection == "" {
		return nil, nil, fmt.Errorf("could not determine bucket, scope and collection from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.connect(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("connecting to Couchbase: %w", err)
	}
	defer s.close()

	keyspace := quoteKeyspace(bucket, scope, collection)
	rows, err := s.query(fetchCtx, "SELECT META().id AS id, d.* FROM "+keyspace+" AS d LIMIT 20", nil)
	if err != nil {
		if isIndexError(err) {
			return nil, nil, fmt.Errorf("a primary index is needed on %s to preview documents: %w", keyspace, err)
		}
		return nil, nil, fmt.Errorf("querying collection: %w", err)
	}

	docs := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var doc map[string]interface{}
		if err := decodeRow(row, &doc); err != nil {
			return nil, nil, fmt.Errorf("decoding document: %w", err)
		}
		docs = append(docs, doc)
	}

	columnNames, dataRows := tabulate(docs)
	return columnNames, dataRows, nil
}

// tabulate lays documents out as a table. Documents in one collection need
// not share fields, so the columns are the union of every key, with the
// document key first and the rest sorted; a field a document lacks is nil.
func tabulate(docs []map[string]interface{}) ([]string, [][]interface{}) {
	seen := make(map[string]struct{})
	for _, doc := range docs {
		for key := range doc {
			seen[key] = struct{}{}
		}
	}

	var columns []string
	for key := range seen {
		if key != "id" {
			columns = append(columns, key)
		}
	}
	sort.Strings(columns)
	if _, ok := seen["id"]; ok {
		columns = append([]string{"id"}, columns...)
	}

	rows := make([][]interface{}, 0, len(docs))
	for _, doc := range docs {
		row := make([]interface{}, len(columns))
		for i, key := range columns {
			row[i] = doc[key]
		}
		rows = append(rows, row)
	}

	return columns, rows
}

// collectionName is the identity of a collection: bucket.scope.collection,
// the same shape the OpenMetadata projection produces for Couchbase.
func collectionName(bucket, scope, collection string) string {
	return bucket + "." + scope + "." + collection
}

// assetMRN is the single place a Couchbase MRN is built, so the asset and
// lineage passes can never address one object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// quoteKeyspace builds a backtick-quoted keyspace path for N1QL. A
// backtick inside a name is escaped by doubling it.
func quoteKeyspace(bucket, scope, collection string) string {
	return quoteIdent(bucket) + "." + quoteIdent(scope) + "." + quoteIdent(collection)
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// bucketTypeName maps the SDK's bucket type, which still uses the old
// "membase" name for a Couchbase bucket, onto what the server UI shows.
func bucketTypeName(t gocb.BucketType) string {
	switch t {
	case gocb.CouchbaseBucketType, "couchbase":
		return "couchbase"
	case gocb.EphemeralBucketType:
		return "ephemeral"
	case gocb.MemcachedBucketType:
		return "memcached"
	}
	return string(t)
}

// durabilityLevelName names a minimum durability level the way the
// server's own REST API does. Unknown (unset) yields "".
func durabilityLevelName(level gocb.DurabilityLevel) string {
	switch level {
	case gocb.DurabilityLevelNone:
		return "none"
	case gocb.DurabilityLevelMajority:
		return "majority"
	case gocb.DurabilityLevelMajorityAndPersistOnMaster:
		return "majorityAndPersistActive"
	case gocb.DurabilityLevelPersistToMajority:
		return "persistToMajority"
	}
	return ""
}
