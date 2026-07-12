package kubernetes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// lastAppliedAnnotation is kubectl's copy of the full applied manifest;
// it is large and duplicates everything else, so it is never included.
const lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// assetName builds an asset display name from path segments, prefixed
// with the cluster name when configured.
func (d *Discoverer) assetName(parts ...string) string {
	if d.config.ClusterName != "" {
		parts = append([]string{d.config.ClusterName}, parts...)
	}
	return strings.Join(parts, "/")
}

func (d *Discoverer) assetMRN(assetType string, parts ...string) string {
	return mrn.New(assetType, "Kubernetes", d.assetName(parts...))
}

// newAsset builds an asset with the shared name, MRN, tag, and source
// wiring. Tags support interpolation from metadata, e.g. ${labels.team}.
func (d *Discoverer) newAsset(assetType, name string, metadata map[string]interface{}) pluginsdk.Asset {
	// Stamp the cloud context (set by cloud plugins) onto every asset,
	// without overwriting anything the asset already carries.
	for k, v := range d.extraMetadata {
		if _, ok := metadata[k]; !ok {
			metadata[k] = v
		}
	}

	mrnValue := mrn.New(assetType, "Kubernetes", name)
	tags := pluginsdk.InterpolateTags(d.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{"Kubernetes"},
		Metadata:  metadata,
		Tags:      tags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "Kubernetes",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (d *Discoverer) clusterMRN() string {
	return mrn.New("Cluster", "Kubernetes", d.config.ClusterName)
}

func (d *Discoverer) createClusterAsset() pluginsdk.Asset {
	metadata := map[string]interface{}{
		"cluster": d.config.ClusterName,
	}

	if version, err := d.client.Discovery().ServerVersion(); err != nil {
		log.Warn().Err(err).Msg("Failed to get server version")
	} else {
		metadata["kubernetes_version"] = version.GitVersion
		if version.Platform != "" {
			metadata["platform"] = version.Platform
		}
	}

	for k, v := range d.clusterMetadata {
		metadata[k] = v
	}

	asset := d.newAsset("Cluster", d.config.ClusterName, metadata)
	asset.ExternalLinks = d.clusterExternalLinks
	return asset
}

func (d *Discoverer) createNamespaceAsset(ns corev1.Namespace) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"namespace": ns.Name,
		"phase":     string(ns.Status.Phase),
	}
	d.addObjectMeta(metadata, ns.ObjectMeta)

	return d.newAsset("Namespace", d.assetName(ns.Name), metadata)
}

func (d *Discoverer) createServiceAsset(svc corev1.Service) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"namespace":    svc.Namespace,
		"service_type": string(svc.Spec.Type),
	}

	if svc.Spec.ClusterIP != "" {
		metadata["cluster_ip"] = svc.Spec.ClusterIP
	}
	if svc.Spec.ExternalName != "" {
		metadata["external_name"] = svc.Spec.ExternalName
	}
	if len(svc.Spec.Ports) > 0 {
		metadata["ports"] = formatServicePorts(svc.Spec.Ports)
	}
	if len(svc.Spec.Selector) > 0 {
		metadata["selector"] = formatLabelMap(svc.Spec.Selector)
	}
	if hosts := loadBalancerHosts(svc.Status.LoadBalancer); hosts != "" {
		metadata["load_balancer_hosts"] = hosts
	}
	d.addObjectMeta(metadata, svc.ObjectMeta)

	return d.newAsset("Service", d.assetName(svc.Namespace, svc.Name), metadata)
}

func (d *Discoverer) createDeploymentAsset(dep appsv1.Deployment) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"namespace":          dep.Namespace,
		"ready_replicas":     dep.Status.ReadyReplicas,
		"available_replicas": dep.Status.AvailableReplicas,
		"updated_replicas":   dep.Status.UpdatedReplicas,
	}

	if dep.Spec.Replicas != nil {
		metadata["replicas"] = *dep.Spec.Replicas
	}
	if dep.Spec.Strategy.Type != "" {
		metadata["strategy"] = string(dep.Spec.Strategy.Type)
	}
	if dep.Spec.Paused {
		metadata["paused"] = true
	}
	if images := containerImages(dep.Spec.Template.Spec.Containers); images != "" {
		metadata["images"] = images
	}
	metadata["container_count"] = len(dep.Spec.Template.Spec.Containers)
	if sa := dep.Spec.Template.Spec.ServiceAccountName; sa != "" {
		metadata["service_account"] = sa
	}
	d.addObjectMeta(metadata, dep.ObjectMeta)

	return d.newAsset("Deployment", d.assetName(dep.Namespace, dep.Name), metadata)
}

