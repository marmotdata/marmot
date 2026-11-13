package superset

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Superset objects are named by the titles people gave them, not by the
// numeric ids OpenMetadata uses. A dataset is the one kind qualified by
// its schema, because two datasets on same-named tables in different
// schemas are different datasets.

// assetMRN builds the MRN of a Superset asset from its dotted path, the way the plugin does from its levels.
func assetMRN(assetType, path string) string {
	return mrn.Asset(assetType, provider, strings.Split(path, ".")...).String()
}

func TestDashboardMRN_IsTheTitle(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/superset/shop-overview", assetMRN("Dashboard", "Shop Overview"))
}

func TestChartMRN_IsTheSliceName(t *testing.T) {
	assert.Equal(t, "mrn://asset/chart/superset/orders-by-status", assetMRN("Chart", "Orders by Status"))
}

func TestDatasetMRN_IsSchemaAndTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/data-model-object/superset/public.orders", assetMRN("Data Model Object", "public.orders"))
}

func TestDatasetMRN_WithoutASchemaIsTheTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/data-model-object/superset/events", assetMRN("Data Model Object", datasetName("", "events")))
}

func TestDataSourceMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/datasource/superset/shop", assetMRN("DataSource", "Shop"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	d := &discovery{config: &Config{}}
	d.config.Zone = "prod"

	asset := d.newAsset("Dashboard", "Shop Overview", nil)
	assert.Equal(t, "mrn://asset/dashboard/superset@prod/shop-overview", *asset.MRN)
}

func TestEveryAsset_MRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discover(t, shop(), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy, *a.MRN)
		assert.Equal(t, strings.ReplaceAll(strings.ToLower(a.Type), " ", "-"), parsed.Type)
		assert.Equal(t, "superset", parsed.Provider)
	}
}

func TestDatasetAsset_IsNamedByItsTableAndNeedsNoPreviousMRN(t *testing.T) {
	result := discover(t, shop(), nil)

	for _, a := range result.Assets {
		if a.Type == "Data Model Object" && *a.MRN == assetMRN("Data Model Object", "public.orders") {
			assert.Equal(t, "orders", *a.Name)
			assert.Empty(t, a.PreviousMRNs, "the MRN is the string earlier releases built")
			return
		}
	}
	t.Fatal("dataset not discovered")
}

// Edges to tables other plugins own are pinned to those plugins' exact
// MRNs, because the server only keeps an edge whose both ends exist.

func TestNativeTableMRN_MatchesThePostgreSQLPlugin(t *testing.T) {
	assert.Equal(t, mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").String(), nativeTableMRN("postgresql", "", "shop", "public", "orders"))
}

func TestNativeTableMRN_TheConnectionsHostBecomesAHint(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql@host=db.internal:5432/shop.public.orders", nativeTableMRN("postgresql", "db.internal:5432", "shop", "public", "orders"))
}

func TestNativeTableMRN_SameTableInTwoSchemasDiffers(t *testing.T) {
	assert.NotEqual(t, nativeTableMRN("postgresql", "", "shop", "public", "orders"), nativeTableMRN("postgresql", "", "shop", "archive", "orders"))
}

func TestConnectionHost_UsesTheParametersThenTheURIThenTheBackendsPort(t *testing.T) {
	var conn databaseConnection
	conn.Parameters.Host = "DB.internal"
	conn.Parameters.Port = "6432"
	assert.Equal(t, "db.internal:6432", connectionHost("postgresql", &conn))

	conn = databaseConnection{SQLAlchemyURI: "postgresql://user:XXXXXXXXXX@Pg.internal/shop"}
	assert.Equal(t, "pg.internal:5432", connectionHost("postgresql", &conn))

	conn = databaseConnection{SQLAlchemyURI: "snowflake://user:XXXXXXXXXX@acme/"}
	assert.Equal(t, "", connectionHost("snowflake", &conn), "a hosted warehouse has no host to match")
}
