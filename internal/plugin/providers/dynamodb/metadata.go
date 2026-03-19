package dynamodb

// DynamoDBFields represents DynamoDB-specific metadata fields
// +marmot:metadata
type DynamoDBFields struct {
	TableArn             string            `json:"table_arn" metadata:"table_arn" description:"The ARN of the DynamoDB table"`
	TableStatus          string            `json:"table_status" metadata:"table_status" description:"Current status of the table (ACTIVE, CREATING, etc.)"`
	CreationDate         string            `json:"creation_date" metadata:"creation_date" description:"Date and time when the table was created"`
	TableClass           string            `json:"table_class" metadata:"table_class" description:"Table class (STANDARD or STANDARD_INFREQUENT_ACCESS)"`
	BillingMode          string            `json:"billing_mode" metadata:"billing_mode" description:"Billing mode of the table (PROVISIONED or PAY_PER_REQUEST)"`
	ReadCapacityUnits    int64             `json:"read_capacity_units" metadata:"read_capacity_units" description:"Provisioned read capacity units"`
	WriteCapacityUnits   int64             `json:"write_capacity_units" metadata:"write_capacity_units" description:"Provisioned write capacity units"`
	KeySchema            string            `json:"key_schema" metadata:"key_schema" description:"Key schema of the table (partition and sort keys)"`
	AttributeDefinitions string            `json:"attribute_definitions" metadata:"attribute_definitions" description:"Attribute definitions for the table's key schema"`
	GSICount             int               `json:"gsi_count" metadata:"gsi_count" description:"Number of global secondary indexes"`
	LSICount             int               `json:"lsi_count" metadata:"lsi_count" description:"Number of local secondary indexes"`
	StreamEnabled        string            `json:"stream_enabled" metadata:"stream_enabled" description:"Whether DynamoDB Streams is enabled"`
	StreamViewType       string            `json:"stream_view_type" metadata:"stream_view_type" description:"Stream view type (KEYS_ONLY, NEW_IMAGE, OLD_IMAGE, NEW_AND_OLD_IMAGES)"`
	EncryptionStatus     string            `json:"encryption_status" metadata:"encryption_status" description:"Status of server-side encryption"`
	EncryptionType       string            `json:"encryption_type" metadata:"encryption_type" description:"Type of server-side encryption (AES256 or KMS)"`
	TableSizeBytes       int64             `json:"table_size_bytes" metadata:"table_size_bytes" description:"Total size of the table in bytes"`
	ItemCount            int64             `json:"item_count" metadata:"item_count" description:"Number of items in the table"`
	DeletionProtection   string            `json:"deletion_protection" metadata:"deletion_protection" description:"Whether deletion protection is enabled"`
	GlobalTableReplicas  string            `json:"global_table_replicas" metadata:"global_table_replicas" description:"Regions where global table replicas exist"`
	TTLStatus            string            `json:"ttl_status" metadata:"ttl_status" description:"Time to Live status (ENABLED or DISABLED)"`
	TTLAttribute         string            `json:"ttl_attribute" metadata:"ttl_attribute" description:"Attribute name used for Time to Live"`
	ContinuousBackups    string            `json:"continuous_backups" metadata:"continuous_backups" description:"Continuous backups status (ENABLED or DISABLED)"`
	PITRStatus           string            `json:"pitr_status" metadata:"pitr_status" description:"Point-in-time recovery status (ENABLED or DISABLED)"`
	Tags                 map[string]string `json:"tags" metadata:"tags" description:"AWS resource tags"`
}
