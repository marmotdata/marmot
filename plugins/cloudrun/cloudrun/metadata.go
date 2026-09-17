package cloudrun

// ServiceFields describes the metadata the Cloud Run plugin emits for a
// Service asset. It is a documentation-only struct so downstream tooling can
// introspect the shape of the metadata map.
type ServiceFields struct {
	UID                           string   `json:"uid" metadata:"uid" description:"Server-assigned unique identifier for the service"`
	Generation                    int64    `json:"generation" metadata:"generation" description:"Number of times the service configuration has changed"`
	Location                      string   `json:"location" metadata:"location" description:"Region the service runs in"`
	ProjectID                     string   `json:"project_id" metadata:"project_id" description:"Google Cloud project the service belongs to"`
	URI                           string   `json:"uri" metadata:"uri" description:"HTTPS endpoint the service is served on"`
	Ingress                       string   `json:"ingress" metadata:"ingress" description:"Which traffic is allowed to reach the service"`
	LaunchStage                   string   `json:"launch_stage" metadata:"launch_stage" description:"Google Cloud launch stage of the features the service uses"`
	Creator                       string   `json:"creator" metadata:"creator" description:"Principal that created the service"`
	LastModifier                  string   `json:"last_modifier" metadata:"last_modifier" description:"Principal that last modified the service"`
	CreateTime                    string   `json:"create_time" metadata:"create_time" description:"Creation timestamp"`
	UpdateTime                    string   `json:"update_time" metadata:"update_time" description:"Last update timestamp"`
	LatestReadyRevision           string   `json:"latest_ready_revision" metadata:"latest_ready_revision" description:"Id of the most recent revision that became ready"`
	LatestCreatedRevision         string   `json:"latest_created_revision" metadata:"latest_created_revision" description:"Id of the most recently created revision"`
	Traffic                       string   `json:"traffic" metadata:"traffic" description:"Traffic split across revisions, for example latest=100"`
	Ready                         string   `json:"ready" metadata:"ready" description:"State of the service terminal condition"`
	Reconciling                   bool     `json:"reconciling" metadata:"reconciling" description:"Whether the service is still converging on its desired state"`
	ExecutionEnvironment          string   `json:"execution_environment" metadata:"execution_environment" description:"Sandbox generation the containers run in"`
	ServiceAccount                string   `json:"service_account" metadata:"service_account" description:"Service account the containers run as"`
	Timeout                       string   `json:"timeout" metadata:"timeout" description:"Maximum duration of a single request"`
	MaxInstanceRequestConcurrency int64    `json:"max_instance_request_concurrency" metadata:"max_instance_request_concurrency" description:"Concurrent requests one instance accepts"`
	MinInstanceCount              int64    `json:"min_instance_count" metadata:"min_instance_count" description:"Minimum number of instances kept running"`
	MaxInstanceCount              int64    `json:"max_instance_count" metadata:"max_instance_count" description:"Maximum number of instances the service scales to"`
	ContainerImage                string   `json:"container_image" metadata:"container_image" description:"Image of the first container"`
	ContainerImages               []string `json:"container_images" metadata:"container_images" description:"Images of every container in the revision"`
	ContainerPorts                []int64  `json:"container_ports" metadata:"container_ports" description:"Ports the containers listen on"`
	EnvVarNames                   []string `json:"env_var_names" metadata:"env_var_names" description:"Names of the container environment variables. Values are never recorded"`
	VPCConnector                  string   `json:"vpc_connector" metadata:"vpc_connector" description:"Serverless VPC Access connector the service uses"`
	VPCEgress                     string   `json:"vpc_egress" metadata:"vpc_egress" description:"Which outbound traffic is routed through the VPC"`
	GCSVolumeBuckets              []string `json:"gcs_volume_buckets" metadata:"gcs_volume_buckets" description:"Cloud Storage buckets mounted as volumes"`
	CloudSQLInstances             []string `json:"cloud_sql_instances" metadata:"cloud_sql_instances" description:"Cloud SQL instances mounted as volumes, as project:region:instance"`
	SecretVolumes                 []string `json:"secret_volumes" metadata:"secret_volumes" description:"Names of the Secret Manager secrets mounted as volumes. Values are never recorded"`
	NFSVolumes                    []string `json:"nfs_volumes" metadata:"nfs_volumes" description:"NFS mounts, as server:path"`
}

