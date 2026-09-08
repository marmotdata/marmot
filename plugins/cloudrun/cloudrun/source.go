// Package cloudrun discovers services and jobs from Google Cloud Run.
package cloudrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	run "google.golang.org/api/run/v2"
)

// provider is the exact service string Marmot stores for Cloud Run assets.
const provider = "Cloud Run"

// Asset types this plugin creates.
const (
	typeService = "Service"
	typeJob     = "Job"
)

// gcsProvider and gcsBucketType are the identity the gcs plugin gives a
// bucket. Lineage edges use them verbatim so a bucket discovered by either
// plugin is the same asset.
const (
	gcsProvider   = "GCS"
	gcsBucketType = "Bucket"
)

// Config for the Cloud Run plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ProjectID       string   `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	Locations       []string `json:"locations,omitempty" description:"Regions to scan. Every region is scanned when this is empty"`
	CredentialsFile string   `json:"credentials_file,omitempty" description:"Path to service account JSON file"`
	CredentialsJSON string   `json:"credentials_json,omitempty" description:"Service account JSON content" sensitive:"true"`
	Endpoint        string   `json:"endpoint,omitempty" description:"Custom endpoint URL, for testing against a local server"`
	DisableAuth     bool     `json:"disable_auth,omitempty" description:"Disable authentication, for local testing"`

	IncludeJobs         bool `json:"include_jobs" description:"Whether to discover jobs" default:"true"`
	IncludeExecutions   bool `json:"include_executions" description:"Whether to read recent job executions as run history" default:"true"`
	MaxExecutionsPerJob int  `json:"max_executions_per_job" description:"How many recent executions to read per job" default:"10" validate:"omitempty,min=1,max=100"`
}

