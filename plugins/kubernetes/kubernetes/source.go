// Package kubernetes discovers namespaces, services, workloads, and
// cron jobs from Kubernetes clusters. Its Discoverer and DiscoveryConfig
// are exported so cloud-specific plugins (eks, gke) can reuse the
// discovery engine and supply their own authentication.
package kubernetes

import (
	"context"
	"fmt"
	"slices"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// listPageSize is the page size for Kubernetes list calls.
const listPageSize = 500

// DiscoveryConfig holds the resource-selection and enrichment options
// shared by every Kubernetes-based plugin. Embed it inline in a plugin
// Config alongside that plugin's connection or auth fields.
type DiscoveryConfig struct {
	pluginsdk.BaseConfig `json:",inline"`

	ClusterName string `json:"cluster_name,omitempty" description:"Cluster name to prefix asset names with"`

	Namespaces        []string `json:"namespaces,omitempty" description:"Namespaces to discover. Empty or [\"*\"] means all namespaces"`
	ExcludeNamespaces []string `json:"exclude_namespaces,omitempty" description:"Namespaces to skip when discovering all namespaces" default:"[\"kube-system\",\"kube-public\",\"kube-node-lease\"]"`
	LabelSelector     string   `json:"label_selector,omitempty" description:"Only discover namespaced resources matching this label selector (e.g. team=data)"`

	DiscoverNamespaces   bool `json:"discover_namespaces" description:"Discover namespaces" default:"true"`
	DiscoverServices     bool `json:"discover_services" description:"Discover services" default:"true"`
	DiscoverDeployments  bool `json:"discover_deployments" description:"Discover deployments" default:"true"`
	DiscoverStatefulSets bool `json:"discover_statefulsets" label:"Discover StatefulSets" description:"Discover stateful sets" default:"true"`
	DiscoverCronJobs     bool `json:"discover_cronjobs" label:"Discover CronJobs" description:"Discover cron jobs, with their recent job runs as run history" default:"true"`
	DiscoverPods         bool `json:"discover_pods" description:"Discover pods. Off by default because pods are short-lived and can flood the catalog" default:"false"`

	LabelsToMetadata      bool `json:"labels_to_metadata" description:"Include resource labels in asset metadata" default:"true"`
	AnnotationsToMetadata bool `json:"annotations_to_metadata" description:"Include resource annotations in asset metadata" default:"false"`
}

// Validate checks the discovery options: at least one resource kind must
// be enabled and the label selector must parse.
func (c *DiscoveryConfig) Validate() error {
	if !c.DiscoverNamespaces && !c.DiscoverServices && !c.DiscoverDeployments &&
		!c.DiscoverStatefulSets && !c.DiscoverCronJobs && !c.DiscoverPods {
		return fmt.Errorf("nothing to discover: enable at least one of discover_namespaces, discover_services, discover_deployments, discover_statefulsets, discover_cronjobs, discover_pods")
	}

	if c.LabelSelector != "" {
		if _, err := labels.Parse(c.LabelSelector); err != nil {
			return fmt.Errorf("invalid label_selector %q: %w", c.LabelSelector, err)
		}
	}

	return nil
}

// Discoverer runs asset discovery against a Kubernetes cluster. Build one
// with a ready client and a DiscoveryConfig, then call Discover.
type Discoverer struct {
	client               k8s.Interface
	config               *DiscoveryConfig
	extraMetadata        map[string]interface{}
	clusterMetadata      map[string]interface{}
	clusterExternalLinks []pluginsdk.AssetExternalLink
}

// NewDiscoverer returns a Discoverer for the given client and config.
func NewDiscoverer(client k8s.Interface, config *DiscoveryConfig) *Discoverer {
	return &Discoverer{client: client, config: config}
}

// WithMetadata stamps the given key/values onto every discovered asset,
// including the cluster asset. Cloud plugins use it to record where the
// cluster lives (provider, project or account, region), so that context
// is on each asset directly and does not depend on lineage traversal.
func (d *Discoverer) WithMetadata(metadata map[string]interface{}) *Discoverer {
	d.extraMetadata = metadata
	return d
}

// WithClusterMetadata adds metadata to the cluster asset only. Cloud
// plugins use it for cluster-scoped identifiers like the cluster ARN.
// Ignored when no cluster asset is produced (no cluster_name set).
func (d *Discoverer) WithClusterMetadata(metadata map[string]interface{}) *Discoverer {
	d.clusterMetadata = metadata
	return d
}

// WithClusterExternalLinks attaches external links to the cluster asset.
// Cloud plugins use it to link to the cluster in the cloud console.
// Ignored when no cluster asset is produced (no cluster_name set).
func (d *Discoverer) WithClusterExternalLinks(links []pluginsdk.AssetExternalLink) *Discoverer {
	d.clusterExternalLinks = links
	return d
}

// Discover discovers namespaces, services, workloads, and cron jobs and
// the lineage and run history between them.
func (d *Discoverer) Discover(ctx context.Context) (*pluginsdk.DiscoveryResult, error) {
	namespaces, err := d.listNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}

	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory

	// The cluster itself is an asset when it has a name; there is no
	// reliable way to identify a cluster from inside the API, so an
	// unnamed cluster stays implicit.
	if d.config.ClusterName != "" {
		assets = append(assets, d.createClusterAsset())
	}

	if d.config.DiscoverNamespaces {
		for _, ns := range namespaces {
			assets = append(assets, d.createNamespaceAsset(ns))
			if d.config.ClusterName != "" {
				lineage = append(lineage, pluginsdk.LineageEdge{
					Source: d.clusterMRN(),
					Target: d.assetMRN("Namespace", ns.Name),
					Type:   "CONTAINS",
				})
			}
		}
	}

	for _, ns := range namespaces {
		nsAssets, nsLineage, nsRunHistory, err := d.discoverNamespace(ctx, ns.Name)
		if err != nil {
			log.Warn().Err(err).Str("namespace", ns.Name).Msg("Failed to discover namespace resources")
			continue
		}
		assets = append(assets, nsAssets...)
		lineage = append(lineage, nsLineage...)
		runHistory = append(runHistory, nsRunHistory...)
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineage", len(lineage)).
		Int("run_history", len(runHistory)).
		Msg("Kubernetes discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineage,
		RunHistory: runHistory,
	}, nil
}

// workloadRef is a discovered workload that services can select.
type workloadRef struct {
	assetType string
	name      string
	podLabels map[string]string
}

// discoverNamespace discovers the configured resource kinds within a
// single namespace and links them together.
func (d *Discoverer) discoverNamespace(ctx context.Context, namespace string) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, []pluginsdk.AssetRunHistory, error) {
	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge
	var runHistory []pluginsdk.AssetRunHistory
	var workloads []workloadRef

	if d.config.DiscoverDeployments {
		deployments, err := d.listDeployments(ctx, namespace)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("listing deployments: %w", err)
		}
		for _, dep := range deployments {
			assets = append(assets, d.createDeploymentAsset(dep))
			lineage = append(lineage, d.namespaceEdge("Deployment", namespace, dep.Name)...)
			workloads = append(workloads, workloadRef{"Deployment", dep.Name, dep.Spec.Template.Labels})
		}
	}

	if d.config.DiscoverStatefulSets {
		statefulSets, err := d.listStatefulSets(ctx, namespace)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("listing stateful sets: %w", err)
		}
		for _, sts := range statefulSets {
			assets = append(assets, d.createStatefulSetAsset(sts))
			lineage = append(lineage, d.namespaceEdge("StatefulSet", namespace, sts.Name)...)
			workloads = append(workloads, workloadRef{"StatefulSet", sts.Name, sts.Spec.Template.Labels})
		}
	}

	if d.config.DiscoverServices {
		services, err := d.listServices(ctx, namespace)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("listing services: %w", err)
		}
		for _, svc := range services {
			assets = append(assets, d.createServiceAsset(svc))
			lineage = append(lineage, d.namespaceEdge("Service", namespace, svc.Name)...)

			for _, w := range workloads {
				if serviceSelects(svc, w.podLabels) {
					lineage = append(lineage, pluginsdk.LineageEdge{
						Source: d.assetMRN("Service", namespace, svc.Name),
						Target: d.assetMRN(w.assetType, namespace, w.name),
						Type:   "EXPOSES",
					})
				}
			}
		}
	}

	if d.config.DiscoverCronJobs {
		cronJobs, err := d.listCronJobs(ctx, namespace)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("listing cron jobs: %w", err)
		}

		jobsByCronJob := map[string][]batchv1.Job{}
		if len(cronJobs) > 0 {
			jobsByCronJob, err = d.mapJobsToCronJobs(ctx, namespace)
			if err != nil {
				log.Warn().Err(err).Str("namespace", namespace).Msg("Failed to list jobs; skipping cron job run history")
			}
		}

		for _, cj := range cronJobs {
			assets = append(assets, d.createCronJobAsset(cj))
			lineage = append(lineage, d.namespaceEdge("CronJob", namespace, cj.Name)...)

			if runs := cronJobRunHistory(d.assetMRN("CronJob", namespace, cj.Name), namespace, cj.Name, jobsByCronJob[cj.Name]); len(runs.Runs) > 0 {
				runHistory = append(runHistory, runs)
			}
		}
	}

	if d.config.DiscoverPods {
		pods, err := d.listPods(ctx, namespace)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("listing pods: %w", err)
		}

		deploymentByReplicaSet := map[string]string{}
		if d.config.DiscoverDeployments {
			deploymentByReplicaSet, err = d.mapReplicaSetsToDeployments(ctx, namespace)
			if err != nil {
				log.Warn().Err(err).Str("namespace", namespace).Msg("Failed to list replica sets; skipping deployment-to-pod lineage")
			}
		}

		for _, pod := range pods {
			assets = append(assets, d.createPodAsset(pod))
			lineage = append(lineage, d.namespaceEdge("Pod", namespace, pod.Name)...)

			ownerKind, ownerName := podController(pod)
			switch ownerKind {
			case "ReplicaSet":
				if deployment, ok := deploymentByReplicaSet[ownerName]; ok {
					lineage = append(lineage, pluginsdk.LineageEdge{
						Source: d.assetMRN("Deployment", namespace, deployment),
						Target: d.assetMRN("Pod", namespace, pod.Name),
						Type:   "CONTAINS",
					})
				}
			case "StatefulSet":
				if d.config.DiscoverStatefulSets {
					lineage = append(lineage, pluginsdk.LineageEdge{
						Source: d.assetMRN("StatefulSet", namespace, ownerName),
						Target: d.assetMRN("Pod", namespace, pod.Name),
						Type:   "CONTAINS",
					})
				}
			}
		}
	}

	return assets, lineage, runHistory, nil
}

