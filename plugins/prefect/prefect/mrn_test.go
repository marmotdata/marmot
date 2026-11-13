package prefect

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSource(zone string) *Source {
	cfg := &Config{Host: "http://localhost:4200/api"}
	cfg.Zone = zone
	return &Source{config: cfg}
}

// A flow's name is its identity, and a task is qualified by the flow it belongs to, because two flows may both have a task called "extract".

func TestPipelineMRN_IsTheFlowName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/prefect/nightly-etl", testSource("").pipelineMRN("nightly-etl"))
}

func TestTaskMRN_QualifiesTheTaskWithItsFlow(t *testing.T) {
	assert.Equal(t, "mrn://asset/task/prefect/nightly-etl.extract", testSource("").taskMRN("nightly-etl", taskName("extract-bf522387")))
}

func TestTaskMRN_SameTaskNameInDifferentFlowsDiffers(t *testing.T) {
	s := testSource("")
	assert.NotEqual(t, s.taskMRN("a", "extract"), s.taskMRN("b", "extract"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/prefect@prod/nightly-etl", testSource("prod").pipelineMRN("nightly-etl"))
	assert.Equal(t, "mrn://asset/task/prefect@prod/nightly-etl.extract", testSource("prod").taskMRN("nightly-etl", "extract"))
}

func TestPipelineAsset_MRNIsKinded(t *testing.T) {
	asset := testSource("").pipelineAsset(Flow{ID: nightlyFlowID, Name: "nightly-etl"}, nil, nil)

	require.NotNil(t, asset.MRN)
	assert.Equal(t, "mrn://asset/pipeline/prefect/nightly-etl", *asset.MRN)
	assert.Equal(t, "nightly-etl", *asset.Name)
}

func TestTaskAsset_KeepsItsOwnNameAndTheOldMRN(t *testing.T) {
	asset := testSource("").taskAsset("nightly-etl", discoveredTask{Name: "extract", Key: "extract-bf522387"})

	require.NotNil(t, asset.MRN)
	assert.Equal(t, "mrn://asset/task/prefect/nightly-etl.extract", *asset.MRN)
	assert.Equal(t, "extract", *asset.Name)
	assert.Equal(t, []string{"mrn://asset/task/prefect/nightly-etl-extract"}, asset.PreviousMRNs)
}

func TestPipelineMRN_SanitisesAFlowNameWithSpaces(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/prefect/nightly-etl", testSource("").pipelineMRN("Nightly ETL"))
}

func TestContainsEdge_PointsAtAnMRNTheSameRunEmits(t *testing.T) {
	// An edge whose endpoint no asset in this run created is dropped by the server, so the CONTAINS target has to be the Task's own MRN.
	assets, edges := testSource("").taskAssets("nightly-etl", "mrn://asset/pipeline/prefect/nightly-etl", []TaskRun{
		{ID: "run-1", TaskKey: "extract-bf522387"},
	})

	require.Len(t, assets, 1)
	require.NotEmpty(t, edges)
	assert.Equal(t, pluginsdk.LineageEdge{
		Source: "mrn://asset/pipeline/prefect/nightly-etl",
		Target: *assets[0].MRN,
		Type:   "CONTAINS",
	}, edges[0])
}

func TestAssetKey_PostgreSQLFollowsTheNamingTableWithAHostHint(t *testing.T) {
	target, ok := parseAssetKey("postgres://Warehouse.internal/shop/public/orders")

	require.True(t, ok)
	assert.Equal(t, "mrn://asset/table/postgresql@host=warehouse.internal:5432/shop.public.orders", target.MRN())
}

func TestAssetKey_ExplicitPortIsKept(t *testing.T) {
	target, ok := parseAssetKey("mysql://db:3307/shop/orders")

	require.True(t, ok)
	assert.Equal(t, "mrn://asset/table/mysql@host=db:3307/shop.orders", target.MRN())
}

func TestAssetKey_BigQueryUsesProjectDatasetTable(t *testing.T) {
	target, ok := parseAssetKey("bigquery://proj/analytics/orders")

	require.True(t, ok)
	assert.Equal(t, "mrn://asset/table/bigquery/proj.analytics.orders", target.MRN())
}

func TestAssetKey_SameTableNameInTwoSchemasDiffers(t *testing.T) {
	a, _ := parseAssetKey("postgres://h/shop/public/orders")
	b, _ := parseAssetKey("postgres://h/shop/archive/orders")
	assert.NotEqual(t, a.MRN(), b.MRN())
}

func TestAssetKey_S3IsTheBucket(t *testing.T) {
	target, ok := parseAssetKey("s3://raw-events/orders/2026-09-08.json")

	require.True(t, ok)
	assert.Equal(t, "mrn://asset/bucket/s3/raw-events", target.MRN())
	assert.Equal(t, mrn.Asset("Bucket", "S3", "raw-events").String(), target.MRN())
}
