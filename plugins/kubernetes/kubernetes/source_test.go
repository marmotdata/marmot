package kubernetes

import (
	"context"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func discover(t *testing.T, config pluginsdk.RawConfig, objects ...runtime.Object) *pluginsdk.DiscoveryResult {
	t.Helper()
	s := &Source{client: fake.NewClientset(objects...)}
	result, err := s.Discover(context.Background(), config)
	require.NoError(t, err)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func TestDiscoverer_WithMetadataStampsEveryAsset(t *testing.T) {
	client := fake.NewClientset(
		namespaceFixture("payments"),
		serviceFixture("payments", "api", nil),
	)
	d := NewDiscoverer(client, &DiscoveryConfig{
		ClusterName:        "prod",
		DiscoverNamespaces: true,
		DiscoverServices:   true,
	}).WithMetadata(map[string]interface{}{"cloud": "GKE", "gcp_project": "my-project"})

	result, err := d.Discover(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		assert.Equal(t, "GKE", a.Metadata["cloud"], "asset %s missing cloud", *a.Name)
		assert.Equal(t, "my-project", a.Metadata["gcp_project"], "asset %s missing project", *a.Name)
	}
}

func TestDiscoverer_ClusterExternalLinks(t *testing.T) {
	client := fake.NewClientset(namespaceFixture("payments"))
	links := []pluginsdk.AssetExternalLink{{Name: "Google Cloud console", URL: "https://console.cloud.google.com/x"}}
	d := NewDiscoverer(client, &DiscoveryConfig{ClusterName: "prod", DiscoverNamespaces: true}).
		WithClusterExternalLinks(links)

	result, err := d.Discover(context.Background())
	require.NoError(t, err)

	cluster := findAsset(result, "Cluster", "prod")
	require.NotNil(t, cluster)
	require.Len(t, cluster.ExternalLinks, 1)
	assert.Equal(t, "https://console.cloud.google.com/x", cluster.ExternalLinks[0].URL)

	// Only the cluster asset carries the link, not the namespaces.
	ns := findAsset(result, "Namespace", "prod/payments")
	require.NotNil(t, ns)
	assert.Empty(t, ns.ExternalLinks)
}

func TestDiscoverer_ClusterMetadataOnlyOnCluster(t *testing.T) {
	client := fake.NewClientset(namespaceFixture("payments"))
	d := NewDiscoverer(client, &DiscoveryConfig{ClusterName: "prod", DiscoverNamespaces: true}).
		WithClusterMetadata(map[string]interface{}{"cluster_arn": "arn:aws:eks:eu-west-1:123456789012:cluster/prod"})

	result, err := d.Discover(context.Background())
	require.NoError(t, err)

	cluster := findAsset(result, "Cluster", "prod")
	require.NotNil(t, cluster)
	assert.Equal(t, "arn:aws:eks:eu-west-1:123456789012:cluster/prod", cluster.Metadata["cluster_arn"])

	// The ARN is cluster-scoped, not stamped on other assets.
	ns := findAsset(result, "Namespace", "prod/payments")
	require.NotNil(t, ns)
	assert.NotContains(t, ns.Metadata, "cluster_arn")
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func namespaceFixture(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
}

func deploymentFixture(namespace, name string, podLabels map[string]string) *appsv1.Deployment {
	replicas := int32(2)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: podLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "example/api:1.2.3"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 2, AvailableReplicas: 2, UpdatedReplicas: 2},
	}
}

func serviceFixture(namespace, name string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.1",
			Selector:  selector,
			Ports:     []corev1.ServicePort{{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP}},
		},
	}
}

func replicaSetFixture(namespace, name, deploymentName string) *appsv1.ReplicaSet {
	controller := true
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				Kind:       "Deployment",
				Name:       deploymentName,
				Controller: &controller,
			}},
		},
	}
}

func podFixture(namespace, name, replicaSetName string) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				Kind:       "ReplicaSet",
				Name:       replicaSetName,
				Controller: &controller,
			}},
		},
		Spec: corev1.PodSpec{
			NodeName:   "node-1",
			Containers: []corev1.Container{{Name: "app", Image: "example/api:1.2.3"}},
		},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			QOSClass:          corev1.PodQOSBurstable,
			ContainerStatuses: []corev1.ContainerStatus{{RestartCount: 3}},
		},
	}
}

