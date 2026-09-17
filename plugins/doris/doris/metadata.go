package doris

// DorisFields describes the metadata fields the Doris plugin emits for
// database, table and view assets. It is kept as a documentation-only
// struct so downstream tooling can introspect the shape of the metadata map.
type DorisFields struct {
	Host                string   `json:"host" metadata:"host" description:"Doris frontend hostname"`
	Port                int      `json:"port" metadata:"port" description:"Doris frontend MySQL protocol port"`
	Catalog             string   `json:"catalog" metadata:"catalog" description:"Doris catalog the object lives in"`
	Database            string   `json:"database" metadata:"database" description:"Database name"`
	DorisVersion        string   `json:"doris_version" metadata:"doris_version" description:"Doris build the frontend reports"`
	TableCount          int      `json:"table_count" metadata:"table_count" description:"Tables in the database"`
	ViewCount           int      `json:"view_count" metadata:"view_count" description:"Views and materialized views in the database"`
	TableName           string   `json:"table_name" metadata:"table_name" description:"Object name without the database"`
	ObjectType          string   `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view, external_table)"`
	Engine              string   `json:"engine" metadata:"engine" description:"Storage engine (Doris, View, MATERIALIZED_VIEW, or the external system)"`
	KeyModel            string   `json:"key_model" metadata:"key_model" description:"Table model (DUPLICATE, UNIQUE, AGGREGATE, none)"`
	KeyColumns          []string `json:"key_columns" metadata:"key_columns" description:"Columns that make up the key"`
	PartitionType       string   `json:"partition_type" metadata:"partition_type" description:"Partitioning (RANGE, LIST, none)"`
	PartitionColumns    []string `json:"partition_columns" metadata:"partition_columns" description:"Columns or expressions the table is partitioned by"`
	PartitionCount      int      `json:"partition_count" metadata:"partition_count" description:"Number of partitions"`
	DistributionType    string   `json:"distribution_type" metadata:"distribution_type" description:"Bucketing (HASH, RANDOM)"`
	DistributionColumns []string `json:"distribution_columns" metadata:"distribution_columns" description:"Columns the data is hashed on"`
	Buckets             int      `json:"buckets" metadata:"buckets" description:"Number of buckets per partition"`
	AutoBucket          bool     `json:"auto_bucket" metadata:"auto_bucket" description:"Whether Doris picks the bucket count"`
	Replication         string   `json:"replication" metadata:"replication" description:"Replication allocation or replica count"`
	StorageMedium       string   `json:"storage_medium" metadata:"storage_medium" description:"Storage medium (hdd, ssd)"`
	Comment             string   `json:"comment" metadata:"comment" description:"Table or view comment"`
	Created             string   `json:"created" metadata:"created" description:"Creation timestamp"`
	Updated             string   `json:"updated" metadata:"updated" description:"Last update timestamp"`
	DDL                 string   `json:"ddl" metadata:"ddl" description:"SHOW CREATE output"`
	Materialized        bool     `json:"materialized" metadata:"materialized" description:"Whether the view is an async materialized view"`
	JobName             string   `json:"job_name" metadata:"job_name" description:"Refresh job of a materialized view"`
	State               string   `json:"state" metadata:"state" description:"State of a materialized view (NORMAL, SCHEMA_CHANGE)"`
	RefreshState        string   `json:"refresh_state" metadata:"refresh_state" description:"Outcome of the last materialized view refresh"`
	RefreshInfo         string   `json:"refresh_info" metadata:"refresh_info" description:"Build and refresh settings of a materialized view"`
	MvPartitionInfo     string   `json:"mv_partition_info" metadata:"mv_partition_info" description:"How a materialized view is partitioned"`
}

// DorisColumnFields describes the per-column fields embedded in an asset's
// schema.
type DorisColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Declared type as Doris prints it (decimalv3(9, 2), array<int>)"`
	IsNullable        bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey      bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Key column of a UNIQUE or AGGREGATE table"`
	IsSortingKey      bool   `json:"is_sorting_key" metadata:"is_sorting_key" description:"Key column of a DUPLICATE table"`
	IsKey             bool   `json:"is_key" metadata:"is_key" description:"Whether the column is part of the table key"`
	AggregationType   string `json:"aggregation_type" metadata:"aggregation_type" description:"Aggregation of a value column on an AGGREGATE table (SUM, MAX, REPLACE, HLL_UNION, BITMAP_UNION)"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value"`
	Description       string `json:"description" metadata:"description" description:"Column comment"`
}
