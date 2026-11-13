package spline

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pipeline is named after the Spark application, so the MRN stays the same across runs of the same job.

func pipelineMRN(zone, name string) string {
	cfg := &Config{}
	cfg.Zone = zone
	return cfg.AssetMRN("Pipeline", provider, name)
}

func TestPipelineMRN_IsTheApplicationName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/spline/daily-etl", pipelineMRN("", "daily-etl"))
}

func TestPipelineMRN_TurnsSpacesIntoHyphens(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/spline/nightly-report", pipelineMRN("", "Nightly Report"))
}

func TestPipelineMRN_CarriesTheZone(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/spline@prod-eu/daily-etl", pipelineMRN("prod-eu", "daily-etl"))
}

func TestPipelineMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := pipelineMRN("", "daily-etl")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

// Lineage points at assets other plugins own, so those MRNs have to match the owning plugin's provider string and path exactly. A JDBC host only hints at the owner's zone.

func TestNativeMRN_PostgresTableIsDatabaseSchemaTable(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:postgresql://pg.internal:5432/warehouse:public.orders")
	require.True(t, ok)

	assert.Equal(t, "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.orders", nativeMRN(ref))
}

func TestNativeMRN_SameTableNameInTwoSchemasDiffers(t *testing.T) {
	a, _ := parseDataSourceURI("jdbc:postgresql://pg:5432/warehouse:reporting.orders")
	b, _ := parseDataSourceURI("jdbc:postgresql://pg:5432/warehouse:staging.orders")

	assert.NotEqual(t, nativeMRN(a), nativeMRN(b))
}

func TestNativeMRN_TwoServersHaveDifferentHostHints(t *testing.T) {
	a, _ := parseDataSourceURI("jdbc:postgresql://pg-a:5432/warehouse:public.orders")
	b, _ := parseDataSourceURI("jdbc:postgresql://pg-b:5432/warehouse:public.orders")

	assert.NotEqual(t, nativeMRN(a), nativeMRN(b))
}

func TestNativeMRN_S3BucketHasNoHostHint(t *testing.T) {
	ref, ok := parseDataSourceURI("s3://marmot-lake/raw/events")
	require.True(t, ok)

	assert.Equal(t, "mrn://asset/bucket/s3/marmot-lake", nativeMRN(ref))
}

func TestNativeMRN_HiveTable(t *testing.T) {
	ref, ok := parseDataSourceURI("hive://sales/orders")
	require.True(t, ok)

	assert.Equal(t, "mrn://asset/table/hive/sales.orders", nativeMRN(ref))
}

func TestNativeMRN_SQLServerTable(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:sqlserver://sql.internal:1433;databaseName=shop:sales.orders")
	require.True(t, ok)

	assert.Equal(t, "mrn://asset/table/sql-server@host=sql.internal:1433/shop.sales.orders", nativeMRN(ref))
}

func TestNativeMRN_IsStableUnderRoundTrip(t *testing.T) {
	ref, ok := parseDataSourceURI("hive://sales/orders")
	require.True(t, ok)
	original := nativeMRN(ref)

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestAsset_PipelineMRNAndName(t *testing.T) {
	source := &Source{config: &Config{}}
	source.config.Zone = "prod-eu"
	app := application{
		Name:    "daily-etl",
		Events:  []ExecutionEvent{{ExecutionEventID: "plan:run", ExecutionPlanID: "plan", Timestamp: 1788856725982}},
		PlanIDs: []string{"plan"},
	}

	asset := source.buildPipelineAsset(app, map[string]*LineageDetailed{}, nil, nil)

	require.NotNil(t, asset.MRN)
	assert.Equal(t, "mrn://asset/pipeline/spline@prod-eu/daily-etl", *asset.MRN)
	assert.Equal(t, "daily-etl", *asset.Name)
	assert.Equal(t, []string{"Spline"}, asset.Providers)
	assert.Equal(t, "Pipeline", asset.Type)
	assert.Empty(t, asset.PreviousMRNs)
}

func TestZoneBindings(t *testing.T) {
	cfg := &Config{Host: "http://Spline.internal:8080/producer"}
	cfg.Zone = "prod"

	assert.Equal(t,
		[]pluginsdk.ZoneBinding{{Provider: "Spline", Host: "spline.internal:8080", Zone: "prod"}},
		(&Source{config: cfg}).zoneBindings())
}