func statefulSetFixture(namespace, name string, podLabels map[string]string) *appsv1.StatefulSet {
	replicas := int32(3)
	storageClass := "fast-ssd"
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: podLabels},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: name + "-headless",
			Selector:    &metav1.LabelSelector{MatchLabels: podLabels},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "db", Image: "postgres:16"}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClass,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("100Gi"),
						},
					},
				},
			}},
		},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 3, UpdatedReplicas: 3},
	}
}

func cronJobFixture(namespace, name string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: batchv1.CronJobSpec{
			Schedule:          "0 2 * * *",
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "etl", Image: "example/etl:2.0"}},
						},
					},
				},
			},
		},
	}
}

func jobFixture(namespace, name, cronJobName string, start, completion *metav1.Time, failed bool) *batchv1.Job {
	controller := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				Kind:       "CronJob",
				Name:       cronJobName,
				Controller: &controller,
			}},
		},
		Status: batchv1.JobStatus{
			StartTime:      start,
			CompletionTime: completion,
		},
	}
	if failed {
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:               batchv1.JobFailed,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Time{Time: start.Add(5 * time.Minute)},
		}}
	}
	return job
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)

	assert.True(t, s.config.DiscoverNamespaces)
	assert.True(t, s.config.DiscoverServices)
	assert.True(t, s.config.DiscoverDeployments)
	assert.True(t, s.config.DiscoverStatefulSets)
	assert.True(t, s.config.DiscoverCronJobs)
	assert.False(t, s.config.DiscoverPods)
	assert.True(t, s.config.LabelsToMetadata)
	assert.False(t, s.config.AnnotationsToMetadata)
	assert.Equal(t, []string{"kube-system", "kube-public", "kube-node-lease"}, s.config.ExcludeNamespaces)
}

func TestValidate_InvalidLabelSelector(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"label_selector": "not a valid selector!!"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "label_selector")
}

func TestValidate_NothingToDiscover(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"discover_namespaces":   false,
		"discover_services":     false,
		"discover_deployments":  false,
		"discover_statefulsets": false,
		"discover_cronjobs":     false,
		"discover_pods":         false,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to discover")
}

func TestValidate_DirectConnection(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":  "https://mycluster.example.com:6443",
		"token": "sa-token",
	})
	require.NoError(t, err)
}

func TestValidate_HostRequiresToken(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "https://mycluster.example.com:6443"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestValidate_TokenRequiresHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"token": "sa-token"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestValidate_HostConflictsWithKubeconfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":            "https://mycluster.example.com:6443",
		"token":           "sa-token",
		"kubeconfig_path": "/etc/kubeconfig",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "separate authentication methods")
}

func TestBuildRestConfig_DirectConnection(t *testing.T) {
	restConfig, err := buildRestConfig(&Config{
		Host:          "https://mycluster.example.com:6443",
		Token:         "sa-token",
		CACertificate: "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----",
	})
	require.NoError(t, err)

	assert.Equal(t, "https://mycluster.example.com:6443", restConfig.Host)
	assert.Equal(t, "sa-token", restConfig.BearerToken)
	assert.Contains(t, string(restConfig.TLSClientConfig.CAData), "BEGIN CERTIFICATE")
}

func TestBuildRestConfig_NoAuthMethodGivesClearError(t *testing.T) {
	// An explicit but missing kubeconfig path forces the loader to fail
	// regardless of the test host's environment.
	_, err := buildRestConfig(&Config{KubeconfigPath: "/nonexistent/marmot-kubeconfig"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host and token")
	assert.Contains(t, err.Error(), "kubeconfig_path")
}

func TestBuildRestConfig_DirectConnectionWithoutCA(t *testing.T) {
	restConfig, err := buildRestConfig(&Config{
		Host:  "https://mycluster.example.com:6443",
		Token: "sa-token",
	})
	require.NoError(t, err)
	assert.Empty(t, restConfig.TLSClientConfig.CAData)
}

func TestDiscover_NamespaceAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{}, namespaceFixture("payments"))

	ns := findAsset(result, "Namespace", "payments")
	require.NotNil(t, ns)
	assert.Equal(t, "Active", ns.Metadata["phase"])
	assert.Equal(t, []string{"Kubernetes"}, ns.Providers)
}