// listNamespaces resolves the namespaces to discover: the configured
// list when set, otherwise all namespaces minus the excluded ones.
// A "*" entry means all namespaces, same as leaving the list empty.
func (d *Discoverer) listNamespaces(ctx context.Context) ([]corev1.Namespace, error) {
	if len(d.config.Namespaces) > 0 && !slices.Contains(d.config.Namespaces, "*") {
		var namespaces []corev1.Namespace
		for _, name := range d.config.Namespaces {
			ns, err := d.client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				log.Warn().Err(err).Str("namespace", name).Msg("Failed to get configured namespace")
				continue
			}
			namespaces = append(namespaces, *ns)
		}
		return namespaces, nil
	}

	excluded := make(map[string]bool, len(d.config.ExcludeNamespaces))
	for _, name := range d.config.ExcludeNamespaces {
		excluded[name] = true
	}

	var namespaces []corev1.Namespace
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		page, err := d.client.CoreV1().Namespaces().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, ns := range page.Items {
			if !excluded[ns.Name] {
				namespaces = append(namespaces, ns)
			}
		}
		if page.Continue == "" {
			return namespaces, nil
		}
		opts.Continue = page.Continue
	}
}

func (d *Discoverer) listServices(ctx context.Context, namespace string) ([]corev1.Service, error) {
	var services []corev1.Service
	opts := metav1.ListOptions{LabelSelector: d.config.LabelSelector, Limit: listPageSize}
	for {
		page, err := d.client.CoreV1().Services(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		services = append(services, page.Items...)
		if page.Continue == "" {
			return services, nil
		}
		opts.Continue = page.Continue
	}
}

func (d *Discoverer) listDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
	var deployments []appsv1.Deployment
	opts := metav1.ListOptions{LabelSelector: d.config.LabelSelector, Limit: listPageSize}
	for {
		page, err := d.client.AppsV1().Deployments(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, page.Items...)
		if page.Continue == "" {
			return deployments, nil
		}
		opts.Continue = page.Continue
	}
}

