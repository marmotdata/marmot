package spline

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pipeline is named after the Spark application, so the MRN stays the same
// across runs of the same job.

func TestPipelineMRN_IsTheApplicationName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/spline/daily-etl", assetMRN("Pipeline", "daily-etl"))
}

func TestPipelineMRN_TurnsSpacesIntoHyphens(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/spline/nightly-report", assetMRN("Pipeline", "Nightly Report"))
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged.
	original := assetMRN("Pipeline", "daily-etl")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

// Lineage points at assets other plugins own, so those MRNs have to match the
// owning plugin's provider string and name shape exactly.

func TestNativeMRN_PostgresTable(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:postgresql://pg.internal:5432/warehouse:public.orders")
	require.True(t, ok)

	assert.Equal(t, "mrn://table/postgresql/orders", nativeMRN(ref))
}

func TestNativeMRN_S3Bucket(t *testing.T) {
	ref, ok := parseDataSourceURI("s3://marmot-lake/raw/events")
	require.True(t, ok)

	assert.Equal(t, "mrn://bucket/s3/marmot-lake", nativeMRN(ref))
}

func TestNativeMRN_HiveTable(t *testing.T) {
	ref, ok := parseDataSourceURI("hive://sales/orders")
	require.True(t, ok)

	assert.Equal(t, "mrn://table/hive/sales.orders", nativeMRN(ref))
}

func TestNativeMRN_SQLServerTableKeepsTheSpaceInTheProvider(t *testing.T) {
	// mrn.New lowercases the provider but does not replace its spaces, and the
	// SQL Server plugin's own assets are stored the same way.
	ref, ok := parseDataSourceURI("jdbc:sqlserver://sql.internal:1433;databaseName=shop:sales.orders")
	require.True(t, ok)

	assert.Equal(t, "mrn://table/sql server/shop.sales.orders", nativeMRN(ref))
}

func TestNativeMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	ref, ok := parseDataSourceURI("hive://sales/orders")
	require.True(t, ok)
	original := nativeMRN(ref)

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

// The server rebuilds identity from (Type, Providers[0], Name), so an asset
// whose declared MRN disagrees would be stored under a different one and every
// edge pointing at it would dangle.
func TestAsset_MRNMatchesWhatTheServerDerives(t *testing.T) {
	source := &Source{config: &Config{}}
	app := application{
		Name:    "daily-etl",
		Events:  []ExecutionEvent{{ExecutionEventID: "plan:run", ExecutionPlanID: "plan", Timestamp: 1788856725982}},
		PlanIDs: []string{"plan"},
	}

	asset := source.buildPipelineAsset(app, map[string]*ExecutionPlanInfo{}, nil, nil, nil)

	require.NotNil(t, asset.MRN)
	require.NotNil(t, asset.Name)
	require.NotEmpty(t, asset.Providers)
	assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
}

func TestAsset_ProviderIsExactlySpline(t *testing.T) {
	source := &Source{config: &Config{}}
	app := application{
		Name:    "daily-etl",
		Events:  []ExecutionEvent{{ExecutionEventID: "plan:run", Timestamp: 1788856725982}},
		PlanIDs: []string{"plan"},
	}

	asset := source.buildPipelineAsset(app, map[string]*ExecutionPlanInfo{}, nil, nil, nil)

	assert.Equal(t, []string{"Spline"}, asset.Providers)
	assert.Equal(t, "Pipeline", asset.Type)
}