func TestDiscover_ServiceAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("payments"),
		serviceFixture("payments", "api", map[string]string{"app": "api"}),
	)

	svc := findAsset(result, "Service", "payments/api")
	require.NotNil(t, svc)
	assert.Equal(t, "payments", svc.Metadata["namespace"])
	assert.Equal(t, "ClusterIP", svc.Metadata["service_type"])
	assert.Equal(t, "10.0.0.1", svc.Metadata["cluster_ip"])
	assert.Equal(t, "http:80/TCP", svc.Metadata["ports"])
	assert.Equal(t, "app=api", svc.Metadata["selector"])
}

func TestDiscover_DeploymentAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("payments"),
		deploymentFixture("payments", "api", map[string]string{"app": "api"}),
	)

	d := findAsset(result, "Deployment", "payments/api")
	require.NotNil(t, d)
	assert.Equal(t, int32(2), d.Metadata["replicas"])
	assert.Equal(t, int32(2), d.Metadata["ready_replicas"])
	assert.Equal(t, "RollingUpdate", d.Metadata["strategy"])
	assert.Equal(t, "example/api:1.2.3", d.Metadata["images"])
}

func TestDiscover_PodsRequireOptIn(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("payments"),
		podFixture("payments", "api-abc123", "api-abc"),
	)

	assert.Nil(t, findAsset(result, "Pod", "payments/api-abc123"))
}

func TestDiscover_PodAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"discover_pods": true},
		namespaceFixture("payments"),
		podFixture("payments", "api-abc123", "api-abc"),
	)

	pod := findAsset(result, "Pod", "payments/api-abc123")
	require.NotNil(t, pod)
	assert.Equal(t, "Running", pod.Metadata["phase"])
	assert.Equal(t, "node-1", pod.Metadata["node"])
	assert.Equal(t, int32(3), pod.Metadata["restart_count"])
	assert.Equal(t, "ReplicaSet", pod.Metadata["owner_kind"])
	assert.Equal(t, "api-abc", pod.Metadata["owner_name"])
}

func TestDiscover_ExcludesSystemNamespacesByDefault(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("payments"),
		namespaceFixture("kube-system"),
	)

	assert.NotNil(t, findAsset(result, "Namespace", "payments"))
	assert.Nil(t, findAsset(result, "Namespace", "kube-system"))
}

func TestDiscover_ScopedToConfiguredNamespaces(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"namespaces": []string{"payments"}},
		namespaceFixture("payments"),
		namespaceFixture("orders"),
		serviceFixture("payments", "api", nil),
		serviceFixture("orders", "api", nil),
	)

	assert.NotNil(t, findAsset(result, "Service", "payments/api"))
	assert.Nil(t, findAsset(result, "Service", "orders/api"))
	assert.Nil(t, findAsset(result, "Namespace", "orders"))
}

func TestDiscover_WildcardNamespaceMeansAll(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"namespaces": []string{"*"}},
		namespaceFixture("payments"),
		namespaceFixture("orders"),
		namespaceFixture("kube-system"),
	)

	assert.NotNil(t, findAsset(result, "Namespace", "payments"))
	assert.NotNil(t, findAsset(result, "Namespace", "orders"))
	assert.Nil(t, findAsset(result, "Namespace", "kube-system"), "wildcard still applies exclude_namespaces")
}

func TestDiscover_NamespaceContainsLineage(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("payments"),
		serviceFixture("payments", "api", nil),
	)

	nsMRN := mrn.New("Namespace", "Kubernetes", "payments")
	svcMRN := mrn.New("Service", "Kubernetes", "payments/api")
	assert.True(t, hasEdge(result, nsMRN, svcMRN, "CONTAINS"))
}

