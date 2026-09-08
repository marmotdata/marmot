package mlflow

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_EmitsAModelPerRegisteredModel(t *testing.T) {
	result := discover(t, seeded(), nil)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, []string{"MLflow"}, churn.Providers)
	assert.NotNil(t, findAsset(result, "Model", "fraud-detector"))
	assert.NotNil(t, findAsset(result, "Model", "fraud-lm-reg"))
}

func TestDiscover_ModelCarriesRegistryMetadata(t *testing.T) {
	result := discover(t, seeded(), nil)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	require.NotNil(t, churn.Description)
	assert.Equal(t, "Predicts churn", *churn.Description)
	assert.Equal(t, "Predicts churn", churn.Metadata["description"])
	assert.Equal(t, map[string]any{"team": "growth"}, churn.Metadata["tags"])
	assert.Equal(t, map[string]any{"champion": "1"}, churn.Metadata["aliases"])
	assert.Equal(t, "1", churn.Metadata["latest_version"])
	assert.Equal(t, 1, churn.Metadata["version_count"])
	assert.Equal(t, "READY", churn.Metadata["status"])
	assert.Equal(t, "2026-09-07T21:11:05Z", churn.Metadata["created_at"])
	assert.NotContains(t, churn.Metadata, "stage", "a stage of None is not a stage")
}

func TestDiscover_ModelCarriesRunMetadata(t *testing.T) {
	result := discover(t, seeded(), nil)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, churnRunID, churn.Metadata["run_id"])
	assert.Equal(t, "baseline", churn.Metadata["run_name"])
	assert.Equal(t, "1", churn.Metadata["experiment_id"])
	assert.Equal(t, "churn", churn.Metadata["experiment"])
	assert.Equal(t, "mlflow-artifacts:/1/"+churnRunID+"/artifacts", churn.Metadata["artifact_uri"])
	assert.Equal(t, map[string]any{"max_depth": "6", "learning_rate": "0.1"}, churn.Metadata["hyperparameters"])
	assert.Equal(t, map[string]any{"accuracy": 0.91, "f1": 0.87}, churn.Metadata["metrics"])
}

func TestDiscover_ModelLinksBackToTheMLflowUI(t *testing.T) {
	f := seeded()
	server := f.start(t)
	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": server.URL + "/"})
	require.NoError(t, err)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, server.URL+"/#/models/churn-predictor", churn.Metadata["url"])
	require.Len(t, churn.ExternalLinks, 1)
	assert.Equal(t, "Open in MLflow", churn.ExternalLinks[0].Name)
	assert.Equal(t, server.URL+"/#/models/churn-predictor", churn.ExternalLinks[0].URL)
}

func TestDiscover_ModelFeaturesComeFromTheLogModelHistoryTag(t *testing.T) {
	f := seeded()
	result := discover(t, f, nil)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	columns := columnsOf(t, churn)
	require.Len(t, columns, 2)
	assert.Equal(t, "long", columns["age"].DataType)
	assert.False(t, columns["age"].Nullable)
	assert.Equal(t, "string", columns["plan"].DataType)
	assert.True(t, columns["plan"].Nullable)

	assert.Empty(t, f.requestsTo("/get-artifact?path=model%2FMLmodel&run_uuid="+churnRunID), "the tag makes the artifact read unnecessary")
}

func TestDiscover_ModelFeaturesFallBackToTheRunsMLmodelFile(t *testing.T) {
	f := seeded()
	result := discover(t, f, nil)

	fraud := findAsset(result, "Model", "fraud-detector")
	require.NotNil(t, fraud)
	columns := columnsOf(t, fraud)
	require.Len(t, columns, 2)
	assert.Equal(t, "double", columns["amount"].DataType)
	assert.False(t, columns["amount"].Nullable)
	assert.True(t, columns["merchant"].Nullable)

	assert.Len(t, f.requestsTo("/get-artifact?path=model%2FMLmodel&run_uuid="+fraudRunID), 1)
}

func TestDiscover_ModelFeaturesFollowAModelsSourceToTheLoggedModel(t *testing.T) {
	f := seeded()
	result := discover(t, f, nil)

	reg := findAsset(result, "Model", "fraud-lm-reg")
	require.NotNil(t, reg)
	columns := columnsOf(t, reg)
	require.Len(t, columns, 2)
	assert.Equal(t, "double", columns["amount"].DataType)

	assert.Len(t, f.requestsTo("/api/2.0/mlflow/logged-models/"+loggedModelID), 1)
	assert.Len(t, f.requestsTo("/api/2.0/mlflow-artifacts/artifacts/2/models/"+loggedModelID+"/artifacts/MLmodel"), 1)
}

