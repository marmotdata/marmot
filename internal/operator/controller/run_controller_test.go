package controller

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	runsv1alpha1 "github.com/marmotdata/marmot/internal/operator/api/v1alpha1"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = runsv1alpha1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	return s
}

func testConfig() OperatorConfig {
	return OperatorConfig{
		MarmotURL:            "http://marmot:8080",
		IngestServiceAccount: "marmot-ingest",
		Image:                "ghcr.io/marmotdata/marmot:v0.8.5",
	}
}

func mustJSON(v interface{}) apiextensionsv1.JSON {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return apiextensionsv1.JSON{Raw: raw}
}

func newTestRun(name, namespace string, schedule string) *runsv1alpha1.Run {
	run := &runsv1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: runsv1alpha1.RunSpec{
			Name: name,
			Runs: []apiextensionsv1.JSON{
				mustJSON(map[string]interface{}{
					"postgresql": map[string]interface{}{
						"host":     "db.example.com",
						"port":     float64(5432),
						"database": "mydb",
						"user":     "readonly",
					},
				}),
			},
			Schedule: schedule,
		},
	}
	return run
}

func TestBuildConfigMap(t *testing.T) {
	run := newTestRun("my-pipeline", "default", "")

	cm := buildConfigMap(run)

	assert.Equal(t, "my-pipeline-config", cm.Name)
	assert.Equal(t, "default", cm.Namespace)
	assert.Contains(t, cm.Data["pipeline.yaml"], "name: my-pipeline")
	assert.Contains(t, cm.Data["pipeline.yaml"], "postgresql")
	assert.Equal(t, "marmot-operator", cm.Labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-pipeline", cm.Labels["runs.marmotdata.io/run"])
}

func TestBuildCronJob(t *testing.T) {
	run := newTestRun("scheduled-pipeline", "default", "0 */6 * * *")
	suspend := false
	run.Spec.Suspend = &suspend
	run.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	backoff := int32(5)
	run.Spec.BackoffLimit = &backoff
	successLimit := int32(3)
	run.Spec.SuccessfulJobsHistoryLimit = &successLimit
	failedLimit := int32(1)
	run.Spec.FailedJobsHistoryLimit = &failedLimit

	cfg := testConfig()
	cj := buildCronJob(run, cfg)

	assert.Equal(t, "scheduled-pipeline", cj.Name)
	assert.Equal(t, "0 */6 * * *", cj.Spec.Schedule)
	assert.Equal(t, false, *cj.Spec.Suspend)
	assert.Equal(t, batchv1.ForbidConcurrent, cj.Spec.ConcurrencyPolicy)
	assert.Equal(t, int32(3), *cj.Spec.SuccessfulJobsHistoryLimit)
	assert.Equal(t, int32(1), *cj.Spec.FailedJobsHistoryLimit)

	// Check job template
	container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "marmot-ingest", container.Name)
	assert.Equal(t, cfg.Image, container.Image)
	assert.Contains(t, container.Command, "marmot")
	assert.Equal(t, int32(5), *cj.Spec.JobTemplate.Spec.BackoffLimit)

	// Check service account
	assert.Equal(t, "marmot-ingest", cj.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName)

	// Check env vars (only MARMOT_HOST, no MARMOT_API_KEY)
	require.Len(t, container.Env, 1)
	assert.Equal(t, "MARMOT_HOST", container.Env[0].Name)
	assert.Equal(t, "http://marmot:8080", container.Env[0].Value)

	// Check volume mount
	assert.Len(t, container.VolumeMounts, 1)
	assert.Equal(t, "/config", container.VolumeMounts[0].MountPath)
}

func TestBuildJob(t *testing.T) {
	run := newTestRun("oneshot-pipeline", "test-ns", "")
	backoff := int32(3)
	run.Spec.BackoffLimit = &backoff
	deadline := int64(3600)
	run.Spec.ActiveDeadlineSeconds = &deadline
	run.Spec.Resources = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}

	cfg := testConfig()
	job := buildJob(run, cfg)

	assert.Equal(t, "oneshot-pipeline", job.Name)
	assert.Equal(t, "test-ns", job.Namespace)
	assert.Equal(t, int32(3), *job.Spec.BackoffLimit)
	assert.Equal(t, int64(3600), *job.Spec.ActiveDeadlineSeconds)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, resource.MustParse("2"), container.Resources.Limits[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("2Gi"), container.Resources.Limits[corev1.ResourceMemory])
}

func TestBuildTeardownJob(t *testing.T) {
	run := newTestRun("my-pipeline", "default", "")

	cfg := testConfig()
	job := buildTeardownJob(run, cfg)

	assert.Equal(t, "my-pipeline-teardown", job.Name)
	container := job.Spec.Template.Spec.Containers[0]
	assert.Contains(t, container.Command[2], "--destroy")
}

func TestRenderPipelineYAML(t *testing.T) {
	run := newTestRun("my-pipeline", "default", "")

	yaml := renderPipelineYAML(run)

	assert.Contains(t, yaml, "name: my-pipeline")
	assert.Contains(t, yaml, "runs:")
	assert.Contains(t, yaml, "postgresql")
	assert.Contains(t, yaml, "host: db.example.com")
}

