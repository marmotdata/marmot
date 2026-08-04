package openmetadata

// OpenMetadataFields records where in OpenMetadata an asset came from.
// It is set on every asset this plugin creates.
// +marmot:metadata
type OpenMetadataFields struct {
	ID          string `json:"openmetadata.id" metadata:"openmetadata.id" description:"OpenMetadata entity id"`
	FQN         string `json:"openmetadata.fqn" metadata:"openmetadata.fqn" description:"Fully qualified name of the entity in OpenMetadata"`
	Service     string `json:"openmetadata.service" metadata:"openmetadata.service" description:"OpenMetadata service the entity belongs to"`
	ServiceType string `json:"openmetadata.service_type" metadata:"openmetadata.service_type" description:"OpenMetadata service type, for example Postgres or Looker"`
	UpdatedAt   string `json:"openmetadata.updated_at" metadata:"openmetadata.updated_at" description:"When the entity last changed in OpenMetadata"`
	URL         string `json:"openmetadata.url" metadata:"openmetadata.url" description:"Address of the entity in the OpenMetadata UI"`
}

// CurationFields represents the curated context OpenMetadata holds that
// a Marmot ingestion run cannot yet create as first class objects, so it
// travels with the asset instead.
// +marmot:metadata
type CurationFields struct {
	Owners        []string `json:"owners" metadata:"owners" description:"Users or teams that own the entity in OpenMetadata"`
	Domains       []string `json:"domains" metadata:"domains" description:"OpenMetadata domains the entity belongs to"`
	DataProducts  []string `json:"data_products" metadata:"data_products" description:"OpenMetadata data products the entity belongs to"`
	GlossaryTerms []string `json:"glossary_terms" metadata:"glossary_terms" description:"Glossary terms assigned to the entity"`
}

// TableFields represents metadata on tables, views and stored
// procedures. The field names match Marmot's own database plugins so a
// merged asset reads the same whichever run contributed it.
// +marmot:metadata
type TableFields struct {
	Database         string   `json:"database" metadata:"database" description:"Database name"`
	Schema           string   `json:"schema" metadata:"schema" description:"Schema name"`
	TableName        string   `json:"table_name" metadata:"table_name" description:"Object name"`
	ObjectType       string   `json:"object_type" metadata:"object_type" description:"OpenMetadata table type, for example Regular, View or MaterializedView"`
	ColumnCount      int      `json:"column_count" metadata:"column_count" description:"Number of columns"`
	RowCount         int64    `json:"row_count" metadata:"row_count" description:"Row count from the OpenMetadata profiler"`
	Size             int64    `json:"size" metadata:"size" description:"Size in bytes from the OpenMetadata profiler"`
	WeeklyQueryCount int      `json:"weekly_query_count" metadata:"weekly_query_count" description:"Queries against the table in the last week"`
	PrimaryKey       []string `json:"primary_key" metadata:"primary_key" description:"Columns forming the primary key"`
	ProcedureType    string   `json:"procedure_type" metadata:"procedure_type" description:"Stored procedure type"`
}

// TopicFields represents metadata on messaging topics.
// +marmot:metadata
type TopicFields struct {
	Partitions        int      `json:"partitions" metadata:"partitions" description:"Number of partitions"`
	ReplicationFactor int      `json:"replication_factor" metadata:"replication_factor" description:"Replication factor"`
	RetentionSize     int64    `json:"retention_size" metadata:"retention_size" description:"Retention size in bytes"`
	RetentionMs       int64    `json:"retention_ms" metadata:"retention_ms" description:"Retention time in milliseconds"`
	MaxMessageSize    int      `json:"max_message_size" metadata:"max_message_size" description:"Maximum message size in bytes"`
	CleanupPolicies   []string `json:"cleanup_policies" metadata:"cleanup_policies" description:"Topic cleanup policies"`
	SchemaType        string   `json:"schema_type" metadata:"schema_type" description:"Message schema type, for example Avro or JSON"`
}

// ContainerFields represents metadata on object storage containers.
// +marmot:metadata
type ContainerFields struct {
	Bucket      string   `json:"bucket" metadata:"bucket" description:"Top level container the object lives in"`
	Prefix      string   `json:"prefix" metadata:"prefix" description:"Path prefix within the bucket"`
	Size        int64    `json:"size" metadata:"size" description:"Size in bytes"`
	ObjectCount int64    `json:"object_count" metadata:"object_count" description:"Number of objects"`
	FileFormats []string `json:"file_formats" metadata:"file_formats" description:"File formats found in the container"`
	Partitioned bool     `json:"partitioned" metadata:"partitioned" description:"Whether the container is partitioned"`
}

// DashboardFields represents metadata on dashboards, charts and
// dashboard data models.
// +marmot:metadata
type DashboardFields struct {
	DashboardType string `json:"dashboard_type" metadata:"dashboard_type" description:"Dashboard type reported by the BI tool"`
	ChartType     string `json:"chart_type" metadata:"chart_type" description:"Chart type reported by the BI tool"`
	DataModelType string `json:"data_model_type" metadata:"data_model_type" description:"Data model type reported by the BI tool"`
	Project       string `json:"project" metadata:"project" description:"Project or workspace the dashboard belongs to"`
	ChartCount    int    `json:"chart_count" metadata:"chart_count" description:"Number of charts on the dashboard"`
}

// PipelineFields represents metadata on pipelines and their tasks.
// +marmot:metadata
type PipelineFields struct {
	ScheduleInterval string   `json:"schedule_interval" metadata:"schedule_interval" description:"Schedule the pipeline runs on"`
	Concurrency      int      `json:"concurrency" metadata:"concurrency" description:"Maximum concurrent runs"`
	TaskCount        int      `json:"task_count" metadata:"task_count" description:"Number of tasks in the pipeline"`
	Pipeline         string   `json:"pipeline" metadata:"pipeline" description:"Pipeline a task belongs to"`
	TaskType         string   `json:"task_type" metadata:"task_type" description:"Task type, for example the Airflow operator"`
	DownstreamTasks  []string `json:"downstream_tasks" metadata:"downstream_tasks" description:"Tasks that run after this one"`
}

// MLModelFields represents metadata on machine learning models.
// +marmot:metadata
type MLModelFields struct {
	Algorithm       string `json:"algorithm" metadata:"algorithm" description:"Algorithm the model uses"`
	Target          string `json:"target" metadata:"target" description:"Column the model predicts"`
	Server          string `json:"server" metadata:"server" description:"Address the model is served from"`
	FeatureCount    int    `json:"feature_count" metadata:"feature_count" description:"Number of features"`
	Storage         string `json:"storage" metadata:"storage" description:"Where the model artefact is stored"`
	ImageRepository string `json:"image_repository" metadata:"image_repository" description:"Repository holding the model image"`
}

// SearchIndexFields represents metadata on search indices.
// +marmot:metadata
type SearchIndexFields struct {
	IndexType  string `json:"index_type" metadata:"index_type" description:"Index type"`
	FieldCount int    `json:"field_count" metadata:"field_count" description:"Number of fields in the index"`
}

// APIFields represents metadata on API collections and endpoints.
// +marmot:metadata
type APIFields struct {
	Collection  string `json:"collection" metadata:"collection" description:"API collection the endpoint belongs to"`
	Method      string `json:"method" metadata:"method" description:"HTTP method"`
	EndpointURL string `json:"endpoint_url" metadata:"endpoint_url" description:"URL of the endpoint"`
	Path        string `json:"path" metadata:"path" description:"Request path"`
}