func TestDiscover_EmitsNoFeaturesWhenNoSignatureIsFound(t *testing.T) {
	f := seeded()
	delete(f.runArtifacts, fraudRunID+"|model/MLmodel")
	result := discover(t, f, nil)

	fraud := findAsset(result, "Model", "fraud-detector")
	require.NotNil(t, fraud)
	assert.NotContains(t, fraud.Schema, "columns")
}

func TestDiscover_PicksTheHighestVersionAsLatest(t *testing.T) {
	result := discover(t, seeded(), nil)

	fraud := findAsset(result, "Model", "fraud-detector")
	require.NotNil(t, fraud)
	assert.Equal(t, "2", fraud.Metadata["latest_version"])
	assert.Equal(t, 2, fraud.Metadata["version_count"])
	assert.Equal(t, fraudRunID, fraud.Metadata["run_id"])
}

func TestDiscover_FallsBackToTheRegistrysLatestVersionsWhenTheSearchFails(t *testing.T) {
	f := seeded()
	f.versionSearchFails = true
	result := discover(t, f, nil)

	fraud := findAsset(result, "Model", "fraud-detector")
	require.NotNil(t, fraud)
	assert.Equal(t, "2", fraud.Metadata["latest_version"])
	assert.Equal(t, 1, fraud.Metadata["version_count"], "only the versions the registry listed are known")
}

func TestDiscover_EmitsAStatisticPerMetric(t *testing.T) {
	result := discover(t, seeded(), nil)

	stats := statisticsOf(result, assetMRN("Model", "churn-predictor"))
	assert.Equal(t, 0.91, stats["asset.metric.accuracy"])
	assert.Equal(t, 0.87, stats["asset.metric.f1"])
}

func TestDiscover_KeepsANaNMetricOutOfStatisticsButNamesItInMetadata(t *testing.T) {
	result := discover(t, seeded(), nil)

	fraud := findAsset(result, "Model", "fraud-detector")
	require.NotNil(t, fraud)
	assert.Equal(t, map[string]any{"auc": 0.97, "nan_metric": "NaN"}, fraud.Metadata["metrics"])

	stats := statisticsOf(result, *fraud.MRN)
	assert.Equal(t, map[string]float64{"asset.metric.auc": 0.97}, stats)
}

func TestDiscover_IncludeMetricsFalseDropsMetricsAndStatistics(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"include_metrics": false})

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.NotContains(t, churn.Metadata, "metrics")
	assert.Equal(t, map[string]any{"max_depth": "6", "learning_rate": "0.1"}, churn.Metadata["hyperparameters"],
		"hyperparameters are not metrics")
	assert.Empty(t, result.Statistics)
}

func TestDiscover_EmitsExperiments(t *testing.T) {
	f := seeded()
	server := f.start(t)
	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": server.URL})
	require.NoError(t, err)

	fraud := findAsset(result, "Experiment", "fraud")
	require.NotNil(t, fraud)
	assert.Equal(t, []string{"MLflow"}, fraud.Providers)
	assert.Equal(t, "2", fraud.Metadata["experiment_id"])
	assert.Equal(t, "mlflow-artifacts:/2", fraud.Metadata["artifact_location"])
	assert.Equal(t, "active", fraud.Metadata["lifecycle_stage"])
	assert.Equal(t, map[string]any{"owner": "risk"}, fraud.Metadata["tags"])
	assert.Equal(t, "2026-09-07T21:11:41Z", fraud.Metadata["created_at"])
	assert.Equal(t, server.URL+"/#/experiments/2", fraud.Metadata["url"])
	require.Len(t, fraud.ExternalLinks, 1)
	assert.Equal(t, server.URL+"/#/experiments/2", fraud.ExternalLinks[0].URL)

	assert.NotNil(t, findAsset(result, "Experiment", "churn"))
	assert.NotNil(t, findAsset(result, "Experiment", "Default"))
}

func TestDiscover_UsesTheNoteTagAsTheExperimentDescription(t *testing.T) {
	f := newFakeMLflow().withExperiments(`[{
		"experiment_id": "7", "name": "pricing", "artifact_location": "mlflow-artifacts:/7",
		"lifecycle_stage": "active", "creation_time": 1788815429950, "last_update_time": 1788815429950,
		"tags": [{"key": "mlflow.note.content", "value": "Price elasticity models"}]
	}]`)
	result := discover(t, f, nil)

	pricing := findAsset(result, "Experiment", "pricing")
	require.NotNil(t, pricing)
	require.NotNil(t, pricing.Description)
	assert.Equal(t, "Price elasticity models", *pricing.Description)
}

func TestDiscover_SkipsDeletedExperiments(t *testing.T) {
	f := seeded().withExperiments(`[{
		"experiment_id": "3", "name": "old", "artifact_location": "mlflow-artifacts:/3",
		"lifecycle_stage": "deleted", "creation_time": 1788815526691, "last_update_time": 1788815526928
	}]`)
	result := discover(t, f, nil)

	assert.Nil(t, findAsset(result, "Experiment", "old"))
}

