package kinesis

// KinesisStreamFields describes the metadata fields the Kinesis plugin
// emits for Stream assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type KinesisStreamFields struct {
	ARN                string   `json:"arn" metadata:"arn" description:"Stream ARN"`
	Status             string   `json:"status" metadata:"status" description:"Stream status (CREATING, DELETING, ACTIVE, UPDATING)"`
	StreamMode         string   `json:"stream_mode" metadata:"stream_mode" description:"Capacity mode (PROVISIONED or ON_DEMAND)"`
	RetentionHours     int      `json:"retention_hours" metadata:"retention_hours" description:"Record retention period in hours"`
	ShardCount         int      `json:"shard_count" metadata:"shard_count" description:"Total number of shards, closed ones included"`
	OpenShardCount     int      `json:"open_shard_count" metadata:"open_shard_count" description:"Number of shards still accepting writes"`
	EncryptionType     string   `json:"encryption_type" metadata:"encryption_type" description:"Server-side encryption type (NONE or KMS)"`
	KMSKeyID           string   `json:"kms_key_id" metadata:"kms_key_id" description:"KMS key used for server-side encryption"`
	EnhancedMonitoring []string `json:"enhanced_monitoring" metadata:"enhanced_monitoring" description:"Shard-level CloudWatch metrics that are enabled"`
	ConsumerCount      int      `json:"consumer_count" metadata:"consumer_count" description:"Number of registered enhanced fan-out consumers"`
	Consumers          []string `json:"consumers" metadata:"consumers" description:"Names of the registered enhanced fan-out consumers"`
	CreatedAt          string   `json:"created_at" metadata:"created_at" description:"Stream creation time (RFC 3339)"`
	Region             string   `json:"region" metadata:"region" description:"AWS region the stream lives in"`
	URL                string   `json:"url" metadata:"url" description:"Link to the stream in the AWS console"`
}

// KinesisSampleFields describes the columns of a stream's data preview.
type KinesisSampleFields struct {
	SequenceNumber string `json:"sequence_number" metadata:"sequence_number" description:"Record sequence number within its shard"`
	PartitionKey   string `json:"partition_key" metadata:"partition_key" description:"Partition key the producer wrote the record with"`
	ArrivalTime    string `json:"arrival_time" metadata:"arrival_time" description:"Approximate time the record reached the stream (RFC 3339)"`
	Data           string `json:"data" metadata:"data" description:"Record payload as text, or base64 when it is not valid UTF-8"`
}
