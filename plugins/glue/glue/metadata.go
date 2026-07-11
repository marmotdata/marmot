package glue

// GlueJobFields represents Glue Job metadata fields
// +marmot:metadata
type GlueJobFields struct {
	Role                  string `json:"role" metadata:"role" description:"IAM role ARN assigned to the job"`
	Type                  string `json:"type" metadata:"type" description:"Job command type (glueetl, pythonshell, gluestreaming)"`
	GlueVersion           string `json:"glue_version" metadata:"glue_version" description:"Glue version used by the job"`
	WorkerType            string `json:"worker_type" metadata:"worker_type" description:"Worker type (Standard, G.1X, G.2X, etc.)"`
	NumberOfWorkers       int32  `json:"number_of_workers" metadata:"number_of_workers" description:"Number of workers allocated to the job"`
	MaxCapacity           float64 `json:"max_capacity" metadata:"max_capacity" description:"Maximum number of DPU that can be allocated"`
	Timeout               int32  `json:"timeout" metadata:"timeout" description:"Job timeout in minutes"`
	MaxRetries            int    `json:"max_retries" metadata:"max_retries" description:"Maximum number of retries"`
	ScriptLocation        string `json:"script_location" metadata:"script_location" description:"S3 location of the job script"`
	Connections           string `json:"connections" metadata:"connections" description:"Connections used by the job"`
	CreatedOn             string `json:"created_on" metadata:"created_on" description:"Date and time the job was created"`
	LastModifiedOn        string `json:"last_modified_on" metadata:"last_modified_on" description:"Date and time the job was last modified"`
	SecurityConfiguration string `json:"security_configuration" metadata:"security_configuration" description:"Security configuration applied to the job"`
}

// GlueDatabaseFields represents Glue Database metadata fields
// +marmot:metadata
type GlueDatabaseFields struct {
	CatalogId   string `json:"catalog_id" metadata:"catalog_id" description:"ID of the Data Catalog"`
	LocationUri string `json:"location_uri" metadata:"location_uri" description:"Location of the database"`
	Description string `json:"description" metadata:"description" description:"Description of the database"`
	CreateTime  string `json:"create_time" metadata:"create_time" description:"Date and time the database was created"`
	Parameters  string `json:"parameters" metadata:"parameters" description:"Database parameters"`
}

// GlueTableFields represents Glue Table metadata fields
// +marmot:metadata
type GlueTableFields struct {
	DatabaseName string `json:"database_name" metadata:"database_name" description:"Name of the database containing the table"`
	TableType    string `json:"table_type" metadata:"table_type" description:"Type of table (EXTERNAL_TABLE, VIRTUAL_VIEW, etc.)"`
	Classification string `json:"classification" metadata:"classification" description:"Classification of the table data (csv, parquet, json, etc.)"`
	Owner        string `json:"owner" metadata:"owner" description:"Owner of the table"`
	Location     string `json:"location" metadata:"location" description:"S3 location of the table data"`
	InputFormat  string `json:"input_format" metadata:"input_format" description:"Hadoop input format class"`
	OutputFormat string `json:"output_format" metadata:"output_format" description:"Hadoop output format class"`
	Serde        string `json:"serde" metadata:"serde" description:"Serialization/deserialization library"`
	PartitionKeys string `json:"partition_keys" metadata:"partition_keys" description:"Partition key columns"`
	CreateTime   string `json:"create_time" metadata:"create_time" description:"Date and time the table was created"`
	UpdateTime   string `json:"update_time" metadata:"update_time" description:"Date and time the table was last updated"`
	Retention    int32  `json:"retention" metadata:"retention" description:"Retention period in days"`
}

// GlueCrawlerFields represents Glue Crawler metadata fields
// +marmot:metadata
type GlueCrawlerFields struct {
	Role                 string `json:"role" metadata:"role" description:"IAM role ARN assigned to the crawler"`
	DatabaseName         string `json:"database_name" metadata:"database_name" description:"Target database for the crawler"`
	State                string `json:"state" metadata:"state" description:"Current state of the crawler (READY, RUNNING, STOPPING)"`
	Schedule             string `json:"schedule" metadata:"schedule" description:"Cron schedule expression"`
	Targets              string `json:"targets" metadata:"targets" description:"Summary of crawler targets"`
	SchemaUpdateBehavior string `json:"schema_update_behavior" metadata:"schema_update_behavior" description:"Behavior when schema changes are detected"`
	SchemaDeleteBehavior string `json:"schema_delete_behavior" metadata:"schema_delete_behavior" description:"Behavior when schema objects are deleted"`
	RecrawlBehavior      string `json:"recrawl_behavior" metadata:"recrawl_behavior" description:"Recrawl behavior policy"`
	CreationTime         string `json:"creation_time" metadata:"creation_time" description:"Date and time the crawler was created"`
	LastUpdated          string `json:"last_updated" metadata:"last_updated" description:"Date and time the crawler was last updated"`
	LastCrawlStatus      string `json:"last_crawl_status" metadata:"last_crawl_status" description:"Status of the last crawl"`
	LastCrawlTime        string `json:"last_crawl_time" metadata:"last_crawl_time" description:"Start time of the last crawl"`
	LastCrawlError       string `json:"last_crawl_error" metadata:"last_crawl_error" description:"Error message from the last crawl"`
	Classifiers          string `json:"classifiers" metadata:"classifiers" description:"Custom classifiers used by the crawler"`
}