func (d *Discoverer) listStatefulSets(ctx context.Context, namespace string) ([]appsv1.StatefulSet, error) {
	var statefulSets []appsv1.StatefulSet
	opts := metav1.ListOptions{LabelSelector: d.config.LabelSelector, Limit: listPageSize}
	for {
		page, err := d.client.AppsV1().StatefulSets(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		statefulSets = append(statefulSets, page.Items...)
		if page.Continue == "" {
			return statefulSets, nil
		}
		opts.Continue = page.Continue
	}
}

func (d *Discoverer) listCronJobs(ctx context.Context, namespace string) ([]batchv1.CronJob, error) {
	var cronJobs []batchv1.CronJob
	opts := metav1.ListOptions{LabelSelector: d.config.LabelSelector, Limit: listPageSize}
	for {
		page, err := d.client.BatchV1().CronJobs(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		cronJobs = append(cronJobs, page.Items...)
		if page.Continue == "" {
			return cronJobs, nil
		}
		opts.Continue = page.Continue
	}
}

func (d *Discoverer) listPods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	var pods []corev1.Pod
	opts := metav1.ListOptions{LabelSelector: d.config.LabelSelector, Limit: listPageSize}
	for {
		page, err := d.client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		pods = append(pods, page.Items...)
		if page.Continue == "" {
			return pods, nil
		}
		opts.Continue = page.Continue
	}
}

// mapReplicaSetsToDeployments maps replica set names to the deployment
// that owns them, so pods (owned by replica sets) can be linked to
// their deployment.
func (d *Discoverer) mapReplicaSetsToDeployments(ctx context.Context, namespace string) (map[string]string, error) {
	result := make(map[string]string)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		page, err := d.client.AppsV1().ReplicaSets(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, rs := range page.Items {
			for _, ref := range rs.OwnerReferences {
				if ref.Controller != nil && *ref.Controller && ref.Kind == "Deployment" {
					result[rs.Name] = ref.Name
				}
			}
		}
		if page.Continue == "" {
			return result, nil
		}
		opts.Continue = page.Continue
	}
}

// mapJobsToCronJobs groups jobs by the cron job that owns them, for
// run history. Jobs without a cron job owner (one-off jobs) are skipped.
func (d *Discoverer) mapJobsToCronJobs(ctx context.Context, namespace string) (map[string][]batchv1.Job, error) {
	result := make(map[string][]batchv1.Job)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		page, err := d.client.BatchV1().Jobs(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, job := range page.Items {
			for _, ref := range job.OwnerReferences {
				if ref.Controller != nil && *ref.Controller && ref.Kind == "CronJob" {
					result[ref.Name] = append(result[ref.Name], job)
				}
			}
		}
		if page.Continue == "" {
			return result, nil
		}
		opts.Continue = page.Continue
	}
}

// namespaceEdge links a namespaced resource to its namespace asset.
// Skipped when namespaces are not discovered, to avoid dangling edges.
func (d *Discoverer) namespaceEdge(assetType, namespace, name string) []pluginsdk.LineageEdge {
	if !d.config.DiscoverNamespaces {
		return nil
	}
	return []pluginsdk.LineageEdge{{
		Source: d.assetMRN("Namespace", namespace),
		Target: d.assetMRN(assetType, namespace, name),
		Type:   "CONTAINS",
	}}
}

// serviceSelects reports whether the service's selector matches a
// workload's pod template labels.
func serviceSelects(svc corev1.Service, podLabels map[string]string) bool {
	if len(svc.Spec.Selector) == 0 {
		return false
	}
	return labels.SelectorFromSet(svc.Spec.Selector).Matches(labels.Set(podLabels))
}

// podController returns the kind and name of the pod's controlling owner.
func podController(pod corev1.Pod) (kind, name string) {
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind, ref.Name
		}
	}
	return "", ""
}

