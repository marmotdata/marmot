package athena

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableMRN_DefaultCatalogAgreesWithTheGluePlugin(t *testing.T) {
	// plugins/glue files a table as (Table, Glue, [database, table]). Athena reads the same Glue Data Catalog, so both runs have to land on one asset.
	assert.Equal(t, "mrn://asset/table/glue/shop.orders", ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"))
	assert.Equal(t, "mrn://asset/view/glue/shop.order_totals", ids.tableMRN(typeView, "AwsDataCatalog", "shop", "order_totals"))
	assert.Equal(t, "mrn://asset/database/glue/shop", ids.databaseMRN("AwsDataCatalog", "shop"))
}

func TestTableMRN_OtherCatalogsAreNamedInThePath(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/glue/lab.shop.orders", ids.tableMRN(typeTable, "lab", "shop", "orders"))
	assert.NotEqual(t, ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"), ids.tableMRN(typeTable, "lab", "shop", "orders"))
}

func TestTableMRN_SameNameInTwoDatabasesDiffers(t *testing.T) {
	assert.NotEqual(t,
		ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"),
		ids.tableMRN(typeTable, "AwsDataCatalog", "archive", "orders"))
}

func TestMRNs_CarryTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod-eu"
	s := &Source{config: cfg}

	assert.Equal(t, "mrn://asset/table/glue@prod-eu/shop.orders", s.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders"))
	assert.Equal(t, "mrn://asset/workgroup/athena@prod-eu/analytics", s.workGroupMRN("analytics"))
	assert.Equal(t, "mrn://asset/catalog/athena@prod-eu/awsdatacatalog", s.catalogMRN("AwsDataCatalog"))
}

func TestWorkGroupAndCatalogMRNs(t *testing.T) {
	assert.Equal(t, "mrn://asset/workgroup/athena/analytics", ids.workGroupMRN("analytics"))
	assert.Equal(t, "mrn://asset/catalog/athena/awsdatacatalog", ids.catalogMRN("AwsDataCatalog"))
}

func TestSavedQueryMRN_IsWorkGroupThenName(t *testing.T) {
	assert.Equal(t, "mrn://asset/data-model-object/athena/analytics.daily-revenue", ids.savedQueryMRN("analytics", "daily-revenue"))
	assert.Equal(t, "mrn://asset/data-model-object/athena/daily-revenue", ids.savedQueryMRN("", "daily-revenue"))
}

func TestPreviousSavedQueryMRN_IsTheFoldedLegacyName(t *testing.T) {
	assert.Equal(t, "mrn://data-model-object/athena/analytics-daily-revenue", previousSavedQueryMRN("analytics", "daily-revenue"))
}

func TestBucketMRN_MatchesTheS3PluginIdentity(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/s3/marmot-lake", bucketMRN("marmot-lake"))
}

func TestTableMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := ids.tableMRN(typeTable, "AwsDataCatalog", "shop", "orders")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestDiscover_EveryAssetMRNIsUnique(t *testing.T) {
	s := newTestSource(t, seedAthena(), seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, result.Assets)

	seen := map[string]struct{}{}
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		assert.NotContains(t, seen, *a.MRN)
		seen[*a.MRN] = struct{}{}
	}
}

func TestDiscover_SavedQueriesCarryTheirPreviousMRN(t *testing.T) {
	s := newTestSource(t, seedAthena(), seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	for _, a := range result.Assets {
		if a.Type == typeSavedQuery {
			assert.Equal(t, []string{"mrn://data-model-object/athena/analytics-daily-revenue"}, a.PreviousMRNs)
		}
	}
}

func TestDiscover_EveryLineageEndpointIsAnAssetOrAKnownExternalIdentity(t *testing.T) {
	s := newTestSource(t, seedAthena(), seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	known := map[string]struct{}{
		// The S3 plugin owns bucket assets; this run only points at them.
		bucketMRN("marmot-lake"):    {},
		bucketMRN("marmot-results"): {},
	}
	for _, a := range result.Assets {
		known[*a.MRN] = struct{}{}
	}

	require.NotEmpty(t, result.Lineage)
	for _, e := range result.Lineage {
		assert.Contains(t, known, e.Source, "edge source %s", e.Source)
		assert.Contains(t, known, e.Target, "edge target %s", e.Target)
	}
}