func (d *Discoverer) createStatefulSetAsset(sts appsv1.StatefulSet) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"namespace":        sts.Namespace,
		"ready_replicas":   sts.Status.ReadyReplicas,
		"updated_replicas": sts.Status.UpdatedReplicas,
	}

	if sts.Spec.Replicas != nil {
		metadata["replicas"] = *sts.Spec.Replicas
	}
	if sts.Spec.UpdateStrategy.Type != "" {
		metadata["strategy"] = string(sts.Spec.UpdateStrategy.Type)
	}
	if sts.Spec.ServiceName != "" {
		metadata["headless_service"] = sts.Spec.ServiceName
	}
	if images := containerImages(sts.Spec.Template.Spec.Containers); images != "" {
		metadata["images"] = images
	}
	metadata["container_count"] = len(sts.Spec.Template.Spec.Containers)
	if sa := sts.Spec.Template.Spec.ServiceAccountName; sa != "" {
		metadata["service_account"] = sa
	}
	if claims := formatVolumeClaims(sts.Spec.VolumeClaimTemplates); claims != "" {
		metadata["volume_claims"] = claims
	}
	d.addObjectMeta(metadata, sts.ObjectMeta)

	return d.newAsset("StatefulSet", d.assetName(sts.Namespace, sts.Name), metadata)
}

func (d *Discoverer) createCronJobAsset(cj batchv1.CronJob) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"namespace": cj.Namespace,
		"schedule":  cj.Spec.Schedule,
	}

	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		metadata["suspended"] = true
	}
	if cj.Spec.ConcurrencyPolicy != "" {
		metadata["concurrency_policy"] = string(cj.Spec.ConcurrencyPolicy)
	}
	if cj.Spec.TimeZone != nil && *cj.Spec.TimeZone != "" {
		metadata["timezone"] = *cj.Spec.TimeZone
	}
	if images := containerImages(cj.Spec.JobTemplate.Spec.Template.Spec.Containers); images != "" {
		metadata["images"] = images
	}
	if cj.Status.LastScheduleTime != nil {
		metadata["last_schedule_time"] = cj.Status.LastScheduleTime.UTC().Format(time.RFC3339)
	}
	if cj.Status.LastSuccessfulTime != nil {
		metadata["last_successful_time"] = cj.Status.LastSuccessfulTime.UTC().Format(time.RFC3339)
	}
	d.addObjectMeta(metadata, cj.ObjectMeta)

	return d.newAsset("CronJob", d.assetName(cj.Namespace, cj.Name), metadata)
}

func (d *Discoverer) createPodAsset(pod corev1.Pod) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"namespace": pod.Namespace,
		"phase":     string(pod.Status.Phase),
	}

	if pod.Spec.NodeName != "" {
		metadata["node"] = pod.Spec.NodeName
	}
	if images := containerImages(pod.Spec.Containers); images != "" {
		metadata["images"] = images
	}
	if pod.Status.QOSClass != "" {
		metadata["qos_class"] = string(pod.Status.QOSClass)
	}
	if sa := pod.Spec.ServiceAccountName; sa != "" {
		metadata["service_account"] = sa
	}

	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	metadata["restart_count"] = restarts

	if ownerKind, ownerName := podController(pod); ownerKind != "" {
		metadata["owner_kind"] = ownerKind
		metadata["owner_name"] = ownerName
	}
	d.addObjectMeta(metadata, pod.ObjectMeta)

	return d.newAsset("Pod", d.assetName(pod.Namespace, pod.Name), metadata)
}

