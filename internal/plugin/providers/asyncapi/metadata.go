// metadata.go
package asyncapi

// SharedFields represents common metadata fields across all resources
// +marmot:metadata
type SharedFields struct {
	ServiceName    string `json:"service_name" description:"Name of the service that owns the resource"`
	ServiceVersion string `json:"service_version" description:"Version of the service"`
	Environment    string `json:"environment" description:"Environment the resource belongs to"`
	Description    string `json:"description" description:"Description of the resource"`
}

// KafkaFields represents Kafka-specific metadata fields
// +marmot:metadata
type KafkaFields struct {
	ClusterId         string   `json:"cluster_id" description:"Kafka cluster ID"`
	Partitions        int      `json:"partitions" description:"Number of partitions"`
	Replicas          int      `json:"replicas" description:"Number of replicas"`
	CleanupPolicies   []string `json:"cleanup_policy" description:"Topic cleanup policies"`
	RetentionMs       int64    `json:"retention_ms" description:"Message retention period in milliseconds"`
	RetentionBytes    int64    `json:"retention_bytes" description:"Maximum size of the topic"`
	MaxMessageBytes   int      `json:"max_message_bytes" description:"Maximum message size"`
	DeleteRetentionMs int64    `json:"delete_retention_ms" description:"Time to retain deleted messages"`
}

// SNSFields represents SNS-specific metadata fields
// +marmot:metadata
type SNSFields struct {
	TopicArn             string `json:"topic_arn" description:"SNS Topic Name/ARN"`
	OrderingType         string `json:"ordering_type" description:"SNS topic ordering type"`
	ContentDeduplication bool   `json:"content_deduplication" description:"Whether content-based deduplication is enabled"`
}

// SQSFields represents SQS-specific metadata fields
// +marmot:metadata
type SQSFields struct {
	Name                   string `json:"name" description:"Name of the SQS queue"`
	FifoQueue              bool   `json:"fifo_queue" description:"Whether this is a FIFO queue"`
	DeduplicationScope     string `json:"deduplication_scope" description:"Scope of deduplication if enabled"`
	FifoThroughputLimit    string `json:"fifo_throughput_limit" description:"FIFO throughput limit type"`
	DeliveryDelay          int    `json:"delivery_delay" description:"Delivery delay in seconds"`
	VisibilityTimeout      int    `json:"visibility_timeout" description:"Visibility timeout in seconds"`
	ReceiveMessageWaitTime int    `json:"receive_message_wait_time" description:"Long polling wait time in seconds"`
	MessageRetentionPeriod int    `json:"message_retention_period" description:"Message retention period in seconds"`
	DLQName                string `json:"dlq_name" description:"Name of the Dead Letter Queue"`
	MaxReceiveCount        int    `json:"max_receive_count" description:"Maximum receives before sending to DLQ"`
}
