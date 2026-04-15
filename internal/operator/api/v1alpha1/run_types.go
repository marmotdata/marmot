package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunSpec defines the desired state of a Run.
type RunSpec struct {
	// Name is the pipeline name, identical to the CLI config "name" field.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Runs is the list of source configurations, identical to CLI YAML format.
	// Each entry maps a source name to its raw configuration.
	// +kubebuilder:validation:Required
	Runs []apiextensionsv1.JSON `json:"runs"`

	// Schedule is a cron expression. When set, a CronJob is created instead of a one-shot Job.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Suspend tells the controller to suspend subsequent executions.
	// Applies only when Schedule is set.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// ConcurrencyPolicy specifies how to treat concurrent executions of a Job.
	// Valid values are Allow, Forbid, Replace.
	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	// +kubebuilder:default=Forbid
	// +optional
	ConcurrencyPolicy batchv1.ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// BackoffLimit specifies the number of retries before marking a Job as failed.
	// +kubebuilder:default=3
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// ActiveDeadlineSeconds specifies the duration in seconds relative to the
	// startTime that the Job may be active before the system tries to terminate it.
	// +optional
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`

	// SuccessfulJobsHistoryLimit is the number of successful finished CronJobs to retain.
	// +kubebuilder:default=3
	// +optional
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// FailedJobsHistoryLimit is the number of failed finished CronJobs to retain.
	// +kubebuilder:default=1
	// +optional
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`

	// Resources overrides the container resource requirements for the ingestion Job pod.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// PodLabels are additional labels applied to the pod template of the Job or CronJob.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// PodAnnotations are additional annotations applied to the pod template of the Job or CronJob.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// TeardownOnDelete controls whether all assets created by this pipeline are
	// destroyed when the Run resource is deleted. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	TeardownOnDelete *bool `json:"teardownOnDelete,omitempty"`
}

// RunPhase describes the high-level state of a Run.
// +kubebuilder:validation:Enum=Idle;Scheduled;Running;Succeeded;Failed;Suspended
type RunPhase string

const (
	RunPhaseIdle      RunPhase = "Idle"
	RunPhaseScheduled RunPhase = "Scheduled"
	RunPhaseRunning   RunPhase = "Running"
	RunPhaseSucceeded RunPhase = "Succeeded"
	RunPhaseFailed    RunPhase = "Failed"
	RunPhaseSuspended RunPhase = "Suspended"
)

// RunStatus defines the observed state of a Run.
type RunStatus struct {
	// ObservedGeneration reflects the generation of the most recently observed Run spec.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the high-level summary of the Run state.
	// +optional
	Phase RunPhase `json:"phase,omitempty"`

	// LastRunTime is the last time a Job was started (scheduled or manually triggered).
	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// LastSuccessfulTime is the last time a Job completed successfully.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`

	// Active is the number of currently running Jobs.
	// +optional
	Active int32 `json:"active,omitempty"`

	// Conditions represent the latest available observations of the Run's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Last Run",type=date,JSONPath=`.status.lastRunTime`
// +kubebuilder:printcolumn:name="Teardown",type=boolean,JSONPath=`.spec.teardownOnDelete`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Run is the Schema for the runs API. It declares an ingestion pipeline
// that the operator reconciles into K8s Jobs or CronJobs running `marmot ingest`.
type Run struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RunSpec   `json:"spec,omitempty"`
	Status RunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RunList contains a list of Run.
type RunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Run `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Run{}, &RunList{})
}