// JobFields describes the metadata the Cloud Run plugin emits for a Job asset.
type JobFields struct {
	UID                    string   `json:"uid" metadata:"uid" description:"Server-assigned unique identifier for the job"`
	Generation             int64    `json:"generation" metadata:"generation" description:"Number of times the job configuration has changed"`
	Location               string   `json:"location" metadata:"location" description:"Region the job runs in"`
	ProjectID              string   `json:"project_id" metadata:"project_id" description:"Google Cloud project the job belongs to"`
	Creator                string   `json:"creator" metadata:"creator" description:"Principal that created the job"`
	LastModifier           string   `json:"last_modifier" metadata:"last_modifier" description:"Principal that last modified the job"`
	CreateTime             string   `json:"create_time" metadata:"create_time" description:"Creation timestamp"`
	UpdateTime             string   `json:"update_time" metadata:"update_time" description:"Last update timestamp"`
	LaunchStage            string   `json:"launch_stage" metadata:"launch_stage" description:"Google Cloud launch stage of the features the job uses"`
	ExecutionCount         int64    `json:"execution_count" metadata:"execution_count" description:"Number of executions created for the job"`
	LatestCreatedExecution string   `json:"latest_created_execution" metadata:"latest_created_execution" description:"Id of the most recently created execution"`
	Reconciling            bool     `json:"reconciling" metadata:"reconciling" description:"Whether the job is still converging on its desired state"`
	TaskCount              int64    `json:"task_count" metadata:"task_count" description:"Number of tasks one execution runs"`
	Parallelism            int64    `json:"parallelism" metadata:"parallelism" description:"How many tasks may run at the same time"`
	MaxRetries             int64    `json:"max_retries" metadata:"max_retries" description:"Retries allowed per failed task"`
	ExecutionEnvironment   string   `json:"execution_environment" metadata:"execution_environment" description:"Sandbox generation the tasks run in"`
	ServiceAccount         string   `json:"service_account" metadata:"service_account" description:"Service account the tasks run as"`
	Timeout                string   `json:"timeout" metadata:"timeout" description:"Maximum duration of a single task"`
	ContainerImage         string   `json:"container_image" metadata:"container_image" description:"Image of the first container"`
	ContainerImages        []string `json:"container_images" metadata:"container_images" description:"Images of every container in the task"`
	ContainerPorts         []int64  `json:"container_ports" metadata:"container_ports" description:"Ports the containers listen on"`
	EnvVarNames            []string `json:"env_var_names" metadata:"env_var_names" description:"Names of the container environment variables. Values are never recorded"`
	VPCConnector           string   `json:"vpc_connector" metadata:"vpc_connector" description:"Serverless VPC Access connector the job uses"`
	VPCEgress              string   `json:"vpc_egress" metadata:"vpc_egress" description:"Which outbound traffic is routed through the VPC"`
	GCSVolumeBuckets       []string `json:"gcs_volume_buckets" metadata:"gcs_volume_buckets" description:"Cloud Storage buckets mounted as volumes"`
	CloudSQLInstances      []string `json:"cloud_sql_instances" metadata:"cloud_sql_instances" description:"Cloud SQL instances mounted as volumes, as project:region:instance"`
	SecretVolumes          []string `json:"secret_volumes" metadata:"secret_volumes" description:"Names of the Secret Manager secrets mounted as volumes. Values are never recorded"`
	NFSVolumes             []string `json:"nfs_volumes" metadata:"nfs_volumes" description:"NFS mounts, as server:path"`
}

// RunFacetFields describes the facets attached to each job run event.
type RunFacetFields struct {
	TaskCount      int64  `json:"task_count" metadata:"task_count" description:"Tasks the execution was asked to run"`
	SucceededCount int64  `json:"succeeded_count" metadata:"succeeded_count" description:"Tasks that succeeded"`
	FailedCount    int64  `json:"failed_count" metadata:"failed_count" description:"Tasks that failed"`
	CancelledCount int64  `json:"cancelled_count" metadata:"cancelled_count" description:"Tasks that were cancelled"`
	RetriedCount   int64  `json:"retried_count" metadata:"retried_count" description:"Tasks that were retried"`
	LogURI         string `json:"log_uri" metadata:"log_uri" description:"Cloud Logging link for the execution"`
}
