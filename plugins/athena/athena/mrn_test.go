package athena

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Marmot server rebuilds an asset's identity from its Type, first
// Provider and Name. These tests pin the strings that come out of that, so a
// change that would move an asset shows up here rather than as a duplicate in
// the catalog.

func TestTableMRN_IsTheBareTableNameUnderGlue(t *testing.T) {
	assert.Equal(t, "mrn://table/glue/orders", tableMRN(typeTable, "orders"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/glue/order_totals", tableMRN(typeView, "order_totals"))
}

func TestDatabaseMRN_IsTheBareDatabaseNameUnderGlue(t *testing.T) {
	assert.Equal(t, "mrn://database/glue/shop", databaseMRN("shop"))
}

func TestWorkGroupMRN_IsTheWorkGroupNameUnderAthena(t *testing.T) {
	assert.Equal(t, "mrn://workgroup/athena/analytics", workGroupMRN("analytics"))
}

func TestSavedQueryMRN_FoldsTheWorkGroupIntoTheName(t *testing.T) {
	// mrn.New turns the separating slash into a hyphen, so the workgroup
	// stays part of the identity without splitting the MRN.
	assert.Equal(t, "mrn://data-model-object/athena/analytics-daily-revenue",
		savedQueryMRN("analytics", "daily-revenue"))
}

func TestCatalogMRN_IsTheCatalogNameUnderAthena(t *testing.T) {
	assert.Equal(t, "mrn://catalog/athena/awsdatacatalog", catalogMRN("AwsDataCatalog"))
}

func TestBucketMRN_MatchesTheS3PluginIdentity(t *testing.T) {
	// The S3 plugin names a bucket asset by its bare bucket name, so a
	// lineage edge from a table location lands on the bucket it already
	// catalogued.
	assert.Equal(t, "mrn://bucket/s3/marmot-lake", bucketMRN("marmot-lake"))
}

func TestTableMRN_AgreesWithTheGluePlugin(t *testing.T) {
	// plugins/glue files a table as (Table, Glue, bare name). Athena reads
	// the same Glue Data Catalog, so both runs have to land on one asset.
	assert.Equal(t, mrn.New("Table", "Glue", "orders"), tableMRN(typeTable, "orders"))
}

func TestTableMRN_AgreesWithTheOpenMetadataProjection(t *testing.T) {
	// plugins/openmetadata projects Athena as {Provider: "Glue", TableName:
	// nameOnly}, so an OpenMetadata import of an Athena service reaches the
	// same asset this plugin creates.
	assert.Equal(t, mrn.New("Table", "Glue", "orders"), tableMRN(typeTable, "orders"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := tableMRN(typeTable, "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestSavedQueryMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := savedQueryMRN("analytics", "daily-revenue")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDiscover_EveryAssetMRNMatchesItsOwnIdentity(t *testing.T) {
	// The server ignores the MRN a plugin sends and rebuilds it from the
	// asset's own fields. An asset whose MRN disagrees with that would be
	// pointed at by lineage edges that never resolve.
	s := newTestSource(t, seedAthena(), seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		require.Len(t, a.Providers, 1)

		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "asset %s", *a.Name)
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
