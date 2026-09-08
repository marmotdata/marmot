package nifi

// NiFiPipelineFields describes the metadata fields the NiFi plugin
// emits for process group (Pipeline) assets. It is kept as a
// documentation-only struct so downstream tooling can introspect the
// shape of the metadata map.
type NiFiPipelineFields struct {
	ID               string `json:"id" metadata:"id" description:"Process group id"`
	ParentID         string `json:"parent_id" metadata:"parent_id" description:"Id of the parent process group"`
	Path             string `json:"path" metadata:"path" description:"Group names from the root, joined with /"`
	Comments         string `json:"comments" metadata:"comments" description:"Comments set on the process group"`
	ParameterContext string `json:"parameter_context" metadata:"parameter_context" description:"Name of the bound parameter context"`
	RunningCount     int    `json:"running_count" metadata:"running_count" description:"Components running in the group and its children"`
	StoppedCount     int    `json:"stopped_count" metadata:"stopped_count" description:"Components stopped in the group and its children"`
	InvalidCount     int    `json:"invalid_count" metadata:"invalid_count" description:"Components that cannot start because their configuration is invalid"`
	DisabledCount    int    `json:"disabled_count" metadata:"disabled_count" description:"Components disabled in the group and its children"`
	ProcessorCount   int    `json:"processor_count" metadata:"processor_count" description:"Processors directly in the group"`
	ConnectionCount  int    `json:"connection_count" metadata:"connection_count" description:"Connections directly in the group"`
	InputPortCount   int    `json:"input_port_count" metadata:"input_port_count" description:"Input ports of the group"`
	OutputPortCount  int    `json:"output_port_count" metadata:"output_port_count" description:"Output ports of the group"`
	NiFiVersion      string `json:"nifi_version" metadata:"nifi_version" description:"NiFi release the instance runs"`
	URL              string `json:"url" metadata:"url" description:"Link to the group in the NiFi UI"`
}

// NiFiTaskFields describes the metadata fields emitted for processor
// (Task) assets.
type NiFiTaskFields struct {
	ID                 string            `json:"id" metadata:"id" description:"Processor id"`
	GroupID            string            `json:"group_id" metadata:"group_id" description:"Id of the process group holding the processor"`
	Pipeline           string            `json:"pipeline" metadata:"pipeline" description:"Path of the process group holding the processor"`
	Type               string            `json:"type" metadata:"type" description:"Processor class name (e.g. PutS3Object)"`
	TypeFull           string            `json:"type_full" metadata:"type_full" description:"Fully qualified processor class name"`
	State              string            `json:"state" metadata:"state" description:"RUNNING, STOPPED, DISABLED or INVALID"`
	SchedulingPeriod   string            `json:"scheduling_period" metadata:"scheduling_period" description:"How often the processor is scheduled"`
	SchedulingStrategy string            `json:"scheduling_strategy" metadata:"scheduling_strategy" description:"TIMER_DRIVEN or CRON_DRIVEN"`
	Comments           string            `json:"comments" metadata:"comments" description:"Comments set on the processor"`
	Properties         map[string]string `json:"properties" metadata:"properties" description:"Configured properties, with sensitive values masked"`
	Relationships      []string          `json:"relationships" metadata:"relationships" description:"Names of the processor's relationships"`
	URL                string            `json:"url" metadata:"url" description:"Link to the processor in the NiFi UI"`
}

// NiFiPortFields describes the extra metadata emitted for input and
// output ports when include_ports is on. Ports are Task assets and
// share id, group_id, pipeline, state, comments and url with processors.
type NiFiPortFields struct {
	PortType string `json:"port_type" metadata:"port_type" description:"INPUT_PORT or OUTPUT_PORT"`
}

// NiFiTopicFields describes the metadata emitted for the Kafka Topic
// assets created from PublishKafka and ConsumeKafka processors.
type NiFiTopicFields struct {
	TopicName string   `json:"topic_name" metadata:"topic_name" description:"Kafka topic name"`
	Producers []string `json:"producers" metadata:"producers" description:"Tasks that publish to the topic"`
	Consumers []string `json:"consumers" metadata:"consumers" description:"Tasks that consume from the topic"`
}