func TestDiscover_LinksTheExperimentToTheModelsItProduced(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.True(t, hasEdge(result, "mrn://experiment/mlflow/churn", "mrn://model/mlflow/churn-predictor", "PRODUCES"))
	assert.True(t, hasEdge(result, "mrn://experiment/mlflow/fraud", "mrn://model/mlflow/fraud-detector", "PRODUCES"))
	assert.True(t, hasEdge(result, "mrn://experiment/mlflow/fraud", "mrn://model/mlflow/fraud-lm-reg", "PRODUCES"))
}

func TestDiscover_IncludeExperimentsFalseDropsExperimentsButKeepsTheName(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"include_experiments": false})

	assert.Nil(t, findAsset(result, "Experiment", "churn"))
	for _, edge := range result.Lineage {
		assert.NotEqual(t, "PRODUCES", edge.Type, "no experiment asset exists for the edge to start from")
	}

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, "churn", churn.Metadata["experiment"])
}

func TestDiscover_EmitsTheDatasetsBehindAModelsRun(t *testing.T) {
	result := discover(t, seeded(), nil)

	customers := findAsset(result, "Dataset", "customers")
	require.NotNil(t, customers)
	assert.Equal(t, []string{"MLflow"}, customers.Providers)
	assert.Equal(t, "abc123", customers.Metadata["digest"])
	assert.Equal(t, "s3", customers.Metadata["source_type"])
	assert.Equal(t, "training", customers.Metadata["context"])
	assert.Equal(t, map[string]any{"uri": "s3://ml-data/customers.parquet"}, customers.Metadata["source"])
	assert.Equal(t, map[string]any{"num_rows": float64(1200), "num_elements": float64(2400)}, customers.Metadata["profile"])
}

func TestDiscover_DatasetColumnsComeFromTheLoggedSchema(t *testing.T) {
	result := discover(t, seeded(), nil)

	customers := findAsset(result, "Dataset", "customers")
	require.NotNil(t, customers)
	columns := columnsOf(t, customers)
	require.Len(t, columns, 2)
	assert.Equal(t, "long", columns["age"].DataType)
	assert.False(t, columns["age"].Nullable)
	assert.True(t, columns["plan"].Nullable)
}

func TestDiscover_LinksDatasetsToTheModelsTheyFeed(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.True(t, hasEdge(result, "mrn://dataset/mlflow/customers", "mrn://model/mlflow/churn-predictor", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://dataset/mlflow/transactions", "mrn://model/mlflow/fraud-detector", "FEEDS"))
}

func TestDiscover_LinksAnS3BucketToTheDatasetReadFromIt(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-data", "mrn://dataset/mlflow/customers", "FEEDS"))
	assert.Nil(t, findAsset(result, "Bucket", "ml-data"), "the bucket belongs to the S3 plugin")
}

func TestDiscover_LinksAGCSBucketToTheDatasetReadFromIt(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.True(t, hasEdge(result, "mrn://bucket/gcs/fraud-data", "mrn://dataset/mlflow/transactions", "FEEDS"))
	assert.Nil(t, findAsset(result, "Bucket", "fraud-data"))
}

func TestDiscover_SharesOneDatasetBetweenModelsAndMergesItsContexts(t *testing.T) {
	// fraud-detector and fraud-lm-reg come from the run that trained on
	// transactions; fraud-eval comes from a run that evaluated on it.
	f := seeded().withModel(evalModelJSON, evalVersionJSON).withRun("evalrun", evalRunJSON)
	result := discover(t, f, nil)

	var count int
	for _, a := range result.Assets {
		if a.Type == "Dataset" && *a.Name == "transactions" {
			count++
		}
	}
	assert.Equal(t, 1, count)

	transactions := findAsset(result, "Dataset", "transactions")
	require.NotNil(t, transactions)
	assert.Equal(t, "eval, training", transactions.Metadata["context"])
	assert.True(t, hasEdge(result, *transactions.MRN, "mrn://model/mlflow/fraud-detector", "FEEDS"))
	assert.True(t, hasEdge(result, *transactions.MRN, "mrn://model/mlflow/fraud-lm-reg", "FEEDS"))
	assert.True(t, hasEdge(result, *transactions.MRN, "mrn://model/mlflow/fraud-eval", "FEEDS"))
}

func TestDiscover_IncludeDatasetsFalseDropsDatasetsAndTheirEdges(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"include_datasets": false})

	assert.Nil(t, findAsset(result, "Dataset", "customers"))
	for _, edge := range result.Lineage {
		assert.NotEqual(t, "FEEDS", edge.Type)
	}
}