func TestDiscover_NoNamespaceEdgesWhenNamespacesDisabled(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"discover_namespaces": false},
		namespaceFixture("payments"),
		serviceFixture("payments", "api", nil),
	)

	assert.Nil(t, findAsset(result, "Namespace", "payments"))
	for _, e := range result.Lineage {
		assert.NotEqual(t, mrn.New("Namespace", "Kubernetes", "payments"), e.Source)
	}
}

func TestDiscover_ServiceExposesDeploymentLineage(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("payments"),
		serviceFixture("payments", "api", map[string]string{"app": "api"}),
		deploymentFixture("payments", "api", map[string]string{"app": "api"}),
		deploymentFixture("payments", "worker", map[string]string{"app": "worker"}),
	)

	svcMRN := mrn.New("Service", "Kubernetes", "payments/api")
	assert.True(t, hasEdge(result, svcMRN, mrn.New("Deployment", "Kubernetes", "payments/api"), "EXPOSES"))
	assert.False(t, hasEdge(result, svcMRN, mrn.New("Deployment", "Kubernetes", "payments/worker"), "EXPOSES"))
}

func TestDiscover_DeploymentContainsPodLineage(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"discover_pods": true},
		namespaceFixture("payments"),
		deploymentFixture("payments", "api", map[string]string{"app": "api"}),
		replicaSetFixture("payments", "api-abc", "api"),
		podFixture("payments", "api-abc123", "api-abc"),
	)

	deploymentMRN := mrn.New("Deployment", "Kubernetes", "payments/api")
	podMRN := mrn.New("Pod", "Kubernetes", "payments/api-abc123")
	assert.True(t, hasEdge(result, deploymentMRN, podMRN, "CONTAINS"))
}

func TestDiscover_ClusterNamePrefixesAssetNames(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"cluster_name": "prod"},
		namespaceFixture("payments"),
		serviceFixture("payments", "api", nil),
	)

	assert.NotNil(t, findAsset(result, "Namespace", "prod/payments"))
	svc := findAsset(result, "Service", "prod/payments/api")
	require.NotNil(t, svc)
	assert.Equal(t, "prod", svc.Metadata["cluster"])
}

func TestDiscover_TagsInterpolateLabels(t *testing.T) {
	d := deploymentFixture("payments", "api", map[string]string{"team": "data-platform"})

	result := discover(t, pluginsdk.RawConfig{"tags": []string{"kubernetes", "${labels.team}"}},
		namespaceFixture("payments"), d,
	)

	deployment := findAsset(result, "Deployment", "payments/api")
	require.NotNil(t, deployment)
	assert.Contains(t, deployment.Tags, "kubernetes")
	assert.Contains(t, deployment.Tags, "data-platform")
}

func TestDiscover_StatefulSetAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("data"),
		statefulSetFixture("data", "postgres", map[string]string{"app": "postgres"}),
	)

	sts := findAsset(result, "StatefulSet", "data/postgres")
	require.NotNil(t, sts)
	assert.Equal(t, int32(3), sts.Metadata["replicas"])
	assert.Equal(t, int32(3), sts.Metadata["ready_replicas"])
	assert.Equal(t, "postgres:16", sts.Metadata["images"])
	assert.Equal(t, "postgres-headless", sts.Metadata["headless_service"])
	assert.Equal(t, "data:100Gi/fast-ssd", sts.Metadata["volume_claims"])
}

func TestDiscover_ServiceExposesStatefulSetLineage(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("data"),
		serviceFixture("data", "postgres", map[string]string{"app": "postgres"}),
		statefulSetFixture("data", "postgres", map[string]string{"app": "postgres"}),
	)

	svcMRN := mrn.New("Service", "Kubernetes", "data/postgres")
	stsMRN := mrn.New("StatefulSet", "Kubernetes", "data/postgres")
	assert.True(t, hasEdge(result, svcMRN, stsMRN, "EXPOSES"))
}