// addObjectMeta adds the metadata every resource kind shares: creation
// time, cluster name, and (per config) labels and annotations. Labels
// go in as a nested map so tags can interpolate them by key.
func (d *Discoverer) addObjectMeta(metadata map[string]interface{}, obj metav1.ObjectMeta) {
	metadata["created_at"] = obj.CreationTimestamp.UTC().Format(time.RFC3339)

	if d.config.ClusterName != "" {
		metadata["cluster"] = d.config.ClusterName
	}

	if d.config.LabelsToMetadata && len(obj.Labels) > 0 {
		labels := make(map[string]interface{}, len(obj.Labels))
		for k, v := range obj.Labels {
			labels[k] = v
		}
		metadata["labels"] = labels
	}

	if d.config.AnnotationsToMetadata {
		annotations := make(map[string]interface{}, len(obj.Annotations))
		for k, v := range obj.Annotations {
			if k == lastAppliedAnnotation {
				continue
			}
			annotations[k] = v
		}
		if len(annotations) > 0 {
			metadata["annotations"] = annotations
		}
	}
}

// cronJobRunHistory converts a cron job's child jobs into run history
// events, one START per job start and one COMPLETE or FAIL per outcome.
func cronJobRunHistory(cronJobMRN, namespace, cronJobName string, jobs []batchv1.Job) pluginsdk.AssetRunHistory {
	var events []pluginsdk.RunHistoryEvent

	for _, job := range jobs {
		facets := map[string]interface{}{
			"job_name": job.Name,
		}

		if job.Status.StartTime != nil {
			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        job.Name,
				JobNamespace: namespace,
				JobName:      cronJobName,
				EventType:    "START",
				EventTime:    job.Status.StartTime.UTC(),
				RunFacets:    facets,
			})
		}

		switch {
		case job.Status.CompletionTime != nil:
			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        job.Name,
				JobNamespace: namespace,
				JobName:      cronJobName,
				EventType:    "COMPLETE",
				EventTime:    job.Status.CompletionTime.UTC(),
				RunFacets:    facets,
			})
		case jobFailed(job):
			eventTime := time.Now()
			if t := jobFailureTime(job); !t.IsZero() {
				eventTime = t
			}
			events = append(events, pluginsdk.RunHistoryEvent{
				RunID:        job.Name,
				JobNamespace: namespace,
				JobName:      cronJobName,
				EventType:    "FAIL",
				EventTime:    eventTime,
				RunFacets:    facets,
			})
		}
	}

	return pluginsdk.AssetRunHistory{
		AssetMRN: cronJobMRN,
		Runs:     events,
	}
}

func jobFailed(job batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func jobFailureTime(job batchv1.Job) time.Time {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return cond.LastTransitionTime.UTC()
		}
	}
	return time.Time{}
}

// formatVolumeClaims renders volume claim templates as
// "name:size/storageClass" entries.
func formatVolumeClaims(claims []corev1.PersistentVolumeClaim) string {
	parts := make([]string, 0, len(claims))
	for _, claim := range claims {
		part := claim.Name
		if storage, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			part += ":" + storage.String()
		}
		if claim.Spec.StorageClassName != nil && *claim.Spec.StorageClassName != "" {
			part += "/" + *claim.Spec.StorageClassName
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func formatServicePorts(ports []corev1.ServicePort) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		part := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.Name != "" {
			part = p.Name + ":" + part
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func formatLabelMap(m map[string]string) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func loadBalancerHosts(lb corev1.LoadBalancerStatus) string {
	var hosts []string
	for _, ingress := range lb.Ingress {
		if ingress.Hostname != "" {
			hosts = append(hosts, ingress.Hostname)
		}
		if ingress.IP != "" {
			hosts = append(hosts, ingress.IP)
		}
	}
	return strings.Join(hosts, ", ")
}

// containerImages returns the deduplicated, sorted container images.
func containerImages(containers []corev1.Container) string {
	seen := make(map[string]bool, len(containers))
	var images []string
	for _, c := range containers {
		if c.Image != "" && !seen[c.Image] {
			seen[c.Image] = true
			images = append(images, c.Image)
		}
	}
	sort.Strings(images)
	return strings.Join(images, ", ")
}
