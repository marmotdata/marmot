package pubsub

// TopicFields describes the metadata fields the plugin emits for topic
// assets. It is kept as a documentation-only struct so downstream tooling can
// introspect the shape of the metadata map.
type TopicFields struct {
	ProjectID                 string            `json:"project_id" metadata:"project_id" description:"Google Cloud project the topic belongs to"`
	TopicName                 string            `json:"topic_name" metadata:"topic_name" description:"Full resource name, projects/{project}/topics/{topic}"`
	SubscriptionCount         int               `json:"subscription_count" metadata:"subscription_count" description:"Number of subscriptions attached to the topic"`
	Subscriptions             []string          `json:"subscriptions" metadata:"subscriptions" description:"Ids of the subscriptions attached to the topic"`
	Labels                    map[string]string `json:"labels" metadata:"labels" description:"Labels set on the topic"`
	KMSKeyName                string            `json:"kms_key_name" metadata:"kms_key_name" description:"Cloud KMS key protecting published messages"`
	Retention                 string            `json:"retention" metadata:"retention" description:"How long published messages stay available to subscribers"`
	AllowedPersistenceRegions []string          `json:"allowed_persistence_regions" metadata:"allowed_persistence_regions" description:"Regions the message storage policy allows"`
	State                     string            `json:"state" metadata:"state" description:"Topic state (ACTIVE, INGESTION_RESOURCE_ERROR)"`
	Schema                    string            `json:"schema" metadata:"schema" description:"Id of the schema published messages are validated against"`
	SchemaType                string            `json:"schema_type" metadata:"schema_type" description:"Schema type (AVRO, PROTOCOL_BUFFER)"`
	SchemaEncoding            string            `json:"schema_encoding" metadata:"schema_encoding" description:"Message encoding the schema is applied to (JSON, BINARY)"`
	SchemaRevision            string            `json:"schema_revision" metadata:"schema_revision" description:"Revision id of the schema"`
	IngestionSource           string            `json:"ingestion_source" metadata:"ingestion_source" description:"External system Pub/Sub imports messages from (aws_kinesis, cloud_storage, azure_event_hubs, amazon_msk, confluent_cloud)"`
	URL                       string            `json:"url" metadata:"url" description:"Link to the topic in the Google Cloud console"`
}

// SubscriptionFields describes the metadata fields the plugin emits for
// subscription assets.
type SubscriptionFields struct {
	ProjectID                  string            `json:"project_id" metadata:"project_id" description:"Google Cloud project the subscription belongs to"`
	SubscriptionName           string            `json:"subscription_name" metadata:"subscription_name" description:"Full resource name, projects/{project}/subscriptions/{subscription}"`
	Topic                      string            `json:"topic" metadata:"topic" description:"Id of the topic the subscription reads"`
	DeliveryType               string            `json:"delivery_type" metadata:"delivery_type" description:"How messages are delivered (pull, push, bigquery, cloud_storage)"`
	PushEndpoint               string            `json:"push_endpoint" metadata:"push_endpoint" description:"URL messages are pushed to"`
	BigQueryTable              string            `json:"bigquery_table" metadata:"bigquery_table" description:"BigQuery table messages are written to"`
	BigQueryUseTopicSchema     bool              `json:"bigquery_use_topic_schema" metadata:"bigquery_use_topic_schema" description:"Whether the topic's schema is used to write the BigQuery rows"`
	BigQueryState              string            `json:"bigquery_state" metadata:"bigquery_state" description:"Whether the BigQuery export is working (ACTIVE when it is)"`
	CloudStorageBucket         string            `json:"cloud_storage_bucket" metadata:"cloud_storage_bucket" description:"Cloud Storage bucket messages are written to"`
	CloudStorageFilenamePrefix string            `json:"cloud_storage_filename_prefix" metadata:"cloud_storage_filename_prefix" description:"Prefix of the objects written to the bucket"`
	CloudStorageFilenameSuffix string            `json:"cloud_storage_filename_suffix" metadata:"cloud_storage_filename_suffix" description:"Suffix of the objects written to the bucket"`
	CloudStorageState          string            `json:"cloud_storage_state" metadata:"cloud_storage_state" description:"Whether the Cloud Storage export is working (ACTIVE when it is)"`
	AckDeadlineSeconds         int               `json:"ack_deadline_seconds" metadata:"ack_deadline_seconds" description:"Seconds a subscriber has to acknowledge a message"`
	MessageRetention           string            `json:"message_retention" metadata:"message_retention" description:"How long unacknowledged messages are kept"`
	RetainAckedMessages        bool              `json:"retain_acked_messages" metadata:"retain_acked_messages" description:"Whether acknowledged messages are kept for replay"`
	ExpirationTTL              string            `json:"expiration_ttl" metadata:"expiration_ttl" description:"How long the subscription can be inactive before it is deleted"`
	Filter                     string            `json:"filter" metadata:"filter" description:"Expression selecting which messages are delivered"`
	EnableMessageOrdering      bool              `json:"enable_message_ordering" metadata:"enable_message_ordering" description:"Whether messages with the same ordering key are delivered in order"`
	ExactlyOnceDelivery        bool              `json:"exactly_once_delivery" metadata:"exactly_once_delivery" description:"Whether exactly once delivery is enabled"`
	DeadLetterTopic            string            `json:"dead_letter_topic" metadata:"dead_letter_topic" description:"Id of the topic undeliverable messages are forwarded to"`
	MaxDeliveryAttempts        int               `json:"max_delivery_attempts" metadata:"max_delivery_attempts" description:"Deliveries attempted before a message goes to the dead letter topic"`
	Labels                     map[string]string `json:"labels" metadata:"labels" description:"Labels set on the subscription"`
	State                      string            `json:"state" metadata:"state" description:"Subscription state (ACTIVE, RESOURCE_ERROR)"`
	Detached                   bool              `json:"detached" metadata:"detached" description:"Whether the subscription is detached from its topic and no longer receives messages"`
	URL                        string            `json:"url" metadata:"url" description:"Link to the subscription in the Google Cloud console"`
}

// TopicColumnFields describes the per-field entries the plugin derives from a
// topic's Avro schema. Only the top level of the record is expanded.
type TopicColumnFields struct {
	ColumnName  string `json:"column_name" metadata:"column_name" description:"Avro field name"`
	DataType    string `json:"data_type" metadata:"data_type" description:"Avro field type, unions joined with a pipe"`
	IsNullable  bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether the field's union includes null"`
	Description string `json:"description" metadata:"description" description:"The Avro field's doc string"`
}
