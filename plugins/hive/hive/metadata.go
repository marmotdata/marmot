package hive

// HiveDatabaseFields describes the metadata fields the Hive plugin emits
// for database assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type HiveDatabaseFields struct {
	Host        string            `json:"host" metadata:"host" description:"HiveServer2 hostname"`
	Port        int               `json:"port" metadata:"port" description:"HiveServer2 port"`
	Database    string            `json:"database" metadata:"database" description:"Database name"`
	Comment     string            `json:"comment" metadata:"comment" description:"Database comment"`
	Location    string            `json:"location" metadata:"location" description:"Warehouse location of the database"`
	Owner       string            `json:"owner" metadata:"owner" description:"Database owner"`
	OwnerType   string            `json:"owner_type" metadata:"owner_type" description:"Owner type (USER, ROLE, GROUP)"`
	Parameters  map[string]string `json:"parameters" metadata:"parameters" description:"Database properties (DBPROPERTIES)"`
	HiveVersion string            `json:"hive_version" metadata:"hive_version" description:"Hive version reported by the server"`
	TableCount  int               `json:"table_count" metadata:"table_count" description:"Number of tables discovered in the database"`
	ViewCount   int               `json:"view_count" metadata:"view_count" description:"Number of views and materialized views discovered in the database"`
}

// HiveTableFields describes the metadata fields the Hive plugin emits for
// table and view assets.
type HiveTableFields struct {
	Host             string            `json:"host" metadata:"host" description:"HiveServer2 hostname"`
	Port             int               `json:"port" metadata:"port" description:"HiveServer2 port"`
	Database         string            `json:"database" metadata:"database" description:"Database the object belongs to"`
	TableName        string            `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType       string            `json:"object_type" metadata:"object_type" description:"Object type (managed, external, view, materialized_view)"`
	Owner            string            `json:"owner" metadata:"owner" description:"Object owner"`
	OwnerType        string            `json:"owner_type" metadata:"owner_type" description:"Owner type (USER, ROLE, GROUP)"`
	Created          string            `json:"created" metadata:"created" description:"Creation time (RFC 3339)"`
	LastAccess       string            `json:"last_access" metadata:"last_access" description:"Last access time (RFC 3339), when Hive tracks it"`
	LastDDL          string            `json:"last_ddl" metadata:"last_ddl" description:"Time of the last DDL change (RFC 3339)"`
	Location         string            `json:"location" metadata:"location" description:"Storage location"`
	InputFormat      string            `json:"input_format" metadata:"input_format" description:"Hadoop input format class"`
	OutputFormat     string            `json:"output_format" metadata:"output_format" description:"Hadoop output format class"`
	Serde            string            `json:"serde" metadata:"serde" description:"SerDe class"`
	Compressed       bool              `json:"compressed" metadata:"compressed" description:"Whether the storage is marked compressed"`
	NumBuckets       int               `json:"num_buckets" metadata:"num_buckets" description:"Number of buckets, for bucketed tables"`
	BucketColumns    []string          `json:"bucket_columns" metadata:"bucket_columns" description:"Columns the table is bucketed by"`
	SortColumns      []string          `json:"sort_columns" metadata:"sort_columns" description:"Columns each bucket is sorted by"`
	Transactional    bool              `json:"transactional" metadata:"transactional" description:"Whether the table is ACID (transactional)"`
	NumFiles         int64             `json:"num_files" metadata:"num_files" description:"Number of files, from table statistics"`
	Comment          string            `json:"comment" metadata:"comment" description:"Table comment"`
	PartitionColumns []string          `json:"partition_columns" metadata:"partition_columns" description:"Partition columns"`
	PartitionCount   int               `json:"partition_count" metadata:"partition_count" description:"Number of partitions"`
	PartitionValues  []string          `json:"partition_values" metadata:"partition_values" description:"First 20 partition specs (dt=2026-01-01)"`
	Parameters       map[string]string `json:"parameters" metadata:"parameters" description:"Table properties not surfaced as their own field"`
	DDL              string            `json:"ddl" metadata:"ddl" description:"SHOW CREATE TABLE output, when include_ddl is set"`
}

// HiveColumnFields describes the per-column fields embedded in an asset's
// schema.
type HiveColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Hive data type as declared, complex types included"`
	IsNullable        bool   `json:"is_nullable" metadata:"is_nullable" description:"False when a NOT NULL constraint covers the column"`
	IsPrimaryKey      bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key constraint"`
	IsPartitionColumn bool   `json:"is_partition_column" metadata:"is_partition_column" description:"Whether the column is a partition column"`
	Description       string `json:"description" metadata:"description" description:"Column comment"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value from a DEFAULT constraint"`
}
