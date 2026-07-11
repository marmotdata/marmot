package s3

// S3Fields represents S3-specific metadata fields
// +marmot:metadata
type S3Fields struct {
	BucketArn            string            `json:"bucket_arn" metadata:"bucket_arn" description:"The ARN of the S3 bucket"`
	Region               string            `json:"region" metadata:"region" description:"The AWS region where the bucket is located"`
	CreationDate         string            `json:"creation_date" metadata:"creation_date" description:"When the bucket was created"`
	Versioning           string            `json:"versioning" metadata:"versioning" description:"Bucket versioning status"`
	Encryption           string            `json:"encryption" metadata:"encryption" description:"Bucket encryption configuration"`
	PublicAccessBlock    string            `json:"public_access_block" metadata:"public_access_block" description:"Public access block configuration"`
	NotificationConfig   string            `json:"notification_config" metadata:"notification_config" description:"Bucket notification configuration"`
	LifecycleConfig      string            `json:"lifecycle_config" metadata:"lifecycle_config" description:"Bucket lifecycle configuration"`
	ReplicationConfig    string            `json:"replication_config" metadata:"replication_config" description:"Bucket replication configuration"`
	WebsiteConfig        string            `json:"website_config" metadata:"website_config" description:"Static website hosting configuration"`
	LoggingConfig        string            `json:"logging_config" metadata:"logging_config" description:"Bucket access logging configuration"`
	AccelerateConfig     string            `json:"accelerate_config" metadata:"accelerate_config" description:"Transfer acceleration configuration"`
	RequestPaymentConfig string            `json:"request_payment_config" metadata:"request_payment_config" description:"Request payment configuration"`
	Tags                 map[string]string `json:"tags" metadata:"tags" description:"AWS resource tags"`
}
