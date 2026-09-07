package kafkaconnect

// KafkaConnectPipelineFields describes the metadata fields the plugin
// emits for a connector's Pipeline asset. It is kept as a
// documentation-only struct so downstream tooling can introspect the
// shape of the metadata map.
type KafkaConnectPipelineFields struct {
	ConnectorClass string            `json:"connector_class" metadata:"connector_class" description:"Java class of the connector"`
	ConnectorType  string            `json:"connector_type" metadata:"connector_type" description:"Connector direction (source, sink)"`
	State          string            `json:"state" metadata:"state" description:"Connector state (RUNNING, PAUSED, FAILED, UNASSIGNED)"`
	WorkerID       string            `json:"worker_id" metadata:"worker_id" description:"Worker the connector instance runs on"`
	TaskCount      int               `json:"task_count" metadata:"task_count" description:"Number of tasks the connector is split into"`
	TaskStates     map[string]string `json:"task_states" metadata:"task_states" description:"State of each task, keyed by task id"`
	Topics         []string          `json:"topics" metadata:"topics" description:"Topics the connector reads or writes"`
	PluginVersion  string            `json:"plugin_version" metadata:"plugin_version" description:"Version of the installed connector plugin"`
	ConnectVersion string            `json:"connect_version" metadata:"connect_version" description:"Version of the Connect worker"`
	KafkaClusterID string            `json:"kafka_cluster_id" metadata:"kafka_cluster_id" description:"Id of the Kafka cluster the worker is attached to"`
	Config         map[string]string `json:"config" metadata:"config" description:"Connector config with credential values masked"`
	Description    string            `json:"description" metadata:"description" description:"Description from the connector config, when set"`
	Error          string            `json:"error" metadata:"error" description:"First 500 characters of the failure trace, when the connector or a task has failed"`
	URL            string            `json:"url" metadata:"url" description:"REST URL of the connector"`
}

// KafkaConnectTaskFields describes the metadata fields the plugin emits
// for a Task asset.
type KafkaConnectTaskFields struct {
	TaskID    int    `json:"task_id" metadata:"task_id" description:"Task id within the connector"`
	State     string `json:"state" metadata:"state" description:"Task state (RUNNING, PAUSED, FAILED, UNASSIGNED)"`
	WorkerID  string `json:"worker_id" metadata:"worker_id" description:"Worker the task runs on"`
	Connector string `json:"connector" metadata:"connector" description:"Name of the connector the task belongs to"`
	Error     string `json:"error" metadata:"error" description:"First 500 characters of the failure trace, when the task has failed"`
}

// KafkaConnectTopicFields describes the metadata fields the plugin emits
// for a Topic asset.
type KafkaConnectTopicFields struct {
	TopicName string   `json:"topic_name" metadata:"topic_name" description:"Topic name"`
	Producers []string `json:"producers" metadata:"producers" description:"Source connectors writing to the topic"`
	Consumers []string `json:"consumers" metadata:"consumers" description:"Sink connectors reading from the topic"`
}
