package mlflow_test

import (
	"encoding/json"
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real MLflow tracking server
// started with --serve-artifacts and seeded over its REST API with:
// experiments churn and fraud plus a deleted one named old; registered
// models churn-predictor (one version, alias champion, a run logging
// max_depth, accuracy 0.91, a log-model history signature with age and
// plan, and the S3 dataset customers), fraud-detector (two versions, the
// newest with an MLmodel file only and the GCS dataset transactions),
// fraud-lm-reg (a models:/ source pointing at a logged model) and
// "it's a model" (a run with a NaN metric).

func trackingURI(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("MARMOT_TEST_MLFLOW_URL")
	if uri == "" {
		t.Skip("MARMOT_TEST_MLFLOW_URL not set, skipping MLflow e2e tests")
	}
	return uri
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		a := &result.Assets[i]
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]pluginsdk.Column {
	t.Helper()
	var columns []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(a.Schema["columns"]), &columns))
	byName := make(map[string]pluginsdk.Column, len(columns))
	for _, c := range columns {
		byName[c.Name] = c
	}
	return byName
}

func TestE2E_Meta(t *testing.T) {
	trackingURI(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "mlflow", meta.ID)
	assert.Equal(t, "MLflow", meta.Name)
	assert.Equal(t, "ml", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingTrackingURIFails(t *testing.T) {
	trackingURI(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheRealServer(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)
}

func TestE2E_DiscoverModelsOverTheWire(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)
	require.NotNil(t, result)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, "mrn://model/mlflow/churn-predictor", *churn.MRN)
	assert.Equal(t, []string{"MLflow"}, churn.Providers)
	require.NotNil(t, churn.Description)
	assert.Equal(t, "Predicts churn", *churn.Description)
	assert.Equal(t, map[string]any{"team": "growth"}, churn.Metadata["tags"])
	assert.Equal(t, map[string]any{"champion": "1"}, churn.Metadata["aliases"])
	assert.Equal(t, "1", churn.Metadata["latest_version"])
	assert.Equal(t, float64(1), churn.Metadata["version_count"], "ints travel as JSON numbers")
	assert.Equal(t, "baseline", churn.Metadata["run_name"])
	assert.Equal(t, "churn", churn.Metadata["experiment"])
	assert.Equal(t, uri+"/#/models/churn-predictor", churn.Metadata["url"])

	params, ok := churn.Metadata["hyperparameters"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "6", params["max_depth"])

	metrics, ok := churn.Metadata["metrics"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0.91, metrics["accuracy"])

	columns := columnsOf(t, churn)
	require.Len(t, columns, 2)
	assert.Equal(t, "long", columns["age"].DataType)
	assert.False(t, columns["age"].Nullable)
	assert.Equal(t, "string", columns["plan"].DataType)
	assert.True(t, columns["plan"].Nullable)

	require.Len(t, churn.ExternalLinks, 1)
	assert.Equal(t, "Open in MLflow", churn.ExternalLinks[0].Name)
}

func TestE2E_DiscoverPicksTheNewestOfSeveralVersions(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)

	fraud := findAsset(result, "Model", "fraud-detector")
	require.NotNil(t, fraud)
	assert.Equal(t, "2", fraud.Metadata["latest_version"])
	assert.Equal(t, float64(2), fraud.Metadata["version_count"])
	assert.Equal(t, "fraud-v2", fraud.Metadata["run_name"])

	// This run logged no model history tag, so the features come from
	// the MLmodel file in the run's artifacts.
	columns := columnsOf(t, fraud)
	require.Len(t, columns, 2)
	assert.Equal(t, "double", columns["amount"].DataType)
	assert.True(t, columns["merchant"].Nullable)
}

func TestE2E_DiscoverFollowsAModelsSourceToTheLoggedModel(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)

	reg := findAsset(result, "Model", "fraud-lm-reg")
	require.NotNil(t, reg)
	assert.Contains(t, reg.Metadata["artifact_uri"], "mlflow-artifacts:/")

	columns := columnsOf(t, reg)
	require.Len(t, columns, 2)
	assert.Equal(t, "double", columns["amount"].DataType)
	assert.Equal(t, "string", columns["merchant"].DataType)
}

