package asyncapi

// SharedFields represents common metadata fields across all resources
// +marmot:metadata
type SharedFields struct {
	ServiceName    string `json:"service_name" metadata:"service_name" description:"Name of the service that owns the resource"`
	ServiceVersion string `json:"service_version" metadata:"service_version" description:"Version of the service"`
	Environment    string `json:"environment" metadata:"environment" description:"Environment the resource belongs to"`
	Description    string `json:"description" metadata:"description" description:"Description of the resource"`
}

// KafkaFields represents Kafka-specific metadata fields
// +marmot:metadata
type KafkaFields struct {
	ClusterId         string   `json:"cluster_id" metadata:"cluster_id" description:"Kafka cluster ID"`
	Partitions        int      `json:"partitions" metadata:"partitions" description:"Number of partitions"`
	Replicas          int      `json:"replicas" metadata:"replicas" description:"Number of replicas"`
	CleanupPolicies   []string `json:"cleanup_policy" metadata:"cleanup_policy" description:"Topic cleanup policies"`
	RetentionMs       int64    `json:"retention_ms" metadata:"retention_ms" description:"Message retention period in milliseconds"`
	RetentionBytes    int64    `json:"retention_bytes" metadata:"retention_bytes" description:"Maximum size of the topic"`
	MaxMessageBytes   int      `json:"max_message_bytes" metadata:"max_message_bytes" description:"Maximum message size"`
	DeleteRetentionMs int64    `json:"delete_retention_ms" metadata:"delete_retention_ms" description:"Time to retain deleted messages"`
}

// SNSFields represents SNS-specific metadata fields
// +marmot:metadata
type SNSFields struct {
	TopicArn             string `json:"topic_arn" metadata:"topic_arn" description:"SNS Topic Name/ARN"`
	OrderingType         string `json:"ordering_type" metadata:"ordering_type" description:"SNS topic ordering type"`
	ContentDeduplication bool   `json:"content_deduplication" metadata:"content_deduplication" description:"Whether content-based deduplication is enabled"`
}

// SQSFields represents SQS-specific metadata fields
// +marmot:metadata
type SQSFields struct {
	Name                   string `json:"name" metadata:"name" description:"Name of the SQS queue"`
	FifoQueue              bool   `json:"fifo_queue" metadata:"fifo_queue" description:"Whether this is a FIFO queue"`
	DeduplicationScope     string `json:"deduplication_scope" metadata:"deduplication_scope" description:"Scope of deduplication if enabled"`
	FifoThroughputLimit    string `json:"fifo_throughput_limit" metadata:"fifo_throughput_limit" description:"FIFO throughput limit type"`
	DeliveryDelay          int    `json:"delivery_delay" metadata:"delivery_delay" description:"Delivery delay in seconds"`
	VisibilityTimeout      int    `json:"visibility_timeout" metadata:"visibility_timeout" description:"Visibility timeout in seconds"`
	ReceiveMessageWaitTime int    `json:"receive_message_wait_time" metadata:"receive_message_wait_time" description:"Long polling wait time in seconds"`
	MessageRetentionPeriod int    `json:"message_retention_period" metadata:"message_retention_period" description:"Message retention period in seconds"`
	DLQName                string `json:"dlq_name" metadata:"dlq_name" description:"Name of the Dead Letter Queue"`
	MaxReceiveCount        int    `json:"max_receive_count" metadata:"max_receive_count" description:"Maximum receives before sending to DLQ"`
}

// AMQPFields represents AMQP-specific metadata fields
// +marmot:metadata
type AMQPFields struct {
	BindingIs          string `json:"binding_is" metadata:"binding_is" description:"AMQP binding type (queue or routingKey)"`
	ExchangeName       string `json:"exchange_name" metadata:"exchange_name" description:"Exchange name"`
	ExchangeType       string `json:"exchange_type" metadata:"exchange_type" description:"Exchange type (topic, fanout, direct, etc.)"`
	ExchangeDurable    bool   `json:"exchange_durable" metadata:"exchange_durable" description:"Exchange durability flag"`
	ExchangeAutoDelete bool   `json:"exchange_auto_delete" metadata:"exchange_auto_delete" description:"Exchange auto delete flag"`
	QueueName          string `json:"queue_name" metadata:"queue_name" description:"Queue name"`
	QueueVHost         string `json:"queue_vhost" metadata:"queue_vhost" description:"Queue virtual host"`
	QueueDurable       bool   `json:"queue_durable" metadata:"queue_durable" description:"Queue durability flag"`
	QueueExclusive     bool   `json:"queue_exclusive" metadata:"queue_exclusive" description:"Queue exclusivity flag"`
	QueueAutoDelete    bool   `json:"queue_auto_delete" metadata:"queue_auto_delete" description:"Queue auto delete flag"`
}
