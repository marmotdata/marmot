package starrocks

// StarRocksDatabaseFields describes the metadata on a Database asset.
// +marmot:metadata
type StarRocksDatabaseFields struct {
	Host             string `json:"host" metadata:"host" description:"Frontend hostname"`
	Port             int    `json:"port" metadata:"port" description:"Frontend MySQL protocol port"`
	Catalog          string `json:"catalog" metadata:"catalog" description:"Catalog the database belongs to"`
	CatalogType      string `json:"catalog_type" metadata:"catalog_type" description:"Catalog type (Internal, Hive, Iceberg, ...)"`
	Database         string `json:"database" metadata:"database" description:"Database name"`
	StarRocksVersion string `json:"starrocks_version" metadata:"starrocks_version" description:"StarRocks version reported by the frontend"`
	TableCount       int    `json:"table_count" metadata:"table_count" description:"Number of tables discovered in the database"`
	ViewCount        int    `json:"view_count" metadata:"view_count" description:"Number of views and materialized views discovered in the database"`
}

// StarRocksTableFields describes the metadata on Table and View assets.
// +marmot:metadata
type StarRocksTableFields struct {
	Host                string `json:"host" metadata:"host" description:"Frontend hostname"`
	Port                int    `json:"port" metadata:"port" description:"Frontend MySQL protocol port"`
	Catalog             string `json:"catalog" metadata:"catalog" description:"Catalog name"`
	Database            string `json:"database" metadata:"database" description:"Database name"`
	TableName           string `json:"table_name" metadata:"table_name" description:"Bare table or view name"`
	ObjectType          string `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view, external_table)"`
	Engine              string `json:"engine" metadata:"engine" description:"Storage engine as reported by information_schema"`
	ExternalEngine      string `json:"external_engine" metadata:"external_engine" description:"Engine of an external table (MySQL, Hive, JDBC, ...)"`
	KeyModel            string `json:"key_model" metadata:"key_model" description:"Table key model (DUPLICATE, AGGREGATE, UNIQUE, PRIMARY)"`
	KeyColumns          string `json:"key_columns" metadata:"key_columns" description:"Comma-separated key columns"`
	PartitionType       string `json:"partition_type" metadata:"partition_type" description:"Partitioning strategy (RANGE, LIST, EXPRESSION)"`
	PartitionColumns    string `json:"partition_columns" metadata:"partition_columns" description:"Comma-separated partition columns"`
	PartitionExpression string `json:"partition_expression" metadata:"partition_expression" description:"Partition expression for expression partitioning"`
	PartitionCount      int    `json:"partition_count" metadata:"partition_count" description:"Number of partitions"`
	Distribution        string `json:"distribution" metadata:"distribution" description:"Bucketing strategy (HASH, RANDOM)"`
	DistributionColumns string `json:"distribution_columns" metadata:"distribution_columns" description:"Comma-separated hash distribution columns"`
	Buckets             int    `json:"buckets" metadata:"buckets" description:"Number of buckets, when fixed"`
	OrderBy             string `json:"order_by" metadata:"order_by" description:"Comma-separated sort key columns from ORDER BY"`
	ReplicationNum      int    `json:"replication_num" metadata:"replication_num" description:"Replica count from table properties"`
	StorageVolume       string `json:"storage_volume" metadata:"storage_volume" description:"Storage volume from table properties"`
	Comment             string `json:"comment" metadata:"comment" description:"Table comment"`
	Created             string `json:"created" metadata:"created" description:"Creation timestamp"`
	Updated             string `json:"updated" metadata:"updated" description:"Last data change timestamp"`
	DDL                 string `json:"ddl" metadata:"ddl" description:"CREATE statement as printed by SHOW CREATE"`
	Materialized        bool   `json:"materialized" metadata:"materialized" description:"Whether a view is an asynchronous materialized view"`
	RefreshType         string `json:"refresh_type" metadata:"refresh_type" description:"Materialized view refresh type (ASYNC, MANUAL)"`
	IsActive            bool   `json:"is_active" metadata:"is_active" description:"Whether the materialized view is active"`
	LastRefreshState    string `json:"last_refresh_state" metadata:"last_refresh_state" description:"State of the last materialized view refresh"`
	LastRefreshStart    string `json:"last_refresh_start_time" metadata:"last_refresh_start_time" description:"Start time of the last materialized view refresh"`
	TaskName            string `json:"task_name" metadata:"task_name" description:"Refresh task name of the materialized view"`
}

// StarRocksColumnFields describes the per-column fields in an asset's
// schema.
// +marmot:metadata
type StarRocksColumnFields struct {
	ColumnName      string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType        string `json:"data_type" metadata:"data_type" description:"Column type as printed by SHOW FULL COLUMNS"`
	IsNullable      bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey    bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is in the PRIMARY KEY or UNIQUE KEY clause"`
	IsSortingKey    bool   `json:"is_sorting_key" metadata:"is_sorting_key" description:"Whether the column is in the DUPLICATE KEY, AGGREGATE KEY or ORDER BY clause"`
	IsKey           bool   `json:"is_key" metadata:"is_key" description:"Whether StarRocks reports the column as a key column"`
	AggregationType string `json:"aggregation_type" metadata:"aggregation_type" description:"Aggregate function of a value column in an AGGREGATE KEY table"`
	IsAutoIncrement bool   `json:"is_auto_increment" metadata:"is_auto_increment" description:"Whether the column is AUTO_INCREMENT"`
	Description     string `json:"description" metadata:"description" description:"Column comment"`
	Default         string `json:"default_expression" metadata:"default_expression" description:"Default value"`
}
