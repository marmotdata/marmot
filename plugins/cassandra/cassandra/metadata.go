package cassandra

// CassandraKeyspaceFields describes the metadata fields the Cassandra plugin
// emits for keyspace assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type CassandraKeyspaceFields struct {
	Keyspace           string         `json:"keyspace" metadata:"keyspace" description:"Keyspace name"`
	ReplicationClass   string         `json:"replication_class" metadata:"replication_class" description:"Replication strategy (SimpleStrategy, NetworkTopologyStrategy)"`
	ReplicationFactor  int            `json:"replication_factor" metadata:"replication_factor" description:"Replication factor when one applies to the whole keyspace"`
	ReplicationFactors map[string]int `json:"replication_factors" metadata:"replication_factors" description:"Replication factor per datacenter"`
	DurableWrites      bool           `json:"durable_writes" metadata:"durable_writes" description:"Whether writes go through the commit log"`
	TableCount         int            `json:"table_count" metadata:"table_count" description:"Number of tables in the keyspace"`
	ViewCount          int            `json:"view_count" metadata:"view_count" description:"Number of materialized views in the keyspace"`
	UserTypes          []string       `json:"user_types" metadata:"user_types" description:"User-defined types declared in the keyspace"`
	ClusterName        string         `json:"cluster_name" metadata:"cluster_name" description:"Cluster name"`
	CassandraVersion   string         `json:"cassandra_version" metadata:"cassandra_version" description:"Cassandra release version of the node queried"`
	Datacenter         string         `json:"datacenter" metadata:"datacenter" description:"Datacenter of the node queried"`
}

// CassandraTableFields describes the metadata fields emitted for table and
// materialized view assets.
type CassandraTableFields struct {
	Keyspace            string            `json:"keyspace" metadata:"keyspace" description:"Keyspace the table belongs to"`
	TableName           string            `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType          string            `json:"object_type" metadata:"object_type" description:"Object type (table, view)"`
	ID                  string            `json:"id" metadata:"id" description:"Table id"`
	Comment             string            `json:"comment" metadata:"comment" description:"Table comment"`
	CompactionClass     string            `json:"compaction_class" metadata:"compaction_class" description:"Compaction strategy"`
	CompressionClass    string            `json:"compression_class" metadata:"compression_class" description:"Compressor"`
	DefaultTTL          int               `json:"default_ttl" metadata:"default_ttl" description:"Default time to live in seconds, 0 for none"`
	GCGraceSeconds      int               `json:"gc_grace_seconds" metadata:"gc_grace_seconds" description:"Seconds tombstones are kept before collection"`
	BloomFilterFPChance float64           `json:"bloom_filter_fp_chance" metadata:"bloom_filter_fp_chance" description:"Bloom filter false positive chance"`
	Caching             map[string]string `json:"caching" metadata:"caching" description:"Key and row cache settings"`
	Flags               []string          `json:"flags" metadata:"flags" description:"Table flags (compound, counter, dense, super)"`
	IsCounter           bool              `json:"is_counter" metadata:"is_counter" description:"Whether the table holds counter columns"`
	PartitionKey        []string          `json:"partition_key" metadata:"partition_key" description:"Partition key columns in key order"`
	ClusteringColumns   []string          `json:"clustering_columns" metadata:"clustering_columns" description:"Clustering columns in key order"`
	ClusteringOrder     map[string]string `json:"clustering_order" metadata:"clustering_order" description:"Sort direction (asc, desc) per clustering column"`
	Indexes             []string          `json:"indexes" metadata:"indexes" description:"Secondary index names"`
	IndexCount          int               `json:"index_count" metadata:"index_count" description:"Number of secondary indexes"`
	BaseTable           string            `json:"base_table" metadata:"base_table" description:"Table a materialized view is built from"`
	WhereClause         string            `json:"where_clause" metadata:"where_clause" description:"Filter a materialized view applies to its base table"`
	IncludeAllColumns   bool              `json:"include_all_columns" metadata:"include_all_columns" description:"Whether a materialized view selects every base table column"`
}

// CassandraColumnFields describes the per-column fields embedded in an
// asset's schema.
type CassandraColumnFields struct {
	ColumnName      string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType        string `json:"data_type" metadata:"data_type" description:"CQL type as stored, for example map<text, int> or frozen<address>"`
	IsNullable      bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether the column can be null (false for key columns)"`
	IsPrimaryKey    bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the partition or clustering key"`
	Kind            string `json:"kind" metadata:"kind" description:"Column kind (partition_key, clustering, regular, static)"`
	Position        int    `json:"position" metadata:"position" description:"Index within the partition or clustering key, -1 otherwise"`
	ClusteringOrder string `json:"clustering_order" metadata:"clustering_order" description:"Sort direction (asc, desc) for clustering columns"`
}
