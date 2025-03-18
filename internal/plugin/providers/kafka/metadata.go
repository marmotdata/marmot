package kafka

// KafkaTopicFields represents Kafka topic-specific metadata fields
// +marmot:metadata
type KafkaTopicFields struct {
	TopicName          string `json:"topic_name" metadata:"topic_name" description:"Name of the Kafka topic"`
	PartitionCount     int32  `json:"partition_count" metadata:"partition_count" description:"Number of partitions"`
	ReplicationFactor  int16  `json:"replication_factor" metadata:"replication_factor" description:"Replication factor"`
	RetentionMs        string `json:"retention_ms" metadata:"retention.ms" description:"Message retention period in milliseconds"`
	RetentionBytes     string `json:"retention_bytes" metadata:"retention.bytes" description:"Maximum size of the topic in bytes"`
	CleanupPolicy      string `json:"cleanup_policy" metadata:"cleanup.policy" description:"Topic cleanup policy"`
	MinInsyncReplicas  string `json:"min_insync_replicas" metadata:"min.insync.replicas" description:"Minimum number of in-sync replicas"`
	MaxMessageBytes    string `json:"max_message_bytes" metadata:"max.message.bytes" description:"Maximum message size in bytes"`
	SegmentBytes       string `json:"segment_bytes" metadata:"segment.bytes" description:"Segment file size in bytes"`
	SegmentMs          string `json:"segment_ms" metadata:"segment.ms" description:"Segment file roll time in milliseconds"`
	DeleteRetentionMs  string `json:"delete_retention_ms" metadata:"delete.retention.ms" description:"Time to retain deleted segments in milliseconds"`
	ValueSchemaId      int    `json:"value_schema_id" metadata:"value_schema_id" description:"ID of the value schema in Schema Registry"`
	ValueSchemaVersion int    `json:"value_schema_version" metadata:"value_schema_version" description:"Version of the value schema"`
	ValueSchemaType    string `json:"value_schema_type" metadata:"value_schema_type" description:"Type of the value schema (AVRO, JSON, etc.)"`
	ValueSchema        string `json:"value_schema" metadata:"value_schema" description:"Value schema definition"`
	KeySchemaId        int    `json:"key_schema_id" metadata:"key_schema_id" description:"ID of the key schema in Schema Registry"`
	KeySchemaVersion   int    `json:"key_schema_version" metadata:"key_schema_version" description:"Version of the key schema"`
	KeySchemaType      string `json:"key_schema_type" metadata:"key_schema_type" description:"Type of the key schema (AVRO, JSON, etc.)"`
	KeySchema          string `json:"key_schema" metadata:"key_schema" description:"Key schema definition"`
}

// KafkaConsumerGroupFields represents Kafka consumer group-specific metadata fields
// +marmot:metadata
type KafkaConsumerGroupFields struct {
	GroupId          string   `json:"group_id" metadata:"group_id" description:"Consumer group ID"`
	State            string   `json:"state" metadata:"state" description:"Current state of the consumer group"`
	Protocol         string   `json:"protocol" metadata:"protocol" description:"Rebalance protocol"`
	ProtocolType     string   `json:"protocol_type" metadata:"protocol_type" description:"Protocol type"`
	SubscribedTopics []string `json:"subscribed_topics" metadata:"subscribed_topics" description:"Topics the group is subscribed to"`
	Members          []string `json:"members" metadata:"members" description:"Members of the consumer group"`
}
