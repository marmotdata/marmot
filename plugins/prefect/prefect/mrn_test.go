package prefect

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A flow's name is its identity, and a task is qualified by the flow it
// belongs to, because two flows may both have a task called "extract".

func TestPipelineMRN_IsTheFlowName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/prefect/nightly-etl", assetMRN("Pipeline", "nightly-etl"))
}

func TestTaskMRN_FoldsTheFlowNameIntoTheTaskName(t *testing.T) {
	// mrn.New turns the separating slash into a hyphen, so the qualifier
	// survives inside a single MRN name segment.
	assert.Equal(t, "mrn://task/prefect/nightly-etl-extract",
		assetMRN("Task", taskName("nightly-etl", "extract-bf522387")))
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged
	// or the asset becomes unreachable from the UI.
	original := assetMRN("Pipeline", "nightly-etl")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTaskMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Task", taskName("nightly-etl", "load-d6bfc39f"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestPipelineAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from Type, Providers[0] and Name, so an
	// MRN that disagrees with those creates a second, orphaned asset.
	s := &Source{config: &Config{Host: "http://localhost:4200/api"}}

	asset := s.pipelineAsset(Flow{ID: nightlyFlowID, Name: "nightly-etl"}, nil, nil)

	require.NotNil(t, asset.MRN)
	require.NotNil(t, asset.Name)
	require.NotEmpty(t, asset.Providers)
	assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	assert.Equal(t, "mrn://pipeline/prefect/nightly-etl", *asset.MRN)
}

func TestTaskAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	s := &Source{config: &Config{Host: "http://localhost:4200/api"}}

	asset := s.taskAsset("nightly-etl", discoveredTask{
		Name: taskName("nightly-etl", "extract-bf522387"),
		Key:  "extract-bf522387",
	})

	require.NotNil(t, asset.MRN)
	require.NotNil(t, asset.Name)
	require.NotEmpty(t, asset.Providers)
	assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	assert.Equal(t, "mrn://task/prefect/nightly-etl-extract", *asset.MRN)
	assert.Equal(t, "nightly-etl/extract", *asset.Name, "the name people read keeps the slash")
}

func TestPipelineMRN_SanitisesAFlowNameWithSpaces(t *testing.T) {
	// mrn.New lowercases and replaces spaces, and the server does the same
	// to the Name it receives, so the two still agree.
	name := "Nightly ETL"

	assert.Equal(t, "mrn://pipeline/prefect/nightly-etl", assetMRN("Pipeline", name))
	assert.Equal(t, assetMRN("Pipeline", name), mrn.New("Pipeline", provider, name))
}

func TestContainsEdge_PointsAtAnMRNTheSameRunEmits(t *testing.T) {
	// An edge whose endpoint no asset in this run created is dropped by
	// the server, so the CONTAINS target has to be the Task's own MRN.
	s := &Source{config: &Config{Host: "http://localhost:4200/api"}}

	assets, edges := s.taskAssets("nightly-etl", "mrn://pipeline/prefect/nightly-etl", []TaskRun{
		{ID: "run-1", TaskKey: "extract-bf522387"},
	})

	require.Len(t, assets, 1)
	require.NotEmpty(t, edges)
	assert.Equal(t, pluginsdk.LineageEdge{
		Source: "mrn://pipeline/prefect/nightly-etl",
		Target: *assets[0].MRN,
		Type:   "CONTAINS",
	}, edges[0])
}

func TestMaterializationEdge_UsesThePostgreSQLPluginsTableIdentity(t *testing.T) {
	// Pinned on purpose: the PostgreSQL plugin files a table under
	// mrn://table/postgresql/<bare name>, and this edge has to land on
	// exactly that or it points at nothing.
	target, ok := parseAssetKey("postgres://warehouse.internal/shop/public/orders")

	require.True(t, ok)
	assert.Equal(t, "mrn://table/postgresql/orders", target.MRN())
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "orders"), target.MRN())
}

func TestMaterializationEdge_UsesTheS3PluginsBucketIdentity(t *testing.T) {
	target, ok := parseAssetKey("s3://raw-events/orders/2026-09-08.json")

	require.True(t, ok)
	assert.Equal(t, "mrn://bucket/s3/raw-events", target.MRN())
	assert.Equal(t, mrn.New("Bucket", "S3", "raw-events"), target.MRN())
}
