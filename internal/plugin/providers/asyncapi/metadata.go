package asyncapi

// SharedFields represents common metadata fields across all AsyncAPI resources
// +marmot:metadata
type SharedFields struct {
	AsyncAPIVersion string `json:"asyncapi_version" metadata:"asyncapi_version" description:"AsyncAPI specification version"`
	ServiceName     string `json:"service_name" metadata:"service_name" description:"Name of the service that owns the resource"`
	ServiceVersion  string `json:"service_version" metadata:"service_version" description:"Version of the service"`
	Environment     string `json:"environment" metadata:"environment" description:"Environment the resource belongs to"`
	ChannelName     string `json:"channel_name" metadata:"channel_name" description:"Name of the channel in the AsyncAPI spec"`
	ChannelAddress  string `json:"channel_address" metadata:"channel_address" description:"Address/topic of the channel"`
	Description     string `json:"description" metadata:"description" description:"Description of the resource"`
}

// ServiceFields represents AsyncAPI service-specific metadata
// +marmot:metadata
type ServiceFields struct {
	ContactName  string   `json:"contact_name" metadata:"contact_name" description:"Contact person name"`
	ContactEmail string   `json:"contact_email" metadata:"contact_email" description:"Contact email address"`
	ContactURL   string   `json:"contact_url" metadata:"contact_url" description:"Contact URL"`
	License      string   `json:"license" metadata:"license" description:"License name"`
	LicenseURL   string   `json:"license_url" metadata:"license_url" description:"License URL"`
	Servers      []string `json:"servers" metadata:"servers" description:"List of server names"`
	Protocols    []string `json:"protocols" metadata:"protocols" description:"List of protocols used"`
	ChannelCount int      `json:"channel_count" metadata:"channel_count" description:"Number of channels"`
	OperationCount int    `json:"operation_count" metadata:"operation_count" description:"Number of operations"`
}

// KafkaFields represents Kafka-specific metadata fields
// +marmot:metadata
type KafkaFields struct {
	TopicName         string   `json:"topic_name" metadata:"topic_name" description:"Kafka topic name"`
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
	TopicName            string `json:"topic_name" metadata:"topic_name" description:"SNS Topic Name"`
	TopicArn             string `json:"topic_arn" metadata:"topic_arn" description:"SNS Topic ARN"`
	OrderingType         string `json:"ordering_type" metadata:"ordering_type" description:"SNS topic ordering type"`
	ContentDeduplication bool   `json:"content_deduplication" metadata:"content_deduplication" description:"Whether content-based deduplication is enabled"`
}

// SQSFields represents SQS-specific metadata fields
// +marmot:metadata
type SQSFields struct {
	QueueName              string `json:"queue_name" metadata:"queue_name" description:"Name of the SQS queue"`
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
	ExchangeVHost      string `json:"exchange_vhost" metadata:"exchange_vhost" description:"Exchange virtual host"`
	ExchangeType       string `json:"exchange_type" metadata:"exchange_type" description:"Exchange type (topic, fanout, direct, etc.)"`
	ExchangeDurable    bool   `json:"exchange_durable" metadata:"exchange_durable" description:"Exchange durability flag"`
	ExchangeAutoDelete bool   `json:"exchange_auto_delete" metadata:"exchange_auto_delete" description:"Exchange auto delete flag"`
	QueueName          string `json:"queue_name" metadata:"queue_name" description:"Queue name"`
	QueueVHost         string `json:"queue_vhost" metadata:"queue_vhost" description:"Queue virtual host"`
	QueueDurable       bool   `json:"queue_durable" metadata:"queue_durable" description:"Queue durability flag"`
	QueueExclusive     bool   `json:"queue_exclusive" metadata:"queue_exclusive" description:"Queue exclusivity flag"`
	QueueAutoDelete    bool   `json:"queue_auto_delete" metadata:"queue_auto_delete" description:"Queue auto delete flag"`
}

// GooglePubSubFields represents Google Pub/Sub-specific metadata fields
// +marmot:metadata
type GooglePubSubFields struct {
	TopicName                string   `json:"topic_name" metadata:"topic_name" description:"Google Pub/Sub topic name"`
	MessageRetentionDuration string   `json:"message_retention_duration" metadata:"message_retention_duration" description:"Message retention duration"`
	AllowedRegions           []string `json:"allowed_regions" metadata:"allowed_regions" description:"Allowed persistence regions"`
	SchemaEncoding           string   `json:"schema_encoding" metadata:"schema_encoding" description:"Schema encoding format"`
	SchemaName               string   `json:"schema_name" metadata:"schema_name" description:"Schema name"`
}
