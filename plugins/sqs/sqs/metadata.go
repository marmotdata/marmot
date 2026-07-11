package sqs

// SQSFields represents SQS-specific metadata fields
// +marmot:metadata
type SQSFields struct {
	QueueArn                  string            `json:"queue_arn" metadata:"queue_arn" description:"The ARN of the SQS queue"`
	VisibilityTimeout         string            `json:"visibility_timeout" metadata:"visibility_timeout" description:"The visibility timeout for the queue"`
	MessageRetentionPeriod    string            `json:"message_retention_period" metadata:"message_retention_period" description:"Message retention period in seconds"`
	MaximumMessageSize        string            `json:"maximum_message_size" metadata:"maximum_message_size" description:"Maximum message size in bytes"`
	DelaySeconds              string            `json:"delay_seconds" metadata:"delay_seconds" description:"Delay seconds for messages"`
	ReceiveMessageWaitTime    string            `json:"receive_message_wait_time" metadata:"receive_message_wait_time" description:"Long polling wait time in seconds"`
	FifoQueue                 bool              `json:"fifo_queue" metadata:"fifo_queue" description:"Whether this is a FIFO queue"`
	ContentBasedDeduplication bool              `json:"content_based_deduplication" metadata:"content_based_deduplication" description:"Whether content-based deduplication is enabled"`
	DeduplicationScope        string            `json:"deduplication_scope" metadata:"deduplication_scope" description:"Deduplication scope for FIFO queues"`
	FifoThroughputLimit       string            `json:"fifo_throughput_limit" metadata:"fifo_throughput_limit" description:"FIFO throughput limit type"`
	RedrivePolicy             string            `json:"redrive_policy" metadata:"redrive_policy" description:"Redrive policy JSON string"`
	Tags                      map[string]string `json:"tags" metadata:"tags" description:"AWS resource tags"`
}