func TestE2E_DiscoverEmitsMetricStatistics(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)

	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == "mrn://model/mlflow/churn-predictor" {
			stats[st.MetricName] = st.Value
		}
	}
	assert.Equal(t, 0.91, stats["asset.metric.accuracy"])
	assert.Equal(t, 0.87, stats["asset.metric.f1"])

	// The run behind "it's a model" logged a NaN metric, which must not
	// reach the statistics.
	for _, st := range result.Statistics {
		assert.NotEqual(t, "asset.metric.nan_metric", st.MetricName)
	}
}

func TestE2E_DiscoverExperimentsOverTheWire(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)

	churn := findAsset(result, "Experiment", "churn")
	require.NotNil(t, churn)
	assert.Equal(t, "mrn://experiment/mlflow/churn", *churn.MRN)
	assert.Equal(t, "active", churn.Metadata["lifecycle_stage"])
	assert.NotEmpty(t, churn.Metadata["experiment_id"])
	assert.Equal(t, uri+"/#/experiments/"+churn.Metadata["experiment_id"].(string), churn.Metadata["url"])

	fraud := findAsset(result, "Experiment", "fraud")
	require.NotNil(t, fraud)
	assert.Equal(t, map[string]any{"owner": "risk"}, fraud.Metadata["tags"])

	assert.Nil(t, findAsset(result, "Experiment", "old"), "deleted experiments stay out")

	assert.True(t, hasEdge(result, "mrn://experiment/mlflow/churn", "mrn://model/mlflow/churn-predictor", "PRODUCES"))
	assert.True(t, hasEdge(result, "mrn://experiment/mlflow/fraud", "mrn://model/mlflow/fraud-detector", "PRODUCES"))
}

func TestE2E_DiscoverDatasetsOverTheWire(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": uri})
	require.NoError(t, err)

	customers := findAsset(result, "Dataset", "customers")
	require.NotNil(t, customers)
	assert.Equal(t, "mrn://dataset/mlflow/customers", *customers.MRN)
	assert.Equal(t, "abc123", customers.Metadata["digest"])
	assert.Equal(t, "s3", customers.Metadata["source_type"])
	assert.Equal(t, "training", customers.Metadata["context"])
	assert.Equal(t, map[string]any{"uri": "s3://ml-data/customers.parquet"}, customers.Metadata["source"])

	columns := columnsOf(t, customers)
	require.Len(t, columns, 2)
	assert.Equal(t, "long", columns["age"].DataType)
	assert.True(t, columns["plan"].Nullable)

	assert.True(t, hasEdge(result, "mrn://dataset/mlflow/customers", "mrn://model/mlflow/churn-predictor", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-data", "mrn://dataset/mlflow/customers", "FEEDS"))
	assert.Nil(t, findAsset(result, "Bucket", "ml-data"))

	// The seed logs transactions twice to the same run, but MLflow keeps
	// one input per (name, digest) and with it only the first context.
	transactions := findAsset(result, "Dataset", "transactions")
	require.NotNil(t, transactions)
	assert.Equal(t, "training", transactions.Metadata["context"])
	assert.True(t, hasEdge(result, "mrn://bucket/gcs/fraud-data", "mrn://dataset/mlflow/transactions", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://dataset/mlflow/transactions", "mrn://model/mlflow/fraud-detector", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://dataset/mlflow/transactions", "mrn://model/mlflow/fraud-lm-reg", "FEEDS"))
}

func TestE2E_DiscoverHonoursTheIncludeToggles(t *testing.T) {
	uri := trackingURI(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{
		"tracking_uri":        uri,
		"include_experiments": false,
		"include_datasets":    false,
		"include_metrics":     false,
		"max_models":          1,
	})
	require.NoError(t, err)

	var models int
	for _, a := range result.Assets {
		assert.Equal(t, "Model", a.Type)
		models++
	}
	assert.Equal(t, 1, models)
	assert.Empty(t, result.Lineage)
	assert.Empty(t, result.Statistics)
}
