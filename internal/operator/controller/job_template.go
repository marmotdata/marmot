package controller

import (
	"encoding/json"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runsv1alpha1 "github.com/marmotdata/marmot/internal/operator/api/v1alpha1"
)

// OperatorConfig holds operator-level settings passed via flags.
type OperatorConfig struct {
	MarmotURL            string
	IngestServiceAccount string
	Image                string
}

// buildConfigMap renders the Run spec into a CLI-compatible pipeline YAML ConfigMap.
func buildConfigMap(run *runsv1alpha1.Run) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(run),
			Namespace: run.Namespace,
			Labels:    commonLabels(run),
		},
		Data: map[string]string{
			"pipeline.yaml": renderPipelineYAML(run),
		},
	}
}

// buildCronJob creates a CronJob from the Run spec.
func buildCronJob(run *runsv1alpha1.Run, cfg OperatorConfig) *batchv1.CronJob {
	suspend := false
	if run.Spec.Suspend != nil {
		suspend = *run.Spec.Suspend
	}

	successLimit := int32(1)
	if run.Spec.SuccessfulJobsHistoryLimit != nil {
		successLimit = *run.Spec.SuccessfulJobsHistoryLimit
	}
	failedLimit := int32(1)
	if run.Spec.FailedJobsHistoryLimit != nil {
		failedLimit = *run.Spec.FailedJobsHistoryLimit
	}

	jobTemplateLabels := commonLabels(run)
	for k, v := range run.Spec.PodLabels {
		jobTemplateLabels[k] = v
	}

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.Name,
			Namespace: run.Namespace,
			Labels:    commonLabels(run),
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   run.Spec.Schedule,
			Suspend:                    &suspend,
			ConcurrencyPolicy:          run.Spec.ConcurrencyPolicy,
			SuccessfulJobsHistoryLimit: &successLimit,
			FailedJobsHistoryLimit:     &failedLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      jobTemplateLabels,
					Annotations: run.Spec.PodAnnotations,
				},
				Spec: buildJobSpec(run, cfg),
			},
		},
	}
	return cj
}

// buildJob creates a one-shot Job from the Run spec.
func buildJob(run *runsv1alpha1.Run, cfg OperatorConfig) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.Name,
			Namespace: run.Namespace,
			Labels:    commonLabels(run),
		},
		Spec: buildJobSpec(run, cfg),
	}
}

// buildTriggerJob creates a one-shot Job for a manually triggered run.
func buildTriggerJob(run *runsv1alpha1.Run, cfg OperatorConfig) *batchv1.Job {
	spec := buildJobSpec(run, cfg)
	ttl := int32(60)
	spec.TTLSecondsAfterFinished = &ttl
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-trigger-%d", run.Name, time.Now().Unix()),
			Namespace: run.Namespace,
			Labels:    commonLabels(run),
		},
		Spec: spec,
	}
}

// buildTeardownJob creates a one-shot Job that runs `marmot ingest --destroy`.
func buildTeardownJob(run *runsv1alpha1.Run, cfg OperatorConfig) *batchv1.Job {
	spec := buildJobSpec(run, cfg)
	// Override command to include --destroy and auto-confirm
	spec.Template.Spec.Containers[0].Command = []string{
		"sh", "-c",
		fmt.Sprintf("echo y | marmot ingest -c /config/pipeline.yaml --destroy --host $MARMOT_HOST"),
	}
	ttl := int32(60)
	spec.TTLSecondsAfterFinished = &ttl
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-teardown", run.Name),
			Namespace: run.Namespace,
			Labels:    commonLabels(run),
		},
		Spec: spec,
	}
}

func buildJobSpec(run *runsv1alpha1.Run, cfg OperatorConfig) batchv1.JobSpec {
	resources := corev1.ResourceRequirements{}
	if run.Spec.Resources != nil {
		resources = *run.Spec.Resources
	}

	podLabels := commonLabels(run)
	for k, v := range run.Spec.PodLabels {
		podLabels[k] = v
	}

	spec := batchv1.JobSpec{
		BackoffLimit:          run.Spec.BackoffLimit,
		ActiveDeadlineSeconds: run.Spec.ActiveDeadlineSeconds,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      podLabels,
				Annotations: run.Spec.PodAnnotations,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: cfg.IngestServiceAccount,
				RestartPolicy:      corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:    "marmot-ingest",
						Image:   cfg.Image,
						Command: []string{"marmot", "ingest", "-c", "/config/pipeline.yaml", "--host", "$(MARMOT_HOST)"},
						Env: []corev1.EnvVar{
							{
								Name:  "MARMOT_HOST",
								Value: cfg.MarmotURL,
							},
						},
						Resources: resources,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config",
								MountPath: "/config",
								ReadOnly:  true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMapName(run),
								},
							},
						},
					},
				},
			},
		},
	}
	return spec
}

// renderPipelineYAML produces the CLI-compatible pipeline YAML from the CRD spec.
func renderPipelineYAML(run *runsv1alpha1.Run) string {
	// Unmarshal each apiextensionsv1.JSON entry back to map[string]interface{}
	// so we can produce CLI-compatible YAML.
	runs := make([]map[string]interface{}, 0, len(run.Spec.Runs))
	for _, raw := range run.Spec.Runs {
		var entry map[string]interface{}
		if err := json.Unmarshal(raw.Raw, &entry); err != nil {
			continue
		}
		runs = append(runs, entry)
	}

	type pipelineConfig struct {
		Name string                   `json:"name"`
		Runs []map[string]interface{} `json:"runs"`
	}
	cfg := pipelineConfig{
		Name: run.Name,
		Runs: runs,
	}

	data, err := yamlMarshal(cfg)
	if err != nil {
		return fmt.Sprintf("name: %s\nruns: []\n", run.Name)
	}
	return string(data)
}

func configMapName(run *runsv1alpha1.Run) string {
	return fmt.Sprintf("%s-config", run.Name)
}

func commonLabels(run *runsv1alpha1.Run) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "marmot-operator",
		"runs.marmotdata.io/run":       run.Name,
	}
}