func TestDiscover_StatefulSetContainsPodLineage(t *testing.T) {
	pod := podFixture("data", "postgres-0", "")
	controller := true
	pod.OwnerReferences = []metav1.OwnerReference{{
		Kind:       "StatefulSet",
		Name:       "postgres",
		Controller: &controller,
	}}

	result := discover(t, pluginsdk.RawConfig{"discover_pods": true},
		namespaceFixture("data"),
		statefulSetFixture("data", "postgres", map[string]string{"app": "postgres"}),
		pod,
	)

	stsMRN := mrn.New("StatefulSet", "Kubernetes", "data/postgres")
	podMRN := mrn.New("Pod", "Kubernetes", "data/postgres-0")
	assert.True(t, hasEdge(result, stsMRN, podMRN, "CONTAINS"))
}

func TestDiscover_CronJobAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("data"),
		cronJobFixture("data", "nightly-etl"),
	)

	cj := findAsset(result, "CronJob", "data/nightly-etl")
	require.NotNil(t, cj)
	assert.Equal(t, "0 2 * * *", cj.Metadata["schedule"])
	assert.Equal(t, "Forbid", cj.Metadata["concurrency_policy"])
	assert.Equal(t, "example/etl:2.0", cj.Metadata["images"])
}

func TestDiscover_CronJobRunHistoryFromJobs(t *testing.T) {
	start := metav1.Time{Time: time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC)}
	completion := metav1.Time{Time: start.Add(10 * time.Minute)}
	failedStart := metav1.Time{Time: start.Add(24 * time.Hour)}

	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("data"),
		cronJobFixture("data", "nightly-etl"),
		jobFixture("data", "nightly-etl-1", "nightly-etl", &start, &completion, false),
		jobFixture("data", "nightly-etl-2", "nightly-etl", &failedStart, nil, true),
	)

	require.Len(t, result.RunHistory, 1)
	history := result.RunHistory[0]
	assert.Equal(t, mrn.New("CronJob", "Kubernetes", "data/nightly-etl"), history.AssetMRN)

	eventTypes := map[string][]string{}
	for _, event := range history.Runs {
		assert.Equal(t, "nightly-etl", event.JobName)
		eventTypes[event.RunID] = append(eventTypes[event.RunID], event.EventType)
	}
	assert.ElementsMatch(t, []string{"START", "COMPLETE"}, eventTypes["nightly-etl-1"])
	assert.ElementsMatch(t, []string{"START", "FAIL"}, eventTypes["nightly-etl-2"])
}

func TestDiscover_OneOffJobsIgnored(t *testing.T) {
	start := metav1.Time{Time: time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC)}
	oneOff := jobFixture("data", "manual-migration", "unused", &start, nil, false)
	oneOff.OwnerReferences = nil

	result := discover(t, pluginsdk.RawConfig{},
		namespaceFixture("data"),
		cronJobFixture("data", "nightly-etl"),
		oneOff,
	)

	assert.Nil(t, findAsset(result, "Job", "data/manual-migration"))
	assert.Empty(t, result.RunHistory)
}

func TestDiscover_ClusterAsset(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{"cluster_name": "prod"},
		namespaceFixture("payments"),
	)

	cluster := findAsset(result, "Cluster", "prod")
	require.NotNil(t, cluster)
	assert.Equal(t, "prod", cluster.Metadata["cluster"])

	clusterMRN := mrn.New("Cluster", "Kubernetes", "prod")
	nsMRN := mrn.New("Namespace", "Kubernetes", "prod/payments")
	assert.True(t, hasEdge(result, clusterMRN, nsMRN, "CONTAINS"))
}

func TestDiscover_NoClusterAssetWithoutClusterName(t *testing.T) {
	result := discover(t, pluginsdk.RawConfig{}, namespaceFixture("payments"))

	for _, a := range result.Assets {
		assert.NotEqual(t, "Cluster", a.Type)
	}
}

func TestDiscover_AnnotationsExcludeLastApplied(t *testing.T) {
	ns := namespaceFixture("payments")
	ns.Annotations = map[string]string{
		"team":                "payments",
		lastAppliedAnnotation: `{"huge":"manifest"}`,
	}

	result := discover(t, pluginsdk.RawConfig{"annotations_to_metadata": true}, ns)

	asset := findAsset(result, "Namespace", "payments")
	require.NotNil(t, asset)
	annotations, ok := asset.Metadata["annotations"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "payments", annotations["team"])
	assert.NotContains(t, annotations, lastAppliedAnnotation)
}
