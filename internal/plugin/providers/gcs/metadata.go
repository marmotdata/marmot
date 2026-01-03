package gcs

// GCSBucketFields defines metadata fields for GCS buckets
// +marmot:metadata
type GCSBucketFields struct {
	BucketName             string `json:"bucket_name" metadata:"bucket_name" description:"Name of the bucket"`
	Location               string `json:"location" metadata:"location" description:"Geographic location of the bucket"`
	LocationType           string `json:"location_type" metadata:"location_type" description:"Location type (region, dual-region, multi-region)"`
	StorageClass           string `json:"storage_class" metadata:"storage_class" description:"Default storage class (STANDARD, NEARLINE, COLDLINE, ARCHIVE)"`
	Created                string `json:"created" metadata:"created" description:"Bucket creation timestamp"`
	Versioning             string `json:"versioning" metadata:"versioning" description:"Whether object versioning is enabled"`
	RequesterPays          bool   `json:"requester_pays" metadata:"requester_pays" description:"Whether requester pays for access"`
	Encryption             string `json:"encryption" metadata:"encryption" description:"Encryption type (google-managed or customer-managed)"`
	KMSKey                 string `json:"kms_key" metadata:"kms_key" description:"Customer-managed encryption key name"`
	LoggingEnabled         bool   `json:"logging_enabled" metadata:"logging_enabled" description:"Whether access logging is enabled"`
	LifecycleRulesCount    int    `json:"lifecycle_rules_count" metadata:"lifecycle_rules_count" description:"Number of lifecycle rules configured"`
	RetentionPeriodSeconds int64  `json:"retention_period_seconds" metadata:"retention_period_seconds" description:"Retention period in seconds"`
	ObjectCount            int64  `json:"object_count" metadata:"object_count" description:"Number of objects in the bucket"`
}
