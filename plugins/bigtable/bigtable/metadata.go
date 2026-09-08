package bigtable

// BigtableInstanceFields describes the metadata fields the Bigtable plugin
// emits for instance assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type BigtableInstanceFields struct {
	ProjectID    string            `json:"project_id" metadata:"project_id" description:"Google Cloud project holding the instance"`
	InstanceID   string            `json:"instance_id" metadata:"instance_id" description:"Instance ID"`
	DisplayName  string            `json:"display_name" metadata:"display_name" description:"Instance display name"`
	State        string            `json:"state" metadata:"state" description:"Instance state (READY, CREATING)"`
	InstanceType string            `json:"instance_type" metadata:"instance_type" description:"Instance type (PRODUCTION, DEVELOPMENT)"`
	Labels       map[string]string `json:"labels" metadata:"labels" description:"Labels set on the instance"`
	Clusters     []BigtableCluster `json:"clusters" metadata:"clusters" description:"Clusters serving the instance"`
	TableCount   int               `json:"table_count" metadata:"table_count" description:"Number of tables in the instance"`
	BackupCount  int               `json:"backup_count" metadata:"backup_count" description:"Number of backups held by the instance's clusters"`
	Emulator     bool              `json:"emulator" metadata:"emulator" description:"Whether the instance was read from an emulator"`
}

// BigtableCluster describes one cluster in an instance's clusters list.
type BigtableCluster struct {
	Name        string `json:"name" metadata:"name" description:"Cluster ID"`
	Zone        string `json:"zone" metadata:"zone" description:"Zone the cluster runs in"`
	Nodes       int    `json:"nodes" metadata:"nodes" description:"Number of nodes serving the cluster"`
	StorageType string `json:"storage_type" metadata:"storage_type" description:"Storage type (SSD, HDD)"`
}

// BigtableTableFields describes the metadata fields the Bigtable plugin
// emits for table assets.
type BigtableTableFields struct {
	ProjectID             string            `json:"project_id" metadata:"project_id" description:"Google Cloud project holding the table"`
	InstanceID            string            `json:"instance_id" metadata:"instance_id" description:"Instance the table belongs to"`
	TableName             string            `json:"table_name" metadata:"table_name" description:"Table name within the instance"`
	ColumnFamilies        []string          `json:"column_families" metadata:"column_families" description:"Column families defined on the table"`
	GCPolicies            map[string]string `json:"gc_policies" metadata:"gc_policies" description:"Garbage collection rule per column family"`
	DeletionProtection    bool              `json:"deletion_protection" metadata:"deletion_protection" description:"Whether the table is protected against deletion"`
	ChangeStreamRetention string            `json:"change_stream_retention" metadata:"change_stream_retention" description:"How long change data is retained"`
	SampledRows           int               `json:"sampled_rows" metadata:"sampled_rows" description:"Number of rows read to find the table's columns"`
	URL                   string            `json:"url" metadata:"url" description:"Google Cloud Console link to the table"`
}

// BigtableColumnFields describes the per-column fields embedded in a table
// asset's schema. Bigtable declares column families but not the qualifiers
// under them, so everything below comes from a sample of rows.
type BigtableColumnFields struct {
	ColumnName   string `json:"column_name" metadata:"column_name" description:"Column name in Bigtable's family:qualifier notation"`
	DataType     string `json:"data_type" metadata:"data_type" description:"Storage type, always bytes"`
	IsNullable   bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether the column may be missing from a row"`
	IsPrimaryKey bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is the row key"`
	ColumnFamily string `json:"column_family" metadata:"column_family" description:"Column family the column belongs to"`
	Qualifier    string `json:"qualifier" metadata:"qualifier" description:"Qualifier within the column family"`
	Occurrence   int    `json:"occurrence" metadata:"occurrence" description:"Number of sampled rows that held the column"`
	InferredType string `json:"inferred_type" metadata:"inferred_type" description:"What the sampled values looked like (text, int64, binary)"`
}