// Example configuration for the plugin
var _ = `
project_id: "acme-prod"
credentials_file: "/etc/marmot/cloudrun-service-account.json"
locations:
  - "europe-west1"
include_jobs: true
include_executions: true
max_executions_per_job: 10
tags:
  - "cloudrun"
  - "serverless"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "cloudrun",
		Name:        "Google Cloud Run",
		Description: "Discover services and jobs from Google Cloud Run",
		Icon:        "cloud-run",
		Category:    "container",
		Status:      "experimental",
		// Discover links mounted GCS buckets to the workload that reads them
		// and reports job executions, so the manifest declares all three.
		Features:   []string{"Assets", "Lineage", "Run History"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Cloud Run plugin.
type Source struct {
	config *Config
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	config.ProjectID = strings.TrimSpace(config.ProjectID)
	for i, location := range config.Locations {
		config.Locations[i] = strings.TrimSpace(location)
	}

	// The Google client library resolves request paths relative to the
	// endpoint, so an endpoint without a trailing slash would lose its last
	// path segment.
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	if config.Endpoint != "" && !strings.HasSuffix(config.Endpoint, "/") {
		config.Endpoint += "/"
	}

	if config.CredentialsJSON != "" && config.CredentialsFile != "" {
		return nil, fmt.Errorf("set either credentials_json or credentials_file, not both")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Cloud Run services and jobs.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	c, err := newClient(ctx, s.config)
	if err != nil {
		return nil, err
	}

	result := &pluginsdk.DiscoveryResult{}

	if err := s.discoverServices(ctx, c, result); err != nil {
		return nil, err
	}

	if s.config.IncludeJobs {
		if err := s.discoverJobs(ctx, c, result); err != nil {
			return nil, err
		}
	}

	log.Info().
		Str("project_id", s.config.ProjectID).
		Int("assets", len(result.Assets)).
		Int("lineages", len(result.Lineage)).
		Int("statistics", len(result.Statistics)).
		Int("run_history", len(result.RunHistory)).
		Msg("Cloud Run discovery completed")

	return result, nil
}

// discoverServices turns every Cloud Run service into an asset. A region that
// fails on its own is a warning; only a project where no region could be read
// at all is an error, because that means the API or the credentials are the
// problem rather than one region.
func (s *Source) discoverServices(ctx context.Context, c *client, result *pluginsdk.DiscoveryResult) error {
	var (
		firstErr error
		listed   int
	)

	for _, parent := range s.parents() {
		services, unreachable, err := c.listServices(ctx, parent)
		if err != nil {
			log.Warn().Err(err).Str("parent", parent).Msg("Failed to list services")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		listed++

		for _, location := range unreachable {
			log.Warn().Str("location", location).Msg("Cloud Run could not reach this location, its services are missing from this run")
		}

		for _, service := range services {
			asset, buckets, ok := s.serviceAsset(service)
			if !ok {
				continue
			}
			result.Assets = append(result.Assets, asset)
			result.Lineage = append(result.Lineage, bucketLineage(buckets, *asset.MRN)...)
		}
	}

	if listed == 0 && firstErr != nil {
		return fmt.Errorf("listing services: %w", firstErr)
	}
	return nil
}

// discoverJobs turns every Cloud Run job into an asset, an execution count
// statistic and, when enabled, the run history of its recent executions.
func (s *Source) discoverJobs(ctx context.Context, c *client, result *pluginsdk.DiscoveryResult) error {
	var (
		firstErr error
		listed   int
	)

	for _, parent := range s.parents() {
		jobs, err := c.listJobs(ctx, parent)
		if err != nil {
			log.Warn().Err(err).Str("parent", parent).Msg("Failed to list jobs")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		listed++

		for _, job := range jobs {
			asset, buckets, ok := s.jobAsset(job)
			if !ok {
				continue
			}
			result.Assets = append(result.Assets, asset)
			result.Lineage = append(result.Lineage, bucketLineage(buckets, *asset.MRN)...)
			result.Statistics = append(result.Statistics, pluginsdk.Statistic{
				AssetMRN:   *asset.MRN,
				MetricName: "asset.execution_count",
				Value:      float64(job.ExecutionCount),
			})

			if !s.config.IncludeExecutions || s.config.MaxExecutionsPerJob <= 0 {
				continue
			}

			executions, err := c.listExecutions(ctx, job.Name, s.config.MaxExecutionsPerJob)
			if err != nil {
				log.Warn().Err(err).Str("job", job.Name).Msg("Failed to list job executions")
				continue
			}

			history := runHistoryFor(*asset.MRN, s.config.ProjectID, *asset.Name, executions)
			if len(history.Runs) > 0 {
				result.RunHistory = append(result.RunHistory, history)
			}
		}
	}

	if listed == 0 && firstErr != nil {
		return fmt.Errorf("listing jobs: %w", firstErr)
	}
	return nil
}

// serviceAsset builds the asset for one Cloud Run service and returns the GCS
// buckets it mounts. A service whose resource name cannot be parsed is skipped
// with a warning: without a location and an id there is no stable identity to
// store it under.
func (s *Source) serviceAsset(service *run.GoogleCloudRunV2Service) (pluginsdk.Asset, []string, bool) {
	parsed, err := parseResourceName(service.Name)
	if err != nil {
		log.Warn().Err(err).Msg("Skipping service with an unparseable resource name")
		return pluginsdk.Asset{}, nil, false
	}

	name := assetName(parsed)
	metadata := map[string]any{}

	setString(metadata, "uid", service.Uid)
	setInt(metadata, "generation", service.Generation)
	setString(metadata, "location", parsed.Location)
	setString(metadata, "project_id", parsed.Project)
	setString(metadata, "uri", service.Uri)
	setString(metadata, "ingress", service.Ingress)
	setString(metadata, "launch_stage", service.LaunchStage)
	setString(metadata, "creator", service.Creator)
	setString(metadata, "last_modifier", service.LastModifier)
	setString(metadata, "create_time", service.CreateTime)
	setString(metadata, "update_time", service.UpdateTime)
	setString(metadata, "latest_ready_revision", resourceID(service.LatestReadyRevision))
	setString(metadata, "latest_created_revision", resourceID(service.LatestCreatedRevision))
	setString(metadata, "traffic", formatTraffic(service.TrafficStatuses, service.Traffic))
	if service.TerminalCondition != nil {
		setString(metadata, "ready", service.TerminalCondition.State)
	}
	if service.Reconciling {
		metadata["reconciling"] = true
	}

	var buckets []string
	if template := service.Template; template != nil {
		setString(metadata, "execution_environment", template.ExecutionEnvironment)
		setString(metadata, "service_account", template.ServiceAccount)
		setString(metadata, "timeout", template.Timeout)
		setInt(metadata, "max_instance_request_concurrency", template.MaxInstanceRequestConcurrency)

		if scaling := template.Scaling; scaling != nil {
			setInt(metadata, "min_instance_count", scaling.MinInstanceCount)
			setInt(metadata, "max_instance_count", scaling.MaxInstanceCount)
		}

		if vpc := template.VpcAccess; vpc != nil {
			setString(metadata, "vpc_connector", vpc.Connector)
			setString(metadata, "vpc_egress", vpc.Egress)
		}

		addContainerMetadata(metadata, template.Containers)
		buckets = addVolumeMetadata(metadata, template.Volumes)
	}

	addLabels(metadata, service.Labels)

	asset := s.newAsset(typeService, name, service.Description, metadata)
	return asset, buckets, true
}

// jobAsset builds the asset for one Cloud Run job and returns the GCS buckets
// its tasks mount.
func (s *Source) jobAsset(job *run.GoogleCloudRunV2Job) (pluginsdk.Asset, []string, bool) {
	parsed, err := parseResourceName(job.Name)
	if err != nil {
		log.Warn().Err(err).Msg("Skipping job with an unparseable resource name")
		return pluginsdk.Asset{}, nil, false
	}

	name := assetName(parsed)
	metadata := map[string]any{}

	setString(metadata, "uid", job.Uid)
	setInt(metadata, "generation", job.Generation)
	setString(metadata, "location", parsed.Location)
	setString(metadata, "project_id", parsed.Project)
	setString(metadata, "creator", job.Creator)
	setString(metadata, "last_modifier", job.LastModifier)
	setString(metadata, "create_time", job.CreateTime)
	setString(metadata, "update_time", job.UpdateTime)
	setString(metadata, "launch_stage", job.LaunchStage)
	setInt(metadata, "execution_count", job.ExecutionCount)
	if job.LatestCreatedExecution != nil {
		setString(metadata, "latest_created_execution", resourceID(job.LatestCreatedExecution.Name))
	}
	if job.Reconciling {
		metadata["reconciling"] = true
	}

	var buckets []string
	if template := job.Template; template != nil {
		setInt(metadata, "task_count", template.TaskCount)
		setInt(metadata, "parallelism", template.Parallelism)

		// A job's Template is the execution template; the task template one
		// level down is what actually carries containers and volumes.
		if task := template.Template; task != nil {
			setString(metadata, "service_account", task.ServiceAccount)
			setString(metadata, "timeout", task.Timeout)
			setString(metadata, "execution_environment", task.ExecutionEnvironment)
			setInt(metadata, "max_retries", task.MaxRetries)

			if vpc := task.VpcAccess; vpc != nil {
				setString(metadata, "vpc_connector", vpc.Connector)
				setString(metadata, "vpc_egress", vpc.Egress)
			}

			addContainerMetadata(metadata, task.Containers)
			buckets = addVolumeMetadata(metadata, task.Volumes)
		}
	}

	addLabels(metadata, job.Labels)

	asset := s.newAsset(typeJob, name, "", metadata)
	return asset, buckets, true
}

// newAsset assembles the parts every Cloud Run asset shares.
func (s *Source) newAsset(assetType, name, description string, metadata map[string]any) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if description != "" {
		asset.Description = &description
	}

	return asset
}

// assetName qualifies a workload by its region. One service or job id can
// exist in several regions of the same project, and the plugin scans every
// region by default, so the region is part of the name rather than only
// metadata.
func assetName(parsed resourceName) string {
	return parsed.Location + "/" + parsed.ID
}

// assetMRN is the single place a Cloud Run MRN is built, so assets and both
// ends of every lineage edge can never drift apart.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// bucketLineage points every mounted GCS bucket at the workload that reads it.
// The bucket end uses the gcs plugin's identity so the edge lands on the same
// asset that plugin creates.
func bucketLineage(buckets []string, workloadMRN string) []pluginsdk.LineageEdge {
	edges := make([]pluginsdk.LineageEdge, 0, len(buckets))
	for _, bucket := range buckets {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: mrn.New(gcsBucketType, gcsProvider, bucket),
			Target: workloadMRN,
			Type:   "FEEDS",
		})
	}
	return edges
}

// addContainerMetadata records what a workload runs. Environment variable
// values are deliberately left out: container env routinely holds credentials,
// so only the names are recorded.
func addContainerMetadata(metadata map[string]any, containers []*run.GoogleCloudRunV2Container) {
	var (
		images   []string
		ports    []int64
		envNames []string
	)

	for _, container := range containers {
		if container == nil {
			continue
		}
		if container.Image != "" {
			images = appendUnique(images, container.Image)
		}
		for _, port := range container.Ports {
			if port != nil && port.ContainerPort != 0 {
				ports = appendUniqueInt(ports, port.ContainerPort)
			}
		}
		for _, env := range container.Env {
			if env != nil && env.Name != "" {
				envNames = appendUnique(envNames, env.Name)
			}
		}
	}

	if len(images) > 0 {
		metadata["container_image"] = images[0]
		metadata["container_images"] = images
	}
	if len(ports) > 0 {
		metadata["container_ports"] = ports
	}
	if len(envNames) > 0 {
		metadata["env_var_names"] = envNames
	}
}

// addVolumeMetadata records what a workload mounts and returns the GCS buckets
// among them, which are the only mounts this plugin turns into lineage.
func addVolumeMetadata(metadata map[string]any, volumes []*run.GoogleCloudRunV2Volume) []string {
	var (
		buckets   []string
		instances []string
		secrets   []string
		nfs       []string
	)

	for _, volume := range volumes {
		if volume == nil {
			continue
		}
		if volume.Gcs != nil && volume.Gcs.Bucket != "" {
			buckets = appendUnique(buckets, volume.Gcs.Bucket)
		}
		if volume.CloudSqlInstance != nil {
			for _, instance := range volume.CloudSqlInstance.Instances {
				if instance != "" {
					instances = appendUnique(instances, instance)
				}
			}
		}
		if volume.Secret != nil && volume.Secret.Secret != "" {
			secrets = appendUnique(secrets, volume.Secret.Secret)
		}
		if volume.Nfs != nil && volume.Nfs.Server != "" {
			nfs = appendUnique(nfs, volume.Nfs.Server+":"+volume.Nfs.Path)
		}
	}

	setStrings(metadata, "gcs_volume_buckets", buckets)
	setStrings(metadata, "cloud_sql_instances", instances)
	setStrings(metadata, "secret_volumes", secrets)
	setStrings(metadata, "nfs_volumes", nfs)

	return buckets
}

// addLabels copies a resource's labels in under a prefix so they stay
// searchable without colliding with the plugin's own metadata keys.
func addLabels(metadata map[string]any, labels map[string]string) {
	for key, value := range labels {
		if key == "" {
			continue
		}
		metadata["label_"+key] = value
	}
}

// formatTraffic renders a traffic split compactly, for example "latest=100" or
// "rev-a=90,rev-b=10". TrafficStatuses is what Cloud Run actually serves;
// Traffic is only what was asked for, so it is the fallback.
func formatTraffic(statuses []*run.GoogleCloudRunV2TrafficTargetStatus, targets []*run.GoogleCloudRunV2TrafficTarget) string {
	var parts []string

	for _, status := range statuses {
		if status == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", trafficLabel(status.Revision), status.Percent))
	}

	if len(parts) == 0 {
		for _, target := range targets {
			if target == nil {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%d", trafficLabel(target.Revision), target.Percent))
		}
	}

	return strings.Join(parts, ",")
}

// trafficLabel names the revision a traffic target points at. A target that
// tracks whichever revision is newest carries no revision name.
func trafficLabel(revision string) string {
	if revision == "" {
		return "latest"
	}
	return resourceID(revision)
}

// runHistoryFor turns a job's recent executions into run events. An execution
// that has started emits a START, and then either the terminal event its
// counters describe or a RUNNING event while it is still going.
func runHistoryFor(jobMRN, projectID, jobName string, executions []*run.GoogleCloudRunV2Execution) pluginsdk.AssetRunHistory {
	history := pluginsdk.AssetRunHistory{AssetMRN: jobMRN}

	for _, execution := range executions {
		if execution == nil {
			continue
		}

		runID := resourceID(execution.Name)
		facets := runFacets(execution)

		newEvent := func(eventType string, eventTime time.Time) pluginsdk.RunHistoryEvent {
			return pluginsdk.RunHistoryEvent{
				RunID:        runID,
				JobNamespace: projectID,
				JobName:      jobName,
				EventType:    eventType,
				EventTime:    eventTime,
				RunFacets:    facets,
			}
		}

		start, startOK := parseEventTime(runID, "startTime", execution.StartTime)
		if startOK {
			history.Runs = append(history.Runs, newEvent("START", start))
		}

		if execution.CompletionTime != "" {
			completion, ok := parseEventTime(runID, "completionTime", execution.CompletionTime)
			if ok {
				history.Runs = append(history.Runs, newEvent(terminalEventType(execution), completion))
			}
			continue
		}

		if startOK {
			// Still running: the only timestamp the API has committed to is
			// the start, so the in-flight event carries that.
			history.Runs = append(history.Runs, newEvent("RUNNING", start))
		}
	}

	return history
}

// terminalEventType reads an execution's task counters to decide how it ended.
// Cancellation is reported ahead of success because a cancelled execution can
// still have tasks that finished before the cancel landed.
func terminalEventType(execution *run.GoogleCloudRunV2Execution) string {
	switch {
	case execution.FailedCount > 0:
		return "FAIL"
	case execution.CancelledCount > 0:
		return "ABORT"
	default:
		return "COMPLETE"
	}
}

// runFacets carries the per-execution counters and the log link, which is the
// one deep link Cloud Run itself hands out.
func runFacets(execution *run.GoogleCloudRunV2Execution) map[string]any {
	facets := map[string]any{
		"task_count":      execution.TaskCount,
		"succeeded_count": execution.SucceededCount,
		"failed_count":    execution.FailedCount,
		"cancelled_count": execution.CancelledCount,
		"retried_count":   execution.RetriedCount,
	}
	setString(facets, "log_uri", execution.LogUri)
	return facets
}

// parseEventTime converts an API timestamp. A timestamp that does not parse
// drops the event rather than recording it at the zero time, which would sort
// to 1970 in the run history.
func parseEventTime(runID, field, value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Warn().Err(err).Str("execution", runID).Str("field", field).Msg("Skipping run event with an unparseable timestamp")
		return time.Time{}, false
	}

	return parsed, true
}

func setString(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

func setInt(metadata map[string]any, key string, value int64) {
	if value != 0 {
		metadata[key] = value
	}
}

func setStrings(metadata map[string]any, key string, values []string) {
	if len(values) > 0 {
		metadata[key] = values
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueInt(values []int64, value int64) []int64 {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