func TestDiscover_FollowsPageTokensToTheEnd(t *testing.T) {
	f := seeded()
	f.pageSize = 1
	result := discover(t, f, nil)

	assert.NotNil(t, findAsset(result, "Model", "churn-predictor"))
	assert.NotNil(t, findAsset(result, "Model", "fraud-detector"))
	assert.NotNil(t, findAsset(result, "Model", "fraud-lm-reg"))
	assert.Len(t, f.requestsTo("/api/2.0/mlflow/registered-models/search"), 3)
}

func TestDiscover_MaxModelsStopsListing(t *testing.T) {
	f := seeded()
	f.pageSize = 1
	result := discover(t, f, pluginsdk.RawConfig{"max_models": 2})

	var models int
	for _, a := range result.Assets {
		if a.Type == "Model" {
			models++
		}
	}
	assert.Equal(t, 2, models)
	assert.Len(t, f.requestsTo("/api/2.0/mlflow/registered-models/search"), 2, "listing stops once the cap is reached")
}

func TestDiscover_SendsBasicAuth(t *testing.T) {
	f := seeded()
	discover(t, f, pluginsdk.RawConfig{"username": "marmot", "password": "s3cret"})

	assert.Equal(t, "Basic bWFybW90OnMzY3JldA==", f.lastAuth())
}

func TestDiscover_SendsABearerToken(t *testing.T) {
	f := seeded()
	discover(t, f, pluginsdk.RawConfig{"token": "tok-123"})

	assert.Equal(t, "Bearer tok-123", f.lastAuth())
}

func TestDiscover_SendsNoAuthByDefault(t *testing.T) {
	f := seeded()
	discover(t, f, nil)

	assert.Empty(t, f.lastAuth())
}

func TestDiscover_QuotesAModelNameHoldingAnApostropheWithDoubleQuotes(t *testing.T) {
	f := newFakeMLflow().withModel(`{"name": "it's a model", "creation_timestamp": 1, "last_updated_timestamp": 1}`)
	discover(t, f, nil)

	require.Len(t, f.requestsTo("/api/2.0/mlflow/model-versions/search"), 1)
	assert.Contains(t, f.requestsTo("/api/2.0/mlflow/model-versions/search")[0], "filter=name%3D%22it%27s+a+model%22")
}

func TestDiscover_KeepsAModelWhoseRunIsGone(t *testing.T) {
	f := seeded()
	delete(f.runs, churnRunID)
	result := discover(t, f, nil)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, churnRunID, churn.Metadata["run_id"])
	assert.Equal(t, "runs:/"+churnRunID+"/model", churn.Metadata["artifact_uri"],
		"without the run, the version source is the best location known")
	assert.NotContains(t, churn.Metadata, "hyperparameters")
	assert.False(t, hasEdge(result, "mrn://experiment/mlflow/churn", *churn.MRN, "PRODUCES"))
}

func TestDiscover_KeepsAModelWithoutVersions(t *testing.T) {
	f := seeded().withModel(`{"name": "empty", "creation_timestamp": 1788815465218, "last_updated_timestamp": 1788815465218}`)
	result := discover(t, f, nil)

	empty := findAsset(result, "Model", "empty")
	require.NotNil(t, empty)
	assert.Equal(t, 0, empty.Metadata["version_count"])
	assert.NotContains(t, empty.Metadata, "latest_version")
}

func TestDiscover_ReportsAStageWhenTheLatestVersionHasOne(t *testing.T) {
	f := newFakeMLflow().withModel(
		`{"name": "staged", "creation_timestamp": 1, "last_updated_timestamp": 1}`,
		`{"name": "staged", "version": "3", "current_stage": "Production", "source": "s3://models/staged/3", "status": "READY"}`,
	)
	result := discover(t, f, nil)

	staged := findAsset(result, "Model", "staged")
	require.NotNil(t, staged)
	assert.Equal(t, "Production", staged.Metadata["stage"])
	assert.Equal(t, "s3://models/staged/3", staged.Metadata["artifact_uri"])
}

func TestDiscover_InterpolatesConfiguredTags(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"tags": []string{"mlflow", "exp:${experiment}"}})

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, []string{"mlflow", "exp:churn"}, churn.Tags)
}

func TestDiscover_UnreachableServerFails(t *testing.T) {
	server := newFakeMLflow().start(t)
	server.Close()

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"tracking_uri": server.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing registered models")
}

func TestDiscover_SurvivesAFailingExperimentListing(t *testing.T) {
	f := seeded()
	f.experimentSearchFails = true
	result := discover(t, f, nil)

	churn := findAsset(result, "Model", "churn-predictor")
	require.NotNil(t, churn)
	assert.Equal(t, "1", churn.Metadata["experiment_id"])
	assert.NotContains(t, churn.Metadata, "experiment", "the name is unknown without the listing")
	assert.Nil(t, findAsset(result, "Experiment", "churn"))
}
