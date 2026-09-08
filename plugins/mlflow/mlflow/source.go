// Package mlflow discovers registered models, experiments and the datasets
// behind them from MLflow tracking servers.
package mlflow

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string on every asset this plugin emits.
const provider = "MLflow"

// Config for the MLflow plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	TrackingURI string `json:"tracking_uri" label:"Tracking URI" description:"MLflow tracking server URL, also used as the model registry" validate:"required,url"`
	Username    string `json:"username,omitempty" description:"Username for basic authentication"`
	Password    string `json:"password,omitempty" description:"Password for basic authentication" sensitive:"true"`
	Token       string `json:"token,omitempty" description:"Bearer token for authentication" sensitive:"true"`
	VerifySSL   bool   `json:"verify_ssl" label:"Verify SSL" description:"Verify the server TLS certificate" default:"true"`

	IncludeExperiments bool `json:"include_experiments" description:"Discover experiments as assets" default:"true"`
	IncludeDatasets    bool `json:"include_datasets" description:"Discover the datasets logged to each model's run" default:"true"`
	IncludeMetrics     bool `json:"include_metrics" description:"Record the run metrics of each model" default:"true"`
	MaxModels          int  `json:"max_models" description:"Maximum number of registered models to discover (0 = unlimited)" default:"0" validate:"omitempty,min=0"`
}