func TestReconcile_CreatesConfigMapAndCronJob(t *testing.T) {
	scheme := testScheme()
	run := newTestRun("scheduled-run", "default", "0 * * * *")
	run.SetResourceVersion("1")

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(run).
		WithStatusSubresource(run).
		Build()

	reconciler := &RunReconciler{
		Client: k8sClient,
		Scheme: scheme,
		Config: testConfig(),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "scheduled-run", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0)

	// Verify ConfigMap was created
	var cm corev1.ConfigMap
	err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "scheduled-run-config", Namespace: "default"}, &cm)
	require.NoError(t, err)
	assert.Contains(t, cm.Data["pipeline.yaml"], "name: scheduled-run")

	// Verify CronJob was created
	var cj batchv1.CronJob
	err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "scheduled-run", Namespace: "default"}, &cj)
	require.NoError(t, err)
	assert.Equal(t, "0 * * * *", cj.Spec.Schedule)
}

func TestReconcile_CreatesConfigMapAndJob(t *testing.T) {
	scheme := testScheme()
	run := newTestRun("oneshot-run", "default", "")
	run.SetResourceVersion("1")

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(run).
		WithStatusSubresource(run).
		Build()

	reconciler := &RunReconciler{
		Client: k8sClient,
		Scheme: scheme,
		Config: testConfig(),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "oneshot-run", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0)

	// Verify ConfigMap was created
	var cm corev1.ConfigMap
	err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "oneshot-run-config", Namespace: "default"}, &cm)
	require.NoError(t, err)

	// Verify Job was created
	var job batchv1.Job
	err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "oneshot-run", Namespace: "default"}, &job)
	require.NoError(t, err)
}

func TestReconcile_NotFound(t *testing.T) {
	scheme := testScheme()

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &RunReconciler{
		Client: k8sClient,
		Scheme: scheme,
		Config: testConfig(),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestBuildJobWithPodLabelsAndAnnotations(t *testing.T) {
	run := newTestRun("labeled-pipeline", "default", "")
	run.Spec.PodLabels = map[string]string{
		"team":        "data-eng",
		"cost-center": "1234",
	}
	run.Spec.PodAnnotations = map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9090",
	}

	cfg := testConfig()
	job := buildJob(run, cfg)

	podLabels := job.Spec.Template.Labels
	assert.Equal(t, "marmot-operator", podLabels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "labeled-pipeline", podLabels["runs.marmotdata.io/run"])
	assert.Equal(t, "data-eng", podLabels["team"])
	assert.Equal(t, "1234", podLabels["cost-center"])

	podAnnotations := job.Spec.Template.Annotations
	assert.Equal(t, "true", podAnnotations["prometheus.io/scrape"])
	assert.Equal(t, "9090", podAnnotations["prometheus.io/port"])
}

func TestBuildCronJobWithPodLabelsAndAnnotations(t *testing.T) {
	run := newTestRun("scheduled-labeled", "default", "0 */6 * * *")
	run.Spec.PodLabels = map[string]string{
		"team": "data-eng",
	}
	run.Spec.PodAnnotations = map[string]string{
		"vault.hashicorp.com/agent-inject": "true",
	}

	cfg := testConfig()
	cj := buildCronJob(run, cfg)

	// Labels/annotations should appear on the JobTemplate ObjectMeta
	jtLabels := cj.Spec.JobTemplate.Labels
	assert.Equal(t, "data-eng", jtLabels["team"])
	assert.Equal(t, "marmot-operator", jtLabels["app.kubernetes.io/managed-by"])

	jtAnnotations := cj.Spec.JobTemplate.Annotations
	assert.Equal(t, "true", jtAnnotations["vault.hashicorp.com/agent-inject"])

	// And on the pod template
	podLabels := cj.Spec.JobTemplate.Spec.Template.Labels
	assert.Equal(t, "data-eng", podLabels["team"])
	assert.Equal(t, "marmot-operator", podLabels["app.kubernetes.io/managed-by"])

	podAnnotations := cj.Spec.JobTemplate.Spec.Template.Annotations
	assert.Equal(t, "true", podAnnotations["vault.hashicorp.com/agent-inject"])
}

func TestBuildJobWithoutPodLabelsAndAnnotations(t *testing.T) {
	run := newTestRun("plain-pipeline", "default", "")

	cfg := testConfig()
	job := buildJob(run, cfg)

	// Only common labels, no extra labels
	podLabels := job.Spec.Template.Labels
	assert.Equal(t, "marmot-operator", podLabels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "plain-pipeline", podLabels["runs.marmotdata.io/run"])
	assert.Len(t, podLabels, 2)

	// No annotations
	assert.Empty(t, job.Spec.Template.Annotations)
}

func TestCommonLabels(t *testing.T) {
	run := newTestRun("test", "default", "")
	labels := commonLabels(run)

	assert.Equal(t, "marmot-operator", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "test", labels["runs.marmotdata.io/run"])
}
