// Package vertexai discovers models, endpoints, datasets, feature groups
// and pipeline jobs from Google Vertex AI.
package vertexai

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
)

// provider is the exact service name every Vertex AI asset is filed under.
// It matches the provider an imported OpenMetadata catalog projects its
// VertexAI services onto, so the two merge into one asset.
const provider = "Vertex AI"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "vertexai",
		Name:        "Vertex AI",
		Description: "Discover models, endpoints, datasets, feature groups and pipeline jobs from Google Vertex AI",
		Icon:        "vertex-ai",
		Category:    "ml",
		Status:      "experimental",
		// Discover emits FEEDS and PRODUCES edges, and run history for
		// pipeline jobs, so the manifest declares all three features.
		Features:   []string{"Assets", "Lineage", "Run History"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Vertex AI plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ProjectID       string   `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	Locations       []string `json:"locations" description:"Regions to scan" validate:"required,min=1"`
	CredentialsFile string   `json:"credentials_file,omitempty" description:"Path to service account JSON file"`
	CredentialsJSON string   `json:"credentials_json,omitempty" description:"Service account JSON content" sensitive:"true"`
	Endpoint        string   `json:"endpoint,omitempty" description:"Custom endpoint URL, for testing against a local server"`
	DisableAuth     bool     `json:"disable_auth,omitempty" description:"Disable authentication, for local testing"`

	IncludeEndpoints     bool `json:"include_endpoints" description:"Whether to discover prediction endpoints" default:"true"`
	IncludeDatasets      bool `json:"include_datasets" description:"Whether to discover managed datasets" default:"true"`
	IncludeFeatureGroups bool `json:"include_feature_groups" description:"Whether to discover feature groups from the feature store" default:"true"`
	IncludePipelineJobs  bool `json:"include_pipeline_jobs" description:"Whether to discover pipeline jobs. Projects keep a long job history, so this is off by default" default:"false"`
	MaxPipelineJobs      int  `json:"max_pipeline_jobs" description:"How many recent pipeline jobs to read" default:"50" validate:"omitempty,min=1,max=1000"`
}

// Example configuration for the plugin
var _ = `
project_id: "acme-ml"
locations:
  - "us-central1"
  - "europe-west4"
credentials_file: "/etc/marmot/vertexai.json"
include_endpoints: true
include_datasets: true
include_feature_groups: true
include_pipeline_jobs: true
max_pipeline_jobs: 100
tags:
  - "gcp"
  - "ml"
`

// Source represents the Vertex AI plugin.
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

	// The client appends its own path to the endpoint, and Google writes
	// every published endpoint with a trailing slash, so normalise to that.
	if endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/"); endpoint != "" {
		config.Endpoint = endpoint + "/"
	} else {
		config.Endpoint = ""
	}

	for i, location := range config.Locations {
		config.Locations[i] = strings.TrimSpace(location)
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Vertex AI models, endpoints, datasets, feature groups
// and pipeline jobs.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	scan := &scan{}
	for _, location := range s.config.Locations {
		service, err := s.serviceFor(ctx, location)
		if err != nil {
			return nil, err
		}
		if err := s.scanLocation(ctx, service, location, scan); err != nil {
			return nil, fmt.Errorf("scanning location %s: %w", location, err)
		}
	}

	return s.build(scan), nil
}

// scan holds what the API returned across every location, before any of it
// is named. Names cannot be decided one resource at a time: a display name
// shared by two resources changes the name of both.
type scan struct {
	models        []scanned[*aiplatform.GoogleCloudAiplatformV1Model]
	endpoints     []scanned[*aiplatform.GoogleCloudAiplatformV1Endpoint]
	datasets      []scanned[*aiplatform.GoogleCloudAiplatformV1Dataset]
	featureGroups []scannedFeatureGroup
	pipelineJobs  []scanned[*aiplatform.GoogleCloudAiplatformV1PipelineJob]
}

// scanned pairs an API resource with the location it was listed from, which
// is the fallback when its resource name does not parse.
type scanned[T any] struct {
	resource T
	location string
}

// scannedFeatureGroup carries a feature group's features, which come from a
// second call made while its location's client is still open. featuresRead
// tells a group with no features apart from one whose features could not be
// read, which are different facts.
type scannedFeatureGroup struct {
	resource     *aiplatform.GoogleCloudAiplatformV1FeatureGroup
	location     string
	features     []*aiplatform.GoogleCloudAiplatformV1Feature
	featuresRead bool
}

// scanLocation lists every enabled resource kind in one location.
func (s *Source) scanLocation(ctx context.Context, service *aiplatform.Service, location string, out *scan) error {
	parent := fmt.Sprintf("projects/%s/locations/%s", s.config.ProjectID, location)

	// Models anchor every other pass, so a failure here means the project
	// or location is unreachable and discovery cannot go on.
	models, err := listModels(ctx, service, parent)
	if err != nil {
		return err
	}
	for _, model := range models {
		out.models = append(out.models, scanned[*aiplatform.GoogleCloudAiplatformV1Model]{resource: model, location: location})
	}

	if s.config.IncludeEndpoints {
		endpoints, err := listEndpoints(ctx, service, parent)
		if err != nil {
			log.Warn().Err(err).Str("location", location).Msg("Failed to list endpoints")
		}
		for _, endpoint := range endpoints {
			out.endpoints = append(out.endpoints, scanned[*aiplatform.GoogleCloudAiplatformV1Endpoint]{resource: endpoint, location: location})
		}
	}

	if s.config.IncludeDatasets {
		datasets, err := listDatasets(ctx, service, parent)
		if err != nil {
			log.Warn().Err(err).Str("location", location).Msg("Failed to list datasets")
		}
		for _, dataset := range datasets {
			out.datasets = append(out.datasets, scanned[*aiplatform.GoogleCloudAiplatformV1Dataset]{resource: dataset, location: location})
		}
	}

	if s.config.IncludeFeatureGroups {
		groups, err := listFeatureGroups(ctx, service, parent)
		if err != nil {
			log.Warn().Err(err).Str("location", location).Msg("Failed to list feature groups")
		}
		for _, group := range groups {
			// One group whose features cannot be read still belongs in the
			// catalog, without its columns.
			features, err := listFeatures(ctx, service, group.Name)
			if err != nil {
				log.Warn().Err(err).Str("feature_group", group.Name).Msg("Failed to list features")
			}
			out.featureGroups = append(out.featureGroups, scannedFeatureGroup{
				resource:     group,
				location:     location,
				features:     features,
				featuresRead: err == nil,
			})
		}
	}

	if s.config.IncludePipelineJobs {
		jobs, err := listPipelineJobs(ctx, service, parent, s.config.MaxPipelineJobs)
		if err != nil {
			log.Warn().Err(err).Str("location", location).Msg("Failed to list pipeline jobs")
		}
		for _, job := range jobs {
			out.pipelineJobs = append(out.pipelineJobs, scanned[*aiplatform.GoogleCloudAiplatformV1PipelineJob]{resource: job, location: location})
		}
	}

	return nil
}

// build turns everything the scan collected into assets, lineage,
// statistics and run history. Jobs are built before models and models
// before endpoints, because each of those passes links back to the names
// the previous one handed out.
func (s *Source) build(scan *scan) *pluginsdk.DiscoveryResult {
	result := &pluginsdk.DiscoveryResult{}
	edges := newEdgeSet()

	jobAssets, jobs, runs := s.pipelineJobAssets(scan.pipelineJobs)
	result.Assets = append(result.Assets, jobAssets...)
	result.RunHistory = append(result.RunHistory, runs...)

	modelAssets, models := s.modelAssets(scan.models, jobs, edges)
	result.Assets = append(result.Assets, modelAssets...)

	endpointAssets := s.endpointAssets(scan.endpoints, models, edges)
	result.Assets = append(result.Assets, endpointAssets...)

	datasetAssets, datasetStats := s.datasetAssets(scan.datasets, edges)
	result.Assets = append(result.Assets, datasetAssets...)
	result.Statistics = append(result.Statistics, datasetStats...)

	groupAssets, groupStats := s.featureGroupAssets(scan.featureGroups, edges)
	result.Assets = append(result.Assets, groupAssets...)
	result.Statistics = append(result.Statistics, groupStats...)

	result.Lineage = edges.all()

	log.Info().
		Int("assets", len(result.Assets)).
		Int("lineages", len(result.Lineage)).
		Int("statistics", len(result.Statistics)).
		Int("run_histories", len(result.RunHistory)).
		Msg("Vertex AI discovery completed")

	return result
}

// assetMRN builds the identity of every asset this plugin owns. The Marmot
// server rebuilds an MRN from an asset's type, provider and name, so every
// MRN and every lineage endpoint has to come from here.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// newAsset fills in the parts every Vertex AI asset shares.
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

// index maps a Vertex resource name to the asset name this run gave it.
// The API refers to resources by their full name, and lineage has to point
// at the name the catalog knows them by.
type index map[string]string

// names decides what to call the assets of one kind. Vertex display names
// are not unique, so a display name that more than one resource shares gets
// its resource id appended, for every resource sharing it. Suffixing only
// the later ones would make a name depend on the order the API listed
// things in.
type names struct {
	shared map[string]bool
}

func newNames(displayNames []string) *names {
	counts := map[string]int{}
	for _, displayName := range displayNames {
		counts[displayName]++
	}

	shared := map[string]bool{}
	for displayName, count := range counts {
		if count > 1 {
			shared[displayName] = true
		}
	}

	return &names{shared: shared}
}

// resolve returns the asset name for one resource.
func (n *names) resolve(displayName, id string) string {
	if displayName == "" {
		return id
	}
	if n.shared[displayName] {
		return fmt.Sprintf("%s (%s)", displayName, id)
	}
	return displayName
}

// resourceName is the project, location and id a fully qualified Vertex
// resource name carries, for example
// projects/acme/locations/us-central1/models/1234567890.
type resourceName struct {
	project  string
	location string
	id       string
}

// parseResourceName splits a fully qualified resource name. The id is the
// last segment, which is what nested resources such as a feature group's
// features are identified by.
func parseResourceName(name string) (resourceName, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 6 || len(parts)%2 != 0 {
		return resourceName{}, false
	}
	if parts[0] != "projects" || parts[2] != "locations" {
		return resourceName{}, false
	}
	for _, part := range parts {
		if part == "" {
			return resourceName{}, false
		}
	}

	return resourceName{project: parts[1], location: parts[3], id: parts[len(parts)-1]}, true
}

// resourceID is the last segment of a resource name, which is how Vertex
// itself refers to a resource once its parent is known.
func resourceID(name string) string {
	if name == "" {
		return ""
	}
	return name[strings.LastIndex(name, "/")+1:]
}

// locate reads the project, location and id out of a resource name and
// falls back to what the run was configured to scan when the name is not
// the shape the API documents.
func (s *Source) locate(name, location string) resourceName {
	parsed, ok := parseResourceName(name)
	if ok {
		return parsed
	}

	log.Debug().Str("resource_name", name).Msg("Unrecognised resource name shape")
	return resourceName{project: s.config.ProjectID, location: location, id: resourceID(name)}
}

// commonMetadata fills the fields every Vertex resource carries.
func commonMetadata(name resourceName, createTime, updateTime string, labels map[string]string) map[string]any {
	metadata := map[string]any{}

	putString(metadata, "resource_id", name.id)
	putString(metadata, "location", name.location)
	putString(metadata, "project_id", name.project)
	putString(metadata, "create_time", createTime)
	putString(metadata, "update_time", updateTime)

	for key, value := range labels {
		if key == "" || value == "" {
			continue
		}
		metadata["label_"+key] = value
	}

	return metadata
}

// bigQueryTable returns the table of a bq://project.dataset.table URI, or
// "" when the value is not one. Vertex also accepts other schemes, and a
// URI naming only a dataset has no table to link to.
func bigQueryTable(uri string) string {
	rest, ok := strings.CutPrefix(uri, "bq://")
	if !ok {
		return ""
	}

	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return ""
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}

	return parts[2]
}

// gcsBucket returns the bucket of a gs:// URI, or "" when the value is not
// one.
func gcsBucket(uri string) string {
	rest, ok := strings.CutPrefix(uri, "gs://")
	if !ok {
		return ""
	}

	bucket, _, _ := strings.Cut(rest, "/")
	return bucket
}

// edgeKey is the identity of a lineage edge. LineageEdge itself holds a
// column-lineage slice, which makes it unusable as a map key.
type edgeKey struct {
	source, target, edgeType string
}

// edgeSet collects lineage edges without duplicates, keeping the order they
// were added in so a run's output does not shuffle between runs.
type edgeSet struct {
	seen  map[edgeKey]bool
	edges []pluginsdk.LineageEdge
}

func newEdgeSet() *edgeSet {
	return &edgeSet{seen: map[edgeKey]bool{}}
}

func (e *edgeSet) add(source, target, edgeType string) {
	if source == "" || target == "" {
		return
	}

	key := edgeKey{source, target, edgeType}
	if e.seen[key] {
		return
	}

	e.seen[key] = true
	e.edges = append(e.edges, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

func (e *edgeSet) all() []pluginsdk.LineageEdge {
	return e.edges
}

// putString records a value only when it is set, so no empty keys reach the
// catalog.
func putString(m map[string]any, key, value string) {
	if value == "" {
		return
	}
	m[key] = value
}

// putStrings records a list only when it has entries.
func putStrings(m map[string]any, key string, values []string) {
	if len(values) == 0 {
		return
	}
	m[key] = values
}

// putBool records a flag only when it is set. The API omits a false
// boolean, so recording one would claim knowledge the response does not
// carry.
func putBool(m map[string]any, key string, value bool) {
	if !value {
		return
	}
	m[key] = true
}
