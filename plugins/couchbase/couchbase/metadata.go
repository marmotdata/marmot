package couchbase

// CouchbaseBucketFields describes the metadata fields the Couchbase plugin
// emits for bucket assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type CouchbaseBucketFields struct {
	BucketType         string `json:"bucket_type" metadata:"bucket_type" description:"Bucket type (couchbase, ephemeral, memcached)"`
	StorageBackend     string `json:"storage_backend" metadata:"storage_backend" description:"Storage backend (couchstore, magma)"`
	RAMQuotaMB         int64  `json:"ram_quota_mb" metadata:"ram_quota_mb" description:"Memory quota per node in MB"`
	Replicas           int    `json:"replicas" metadata:"replicas" description:"Number of replica copies"`
	EvictionPolicy     string `json:"eviction_policy" metadata:"eviction_policy" description:"Eviction policy (valueOnly, fullEviction, noEviction, nruEviction)"`
	ConflictResolution string `json:"conflict_resolution" metadata:"conflict_resolution" description:"XDCR conflict resolution type (seqno, lww, custom)"`
	MaxTTL             int64  `json:"max_ttl" metadata:"max_ttl" description:"Maximum document expiry in seconds, when set"`
	DurabilityMinLevel string `json:"durability_min_level" metadata:"durability_min_level" description:"Minimum durability level for writes"`
	FlushEnabled       bool   `json:"flush_enabled" metadata:"flush_enabled" description:"Whether the bucket can be flushed"`
	ScopeCount         int    `json:"scope_count" metadata:"scope_count" description:"Number of discovered scopes"`
	CollectionCount    int    `json:"collection_count" metadata:"collection_count" description:"Number of discovered collections"`
	ItemCount          int64  `json:"item_count" metadata:"item_count" description:"Number of documents in the bucket"`
	DiskUsedBytes      int64  `json:"disk_used_bytes" metadata:"disk_used_bytes" description:"Disk space used by the bucket in bytes"`
	DataUsedBytes      int64  `json:"data_used_bytes" metadata:"data_used_bytes" description:"Size of the bucket's data in bytes"`
	MemUsedBytes       int64  `json:"mem_used_bytes" metadata:"mem_used_bytes" description:"Memory used by the bucket in bytes"`
	ClusterVersion     string `json:"cluster_version" metadata:"cluster_version" description:"Couchbase Server version"`
}

// CouchbaseCollectionFields describes the metadata fields the Couchbase
// plugin emits for collection assets.
type CouchbaseCollectionFields struct {
	Bucket        string   `json:"bucket" metadata:"bucket" description:"Bucket name"`
	Scope         string   `json:"scope" metadata:"scope" description:"Scope name"`
	Collection    string   `json:"collection" metadata:"collection" description:"Collection name"`
	MaxTTL        int64    `json:"max_ttl" metadata:"max_ttl" description:"Maximum document expiry in seconds, when set"`
	History       bool     `json:"history" metadata:"history" description:"Whether change history retention is enabled"`
	IndexCount    int      `json:"index_count" metadata:"index_count" description:"Number of query indexes on the collection"`
	Indexes       []string `json:"indexes" metadata:"indexes" description:"Query index names"`
	PrimaryIndex  bool     `json:"primary_index" metadata:"primary_index" description:"Whether the collection has a primary index"`
	DocumentCount int64    `json:"document_count" metadata:"document_count" description:"Number of documents in the collection"`
}

// CouchbaseColumnFields describes the per-field entries embedded in a
// collection asset's schema, inferred from sampled documents.
type CouchbaseColumnFields struct {
	ColumnName string  `json:"column_name" metadata:"column_name" description:"Field name, with nested fields as parent.child"`
	DataType   string  `json:"data_type" metadata:"data_type" description:"Observed JSON type, joined with | when mixed"`
	IsNullable bool    `json:"is_nullable" metadata:"is_nullable" description:"Whether the field is absent from some sampled documents"`
	Occurrence float64 `json:"occurrence" metadata:"occurrence" description:"Fraction of sampled documents holding the field"`
}