// Config for the standalone Kubernetes plugin: discovery options plus a
// kubeconfig or direct host/token connection.
type Config struct {
	DiscoveryConfig `json:",inline"`

	KubeconfigPath string `json:"kubeconfig_path,omitempty" description:"Kubeconfig path. Defaults to in-cluster, then $KUBECONFIG"`
	Context        string `json:"context,omitempty" description:"Kubeconfig context. Defaults to the current context"`

	Host          string `json:"host,omitempty" description:"API server URL for direct token authentication" validate:"omitempty,url"`
	Token         string `json:"token,omitempty" description:"Bearer token, typically a service account token" sensitive:"true"`
	CACertificate string `json:"ca_certificate,omitempty" label:"CA Certificate" description:"PEM-encoded CA certificate of the API server"`
}

// Example configuration for the plugin
var _ = `
cluster_name: "prod"
namespaces:
  - "payments"
  - "orders"
discover_pods: false
tags:
  - "kubernetes"
  - "${labels.team}"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "kubernetes",
		Name:        "Kubernetes",
		Description: "Discover namespaces, services, workloads, and cron jobs from Kubernetes clusters",
		Icon:        "kubernetes",
		Category:    "compute",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source implements the standalone Kubernetes plugin.
type Source struct {
	config *Config
	client k8s.Interface
}

// Validate validates the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	if err := config.DiscoveryConfig.Validate(); err != nil {
		return nil, err
	}

	if config.Host != "" && config.Token == "" {
		return nil, fmt.Errorf("token is required when host is set")
	}
	if config.Token != "" && config.Host == "" {
		return nil, fmt.Errorf("host is required when token is set")
	}
	if config.Host != "" && (config.KubeconfigPath != "" || config.Context != "") {
		return nil, fmt.Errorf("host/token and kubeconfig_path/context are separate authentication methods: set one or the other")
	}

	s.config = config
	return rawConfig, nil
}

// Discover builds a client from the plugin config and runs discovery.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)
	s.config = config

	if s.client == nil {
		client, err := newClient(config)
		if err != nil {
			return nil, fmt.Errorf("creating Kubernetes client: %w", err)
		}
		s.client = client
	}

	return NewDiscoverer(s.client, &config.DiscoveryConfig).Discover(ctx)
}

// newClient builds a Kubernetes client from the plugin configuration.
// A host/token pair connects directly; otherwise it prefers in-cluster
// config, then falls back to the default kubeconfig loading rules.
func newClient(config *Config) (k8s.Interface, error) {
	restConfig, err := buildRestConfig(config)
	if err != nil {
		return nil, err
	}
	return k8s.NewForConfig(restConfig)
}

func buildRestConfig(config *Config) (*rest.Config, error) {
	if config.Host != "" {
		restConfig := &rest.Config{
			Host:        config.Host,
			BearerToken: config.Token,
		}
		if config.CACertificate != "" {
			restConfig.TLSClientConfig = rest.TLSClientConfig{CAData: []byte(config.CACertificate)}
		}
		return restConfig, nil
	}

	if config.KubeconfigPath == "" && config.Context == "" {
		if restConfig, err := rest.InClusterConfig(); err == nil {
			return restConfig, nil
		}
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = config.KubeconfigPath
	overrides := &clientcmd.ConfigOverrides{CurrentContext: config.Context}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("no usable connection: set host and token to connect to a remote cluster, or kubeconfig_path to point at a kubeconfig file (in-cluster and default kubeconfig are both unavailable): %w", err)
	}
	return restConfig, nil
}