// Example configuration for the plugin
var _ = `
tracking_uri: "https://mlflow.company.com"
username: "marmot"
password: "mlflow_secure_pass"
include_experiments: true
include_datasets: true
include_metrics: true
tags:
  - "mlflow"
  - "ml-platform"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "mlflow",
		Name:        "MLflow",
		Description: "Discover registered models, experiments and training datasets from MLflow tracking servers",
		Icon:        "mlflow",
		Category:    "ml",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the MLflow plugin.
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

	config.TrackingURI = strings.TrimSuffix(strings.TrimSpace(config.TrackingURI), "/")

	if config.Token != "" && config.Username != "" {
		return nil, fmt.Errorf("provide either username and password or token, not both")
	}
	if config.Password != "" && config.Username == "" {
		return nil, fmt.Errorf("password requires a username")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers registered models, experiments and datasets.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	d := &discovery{
		config:      s.config,
		client:      newClient(s.config),
		experiments: make(map[string]experiment),
		datasets:    make(map[string]int),
		edges:       make(map[string]struct{}),
	}

	log.Debug().Str("tracking_uri", s.config.TrackingURI).Msg("Starting MLflow discovery")

	// The registry is the reason the plugin exists, so an unreachable one
	// fails the run. Everything after it degrades to a warning.
	models, err := d.client.searchRegisteredModels(ctx, s.config.MaxModels)
	if err != nil {
		return nil, fmt.Errorf("listing registered models: %w", err)
	}
	log.Debug().Int("count", len(models)).Msg("Found registered models")

	// Experiments are listed even when they are not wanted as assets, so
	// a model can still name the experiment it came from.
	experiments, err := d.client.searchExperiments(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list experiments")
	}
	for _, e := range experiments {
		if e.LifecycleStage == "deleted" {
			continue
		}
		d.experiments[e.ExperimentID] = e
		if s.config.IncludeExperiments {
			d.assets = append(d.assets, d.experimentAsset(e))
		}
	}

	for _, m := range models {
		d.discoverModel(ctx, m)
	}

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Int("statistics", len(d.statistics)).
		Msg("MLflow discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     d.assets,
		Lineage:    d.lineage,
		Statistics: d.statistics,
	}, nil
}

// discovery accumulates one run's output. Two models trained on the same
// data share one Dataset asset, so datasets and edges are deduplicated
// here rather than emitted twice.
type discovery struct {
	config *Config
	client *client

	// experiments holds every active experiment by id.
	experiments map[string]experiment

	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// datasets maps a dataset name to its index in assets.
	datasets map[string]int
	edges    map[string]struct{}
}

func (d *discovery) link(source, target, edgeType string) {
	key := source + "|" + target + "|" + edgeType
	if _, seen := d.edges[key]; seen {
		return
	}
	d.edges[key] = struct{}{}
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// discoverModel emits one registered model, its features, its lineage to
// the experiment and datasets behind its newest version, and its metrics.
func (d *discovery) discoverModel(ctx context.Context, m registeredModel) {
	versions := d.modelVersions(ctx, m)
	latest := newestVersion(versions)

	var r *run
	if latest != nil && latest.RunID != "" {
		var err error
		r, err = d.client.getRun(ctx, latest.RunID)
		if err != nil {
			log.Warn().Err(err).Str("model", m.Name).Str("run_id", latest.RunID).Msg("Failed to fetch the run behind the latest model version")
		}
	}

	asset := d.modelAsset(m, latest, len(versions), r)
	if columns := d.signatureColumns(ctx, latest, r); len(columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, columns); err != nil {
			log.Warn().Err(err).Str("model", m.Name).Msg("Failed to set model features")
		}
	}
	d.assets = append(d.assets, asset)
	modelMRN := *asset.MRN

	if r == nil {
		return
	}

	if d.config.IncludeExperiments {
		if exp, ok := d.experiments[r.Info.ExperimentID]; ok {
			d.link(assetMRN("Experiment", exp.Name), modelMRN, "PRODUCES")
		}
	}

	if d.config.IncludeMetrics {
		for _, metric := range r.Data.Metrics {
			if !metric.Value.finite() {
				continue
			}
			d.statistics = append(d.statistics, pluginsdk.Statistic{
				AssetMRN:   modelMRN,
				MetricName: "asset.metric." + metric.Key,
				Value:      float64(metric.Value),
			})
		}
	}

	if d.config.IncludeDatasets {
		for _, input := range r.Inputs.DatasetInputs {
			datasetMRN := d.addDataset(input)
			if datasetMRN == "" {
				continue
			}
			d.link(datasetMRN, modelMRN, "FEEDS")
		}
	}
}

// modelVersions returns every version of a model. The registry's own
// search is the source of truth; if it fails the model's latest_versions
// (one per stage) still identify the newest version.
func (d *discovery) modelVersions(ctx context.Context, m registeredModel) []modelVersion {
	versions, err := d.client.searchModelVersions(ctx, m.Name)
	if err != nil {
		log.Warn().Err(err).Str("model", m.Name).Msg("Failed to search model versions, using the registry's latest versions")
		return m.LatestVersions
	}
	if len(versions) == 0 {
		return m.LatestVersions
	}
	return versions
}

// newestVersion picks the highest numbered version.
func newestVersion(versions []modelVersion) *modelVersion {
	var newest *modelVersion
	for i := range versions {
		if newest == nil || versions[i].versionNumber() > newest.versionNumber() {
			newest = &versions[i]
		}
	}
	return newest
}

func (d *discovery) modelAsset(m registeredModel, latest *modelVersion, versionCount int, r *run) pluginsdk.Asset {
	metadata := map[string]any{
		"created_at":    formatMillis(m.CreationTimestamp),
		"updated_at":    formatMillis(m.LastUpdatedTimestamp),
		"version_count": versionCount,
		"url":           d.modelURL(m.Name),
	}
	putIf(metadata, "description", m.Description)
	if tags := keyValueMap(m.Tags); len(tags) > 0 {
		metadata["tags"] = tags
	}
	if len(m.Aliases) > 0 {
		aliases := make(map[string]any, len(m.Aliases))
		for _, a := range m.Aliases {
			aliases[a.Alias] = a.Version
		}
		metadata["aliases"] = aliases
	}

	if latest != nil {
		metadata["latest_version"] = latest.Version
		if latest.CurrentStage != "" && latest.CurrentStage != "None" {
			metadata["stage"] = latest.CurrentStage
		}
		putIf(metadata, "run_id", latest.RunID)
		putIf(metadata, "status", latest.Status)
		putIf(metadata, "artifact_uri", latest.Source)
	}

	if r != nil {
		putIf(metadata, "run_name", r.Info.RunName)
		putIf(metadata, "experiment_id", r.Info.ExperimentID)
		if exp, ok := d.experiments[r.Info.ExperimentID]; ok {
			metadata["experiment"] = exp.Name
		}
		putIf(metadata, "artifact_uri", r.Info.ArtifactURI)
		if params := keyValueMap(r.Data.Params); len(params) > 0 {
			metadata["hyperparameters"] = params
		}
		if d.config.IncludeMetrics {
			if metrics := metricMap(r.Data.Metrics); len(metrics) > 0 {
				metadata["metrics"] = metrics
			}
		}
	}

	name := m.Name
	asset := d.newAsset("Model", name, metadata)
	if m.Description != "" {
		description := m.Description
		asset.Description = &description
	}
	asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in MLflow", URL: d.modelURL(name)}}
	return asset
}

func (d *discovery) experimentAsset(e experiment) pluginsdk.Asset {
	metadata := map[string]any{
		"experiment_id":   e.ExperimentID,
		"lifecycle_stage": e.LifecycleStage,
		"created_at":      formatMillis(e.CreationTime),
		"updated_at":      formatMillis(e.LastUpdateTime),
		"url":             d.experimentURL(e.ExperimentID),
	}
	putIf(metadata, "artifact_location", e.ArtifactLocation)
	if tags := keyValueMap(e.Tags); len(tags) > 0 {
		metadata["tags"] = tags
	}

	asset := d.newAsset("Experiment", e.Name, metadata)
	// The description typed into the MLflow UI is stored as a tag.
	for _, t := range e.Tags {
		if t.Key == "mlflow.note.content" && t.Value != "" {
			description := t.Value
			asset.Description = &description
		}
	}
	asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in MLflow", URL: d.experimentURL(e.ExperimentID)}}
	return asset
}

// newAsset builds an asset with the fields every kind shares.
func (d *discovery) newAsset(assetType, name string, metadata map[string]any) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)
	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(d.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (d *discovery) modelURL(name string) string {
	return d.config.TrackingURI + "/#/models/" + url.PathEscape(name)
}

func (d *discovery) experimentURL(id string) string {
	return d.config.TrackingURI + "/#/experiments/" + url.PathEscape(id)
}

// assetMRN is the single place an MLflow MRN is built, so assets and the
// edges between them can never drift into addressing one thing two ways.
// Every kind is addressed by its own MLflow name: model names are unique
// per registry, and experiment and dataset names per tracking server.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// assetMRNFor addresses an asset another plugin owns, with that plugin's
// exact provider string, so an edge to it lands on the asset that plugin
// creates rather than on a duplicate.
func assetMRNFor(assetType, otherProvider, name string) string {
	return mrn.New(assetType, otherProvider, name)
}

func putIf(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

func keyValueMap(pairs []keyValue) map[string]any {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]any, len(pairs))
	for _, p := range pairs {
		out[p.Key] = p.Value
	}
	return out
}

// metricMap keeps the latest value of every metric. NaN and infinity
// cannot travel as JSON numbers, so they are recorded by name instead.
func metricMap(metrics []metric) map[string]any {
	if len(metrics) == 0 {
		return nil
	}
	out := make(map[string]any, len(metrics))
	for _, m := range metrics {
		if m.Value.finite() {
			out[m.Key] = float64(m.Value)
		} else {
			out[m.Key] = fmt.Sprint(float64(m.Value))
		}
	}
	return out
}

func formatMillis(millis int64) string {
	return time.UnixMilli(millis).UTC().Format(time.RFC3339)
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
