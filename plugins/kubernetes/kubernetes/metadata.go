package kubernetes

// ClusterFields represents Kubernetes cluster metadata fields
// +marmot:metadata
type ClusterFields struct {
	Cluster           string `json:"cluster" metadata:"cluster" description:"Configured cluster name"`
	KubernetesVersion string `json:"kubernetes_version" metadata:"kubernetes_version" description:"Kubernetes server version"`
	Platform          string `json:"platform" metadata:"platform" description:"Server platform (e.g. linux/amd64)"`
}

// NamespaceFields represents Kubernetes namespace metadata fields
// +marmot:metadata
type NamespaceFields struct {
	Namespace   string            `json:"namespace" metadata:"namespace" description:"Namespace name"`
	Phase       string            `json:"phase" metadata:"phase" description:"Namespace lifecycle phase (Active, Terminating)"`
	Cluster     string            `json:"cluster" metadata:"cluster" description:"Configured cluster name"`
	CreatedAt   string            `json:"created_at" metadata:"created_at" description:"Resource creation timestamp"`
	Labels      map[string]string `json:"labels" metadata:"labels" description:"Resource labels"`
	Annotations map[string]string `json:"annotations" metadata:"annotations" description:"Resource annotations"`
}

// ServiceFields represents Kubernetes service metadata fields
// +marmot:metadata
type ServiceFields struct {
	Namespace         string `json:"namespace" metadata:"namespace" description:"Namespace the service lives in"`
	ServiceType       string `json:"service_type" metadata:"service_type" description:"Service type (ClusterIP, NodePort, LoadBalancer, ExternalName)"`
	ClusterIP         string `json:"cluster_ip" metadata:"cluster_ip" description:"Cluster IP address (None for headless services)"`
	ExternalName      string `json:"external_name" metadata:"external_name" description:"External DNS name for ExternalName services"`
	Ports             string `json:"ports" metadata:"ports" description:"Exposed ports (name:port/protocol, comma-separated)"`
	Selector          string `json:"selector" metadata:"selector" description:"Pod selector labels (key=value, comma-separated)"`
	LoadBalancerHosts string `json:"load_balancer_hosts" metadata:"load_balancer_hosts" description:"Load balancer ingress hostnames and IPs"`
}

// DeploymentFields represents Kubernetes deployment metadata fields
// +marmot:metadata
type DeploymentFields struct {
	Namespace         string `json:"namespace" metadata:"namespace" description:"Namespace the deployment lives in"`
	Replicas          int32  `json:"replicas" metadata:"replicas" description:"Desired replica count"`
	ReadyReplicas     int32  `json:"ready_replicas" metadata:"ready_replicas" description:"Number of ready replicas"`
	AvailableReplicas int32  `json:"available_replicas" metadata:"available_replicas" description:"Number of available replicas"`
	UpdatedReplicas   int32  `json:"updated_replicas" metadata:"updated_replicas" description:"Number of replicas updated to the latest pod template"`
	Strategy          string `json:"strategy" metadata:"strategy" description:"Rollout strategy (RollingUpdate, Recreate)"`
	Paused            bool   `json:"paused" metadata:"paused" description:"Whether rollouts are paused"`
	Images            string `json:"images" metadata:"images" description:"Container images (comma-separated)"`
	ContainerCount    int    `json:"container_count" metadata:"container_count" description:"Number of containers in the pod template"`
	ServiceAccount    string `json:"service_account" metadata:"service_account" description:"Service account the pods run as"`
}

// StatefulSetFields represents Kubernetes stateful set metadata fields
// +marmot:metadata
type StatefulSetFields struct {
	Namespace       string `json:"namespace" metadata:"namespace" description:"Namespace the stateful set lives in"`
	Replicas        int32  `json:"replicas" metadata:"replicas" description:"Desired replica count"`
	ReadyReplicas   int32  `json:"ready_replicas" metadata:"ready_replicas" description:"Number of ready replicas"`
	UpdatedReplicas int32  `json:"updated_replicas" metadata:"updated_replicas" description:"Number of replicas updated to the latest pod template"`
	Strategy        string `json:"strategy" metadata:"strategy" description:"Update strategy (RollingUpdate, OnDelete)"`
	HeadlessService string `json:"headless_service" metadata:"headless_service" description:"Headless service governing the stateful set"`
	Images          string `json:"images" metadata:"images" description:"Container images (comma-separated)"`
	ContainerCount  int    `json:"container_count" metadata:"container_count" description:"Number of containers in the pod template"`
	ServiceAccount  string `json:"service_account" metadata:"service_account" description:"Service account the pods run as"`
	VolumeClaims    string `json:"volume_claims" metadata:"volume_claims" description:"Volume claim templates (name:size/storageClass, comma-separated)"`
}

// CronJobFields represents Kubernetes cron job metadata fields
// +marmot:metadata
type CronJobFields struct {
	Namespace          string `json:"namespace" metadata:"namespace" description:"Namespace the cron job lives in"`
	Schedule           string `json:"schedule" metadata:"schedule" description:"Cron schedule expression"`
	Timezone           string `json:"timezone" metadata:"timezone" description:"Time zone the schedule is evaluated in"`
	Suspended          bool   `json:"suspended" metadata:"suspended" description:"Whether the cron job is suspended"`
	ConcurrencyPolicy  string `json:"concurrency_policy" metadata:"concurrency_policy" description:"Concurrency policy (Allow, Forbid, Replace)"`
	Images             string `json:"images" metadata:"images" description:"Container images (comma-separated)"`
	LastScheduleTime   string `json:"last_schedule_time" metadata:"last_schedule_time" description:"When the cron job last fired"`
	LastSuccessfulTime string `json:"last_successful_time" metadata:"last_successful_time" description:"When the cron job last completed successfully"`
}

// PodFields represents Kubernetes pod metadata fields
// +marmot:metadata
type PodFields struct {
	Namespace      string `json:"namespace" metadata:"namespace" description:"Namespace the pod lives in"`
	Phase          string `json:"phase" metadata:"phase" description:"Pod lifecycle phase (Pending, Running, Succeeded, Failed)"`
	Node           string `json:"node" metadata:"node" description:"Node the pod is scheduled on"`
	Images         string `json:"images" metadata:"images" description:"Container images (comma-separated)"`
	QOSClass       string `json:"qos_class" metadata:"qos_class" description:"Quality of service class (Guaranteed, Burstable, BestEffort)"`
	ServiceAccount string `json:"service_account" metadata:"service_account" description:"Service account the pod runs as"`
	RestartCount   int32  `json:"restart_count" metadata:"restart_count" description:"Total container restarts"`
	OwnerKind      string `json:"owner_kind" metadata:"owner_kind" description:"Kind of the controlling owner (ReplicaSet, StatefulSet, DaemonSet, Job)"`
	OwnerName      string `json:"owner_name" metadata:"owner_name" description:"Name of the controlling owner"`
}
