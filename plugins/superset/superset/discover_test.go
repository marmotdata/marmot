package superset

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_FailsWhenLoginIsRejected(t *testing.T) {
	server := newFakeSuperset().rejectingLogin().start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": server.URL, "username": "admin", "password": "wrong",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "logging in")
	assert.NotContains(t, err.Error(), "wrong", "the password must not appear in the error")
}

func TestDiscover_SendsTheProviderOnLogin(t *testing.T) {
	f := newFakeSuperset()
	server := f.start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": server.URL, "username": "admin", "password": "admin", "provider": "ldap",
	})
	require.NoError(t, err)

	require.NotEmpty(t, f.requests)
	login := f.requests[0]
	assert.Equal(t, "/api/v1/security/login", login.URL.Path)
	assert.Equal(t, http.MethodPost, login.Method)
}

func TestDiscover_FailsWhenTheServerIsNotSuperset(t *testing.T) {
	// A wrong host must be an error, not an empty import.
	server := newFakeSuperset().withFailure("database/", http.StatusNotFound).start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": server.URL, "username": "admin", "password": "admin",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "databases")
}

func TestDiscover_CataloguesADashboard(t *testing.T) {
	result := discover(t, shop(), nil)

	d := findAsset(result, "Dashboard", "Shop Overview")
	require.NotNil(t, d)
	assert.Equal(t, "Dashboard", d.Type)
	assert.Equal(t, []string{"Superset"}, d.Providers)
	assert.Equal(t, "Shop Overview", *d.Name)
	assert.Equal(t, 1, d.Metadata["id"])
	assert.Equal(t, "shop-overview", d.Metadata["slug"])
	assert.Equal(t, true, d.Metadata["published"])
	assert.Equal(t, "published", d.Metadata["status"])
	assert.Equal(t, []string{"A B"}, d.Metadata["owners"])
	assert.Equal(t, "2026-09-07T21:12:01.731766+0000", d.Metadata["changed_on"])
	assert.Equal(t, 2, d.Metadata["chart_count"])
}

func TestDiscover_LinksADashboardToSupersetWithAnAbsoluteURL(t *testing.T) {
	f := shop()
	server := f.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": server.URL + "/", "username": "admin", "password": "admin",
	})
	require.NoError(t, err)

	d := findAsset(result, "Dashboard", "Shop Overview")
	require.NotNil(t, d)
	assert.Equal(t, server.URL+"/superset/dashboard/shop-overview/", d.Metadata["url"])
	require.Len(t, d.ExternalLinks, 1)
	assert.Equal(t, "Open in Superset", d.ExternalLinks[0].Name)
	assert.Equal(t, server.URL+"/superset/dashboard/shop-overview/", d.ExternalLinks[0].URL)
}

func TestDiscover_AddsUserTagsToADashboard(t *testing.T) {
	// Superset stores ownership and favourites as tags too (type 2 and
	// 3); only type 1 is something a person typed in.
	tagged := dashboardEntry(1, "Shop Overview", "shop-overview", true)
	tagged["tags"] = []map[string]interface{}{
		{"id": 1, "name": "finance", "type": 1},
		{"id": 2, "name": "owner:1", "type": 2},
		{"id": 3, "name": "favorited_by:1", "type": 3},
	}
	result := discover(t, newFakeSuperset().withDashboard(tagged), pluginsdk.RawConfig{"tags": []string{"bi"}})

	d := findAsset(result, "Dashboard", "Shop Overview")
	require.NotNil(t, d)
	assert.Equal(t, []string{"bi", "finance"}, d.Tags)
	assert.Equal(t, []string{"finance"}, d.Metadata["tags"])
}

func TestDiscover_IncludesDraftDashboardsByDefault(t *testing.T) {
	result := discover(t, shop(), nil)

	draft := findAsset(result, "Dashboard", "Draft Scratchpad")
	require.NotNil(t, draft)
	assert.Equal(t, false, draft.Metadata["published"])
	assert.Equal(t, "draft", draft.Metadata["status"])
}

func TestDiscover_SkipsDraftDashboardsWhenConfigured(t *testing.T) {
	result := discover(t, shop(), pluginsdk.RawConfig{"include_draft": false})

	assert.Nil(t, findAsset(result, "Dashboard", "Draft Scratchpad"))
	assert.NotNil(t, findAsset(result, "Dashboard", "Shop Overview"))
}

func TestDiscover_CataloguesAChart(t *testing.T) {
	described := chartEntry(1, "Orders by Status", "table", 1, "public.orders", dashboardRefEntry(1, "Shop Overview"))
	described["description"] = "Count of orders per status"
	result := discover(t, newFakeSuperset().withChart(described), nil)

	c := findAsset(result, "Chart", "Orders by Status")
	require.NotNil(t, c)
	assert.Equal(t, "Chart", c.Type)
	assert.Equal(t, []string{"Superset"}, c.Providers)
	require.NotNil(t, c.Description)
	assert.Equal(t, "Count of orders per status", *c.Description)
	assert.Equal(t, 1, c.Metadata["id"])
	assert.Equal(t, "table", c.Metadata["viz_type"])
	assert.Equal(t, "Table", c.Metadata["chart_type"])
	assert.Equal(t, 1, c.Metadata["datasource_id"])
	assert.Equal(t, "table", c.Metadata["datasource_type"])
	assert.Equal(t, "public.orders", c.Metadata["dataset"])
	assert.Equal(t, []string{"A B"}, c.Metadata["owners"])
	assert.Equal(t, []string{"Shop Overview"}, c.Metadata["dashboards"])
	assert.Equal(t, "2026-09-07T21:12:01.991148+0000", c.Metadata["changed_on"])
	assert.True(t, strings.HasSuffix(c.Metadata["url"].(string), "/explore/?slice_id=1"))
	require.Len(t, c.ExternalLinks, 1)
	assert.Equal(t, "Open in Superset", c.ExternalLinks[0].Name)
}

func TestDiscover_NormalisesTheChartType(t *testing.T) {
	result := discover(t, newFakeSuperset().
		withChart(chartEntry(1, "Bars", "echarts_timeseries_bar", 1, "public.orders")).
		withChart(chartEntry(2, "Slices", "pie", 1, "public.orders")).
		withChart(chartEntry(3, "Layers", "deck_scatter", 1, "public.orders")).
		withChart(chartEntry(4, "Words", "word_cloud", 1, "public.orders")), nil)

	assert.Equal(t, "Bar", findAsset(result, "Chart", "Bars").Metadata["chart_type"])
	assert.Equal(t, "Pie", findAsset(result, "Chart", "Slices").Metadata["chart_type"])
	assert.Equal(t, "Map", findAsset(result, "Chart", "Layers").Metadata["chart_type"])
	assert.Equal(t, "Other", findAsset(result, "Chart", "Words").Metadata["chart_type"])
}

func TestDiscover_BuildsAChartURLWhenSupersetReportsNone(t *testing.T) {
	bare := chartEntry(7, "Unlinked", "table", 1, "public.orders")
	delete(bare, "url")
	f := newFakeSuperset().withChart(bare)
	server := f.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": server.URL, "username": "admin", "password": "admin",
	})
	require.NoError(t, err)

	c := findAsset(result, "Chart", "Unlinked")
	require.NotNil(t, c)
	assert.Equal(t, server.URL+"/explore/?slice_id=7", c.Metadata["url"])
}

func TestDiscover_CataloguesAPhysicalDatasetWithItsColumns(t *testing.T) {
	result := discover(t, shop(), nil)

	ds := findAsset(result, "Data Model Object", "public.orders")
	require.NotNil(t, ds)
	assert.Equal(t, "Data Model Object", ds.Type)
	assert.Equal(t, "public.orders", *ds.Name, "a dataset is named schema.table")
	assert.Equal(t, 1, ds.Metadata["id"])
	assert.Equal(t, "physical", ds.Metadata["kind"])
	assert.Equal(t, "Shop", ds.Metadata["database"])
	assert.Equal(t, 1, ds.Metadata["database_id"])
	assert.Equal(t, "postgresql", ds.Metadata["backend"])
	assert.Equal(t, "public", ds.Metadata["schema"])
	assert.Equal(t, "orders", ds.Metadata["table_name"])
	assert.Equal(t, []string{"A B"}, ds.Metadata["owners"])
	assert.True(t, strings.HasSuffix(ds.Metadata["url"].(string), "/tablemodelview/edit/1"))
	assert.Nil(t, ds.Query, "a physical dataset has no query")

	columns := schemaColumns(t, ds)
	require.Contains(t, columns, "total")
	require.Contains(t, columns, "status")
	require.Contains(t, columns, "total_with_tax")
	assert.Equal(t, "NUMERIC(10, 2)", columns["total"]["data_type"])
	assert.Equal(t, "Order lifecycle state", columns["status"]["description"])
	assert.Equal(t, "total * 1.21", columns["total_with_tax"]["expression"], "a calculated column carries its expression")
	assert.NotContains(t, columns["total"], "expression", "a plain column carries no expression")
}

func TestDiscover_CataloguesAVirtualDatasetWithItsQuery(t *testing.T) {
	result := discover(t, shop(), nil)

	ds := findAsset(result, "Data Model Object", "public.order_totals")
	require.NotNil(t, ds)
	assert.Equal(t, "virtual", ds.Metadata["kind"])
	require.NotNil(t, ds.Query)
	assert.True(t, strings.HasPrefix(*ds.Query, "SELECT c.id AS customer_id"))
	require.NotNil(t, ds.QueryLanguage)
	assert.Equal(t, "SQL", *ds.QueryLanguage)
}

func TestDiscover_NamesADatasetWithoutASchemaByItsTableAlone(t *testing.T) {
	result := discover(t, newFakeSuperset().
		withDatabase(databaseEntry(1, "Local", "sqlite"), nil).
		withDataset(datasetEntry(9, "", "events", "physical", "", 1, "Local"), "sqlite"), nil)

	ds := findAsset(result, "Data Model Object", "events")
	require.NotNil(t, ds)
	assert.Equal(t, "events", *ds.Name)
	assert.NotContains(t, ds.Metadata, "schema")
}

func TestDiscover_UsesTheDatasetListingWhenTheDetailFails(t *testing.T) {
	result := discover(t, shop().withFailure("dataset/1", http.StatusInternalServerError), nil)

	ds := findAsset(result, "Data Model Object", "public.orders")
	require.NotNil(t, ds, "one broken detail must not drop the dataset")
	assert.Equal(t, "postgresql", ds.Metadata["backend"], "the backend still comes from the database listing")
	assert.NotContains(t, ds.Schema, "columns")
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", *ds.MRN, "FEEDS"),
		"table lineage still works from the listing")
}

func TestDiscover_CataloguesADatabaseConnection(t *testing.T) {
	result := discover(t, shop(), nil)

	db := findAsset(result, "DataSource", "Shop")
	require.NotNil(t, db)
	assert.Equal(t, "DataSource", db.Type)
	assert.Equal(t, 1, db.Metadata["id"])
	assert.Equal(t, "postgresql", db.Metadata["backend"])
	assert.Equal(t, "psycopg2", db.Metadata["driver"])
	assert.Equal(t, "marmot-test-superset-pg", db.Metadata["host"])
	assert.Equal(t, "5432", db.Metadata["port"])
	assert.Equal(t, "shop", db.Metadata["database"])
	assert.Equal(t, true, db.Metadata["expose_in_sqllab"])
	assert.Equal(t, false, db.Metadata["allow_dml"])
}

func TestDiscover_NeverRecordsAConnectionPassword(t *testing.T) {
	// Superset masks the password itself; the plugin still redacts, so a
	// server that did not mask could not leak one into the catalog.
	unmasked := connectionEntry(1, "Shop", "postgresql", "psycopg2",
		"postgresql://marmot:s3cret@db.internal:5432/shop", "db.internal", 5432, "shop")
	unmasked["parameters"].(map[string]interface{})["password"] = "s3cret"

	result := discover(t, newFakeSuperset().withDatabase(databaseEntry(1, "Shop", "postgresql"), unmasked), nil)

	db := findAsset(result, "DataSource", "Shop")
	require.NotNil(t, db)
	assert.Equal(t, "postgresql://marmot:xxx@db.internal:5432/shop", db.Metadata["sqlalchemy_uri"])
	for key, value := range db.Metadata {
		assert.NotContains(t, fmt.Sprint(value), "s3cret", "metadata key %s leaks the password", key)
	}
}

func TestDiscover_SurvivesAMissingConnectionEndpoint(t *testing.T) {
	result := discover(t, newFakeSuperset().
		withDatabase(databaseEntry(1, "Shop", "postgresql"), nil).
		withDataset(datasetEntry(1, "public", "orders", "physical", "", 1, "Shop"), "postgresql"), nil)

	db := findAsset(result, "DataSource", "Shop")
	require.NotNil(t, db)
	assert.NotContains(t, db.Metadata, "host")
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://data-model-object/superset/public.orders", "FEEDS"),
		"a bare-named backend needs no connection details for table lineage")
}

func TestDiscover_LinksADashboardToItsCharts(t *testing.T) {
	result := discover(t, shop(), nil)

	dashboard := assetMRN("Dashboard", "Shop Overview")
	assert.True(t, hasEdge(result, dashboard, assetMRN("Chart", "Orders by Status"), "CONTAINS"))
	assert.True(t, hasEdge(result, dashboard, assetMRN("Chart", "Spend per Customer"), "CONTAINS"))
	assert.False(t, hasEdge(result, dashboard, assetMRN("Chart", "Product Prices"), "CONTAINS"))
}

func TestDiscover_ReadsTheLayoutWhenTheChartsEndpointIsMissing(t *testing.T) {
	f := newFakeSuperset().
		withChart(chartEntry(3, "Product Prices", "table", 1, "public.orders")).
		withDashboard(dashboardEntry(2, "Draft Scratchpad", "draft-scratchpad", true)).
		withLayout(2, 3).
		withFailure("dashboard/2/charts", http.StatusNotFound)

	result := discover(t, f, nil)

	d := findAsset(result, "Dashboard", "Draft Scratchpad")
	require.NotNil(t, d)
	assert.Equal(t, 1, d.Metadata["chart_count"])
	assert.True(t, hasEdge(result, *d.MRN, assetMRN("Chart", "Product Prices"), "CONTAINS"))
}

func TestDiscover_LinksADatasetToTheChartsThatReadIt(t *testing.T) {
	result := discover(t, shop(), nil)

	assert.True(t, hasEdge(result, assetMRN("Data Model Object", "public.orders"), assetMRN("Chart", "Orders by Status"), "FEEDS"))
	assert.True(t, hasEdge(result, assetMRN("Data Model Object", "public.order_totals"), assetMRN("Chart", "Spend per Customer"), "FEEDS"))
}

func TestDiscover_LinksADatabaseToItsDatasets(t *testing.T) {
	result := discover(t, shop(), nil)

	assert.True(t, hasEdge(result, assetMRN("DataSource", "Shop"), assetMRN("Data Model Object", "public.orders"), "FEEDS"))
	assert.True(t, hasEdge(result, assetMRN("DataSource", "Shop"), assetMRN("Data Model Object", "public.order_totals"), "FEEDS"))
}

func TestDiscover_LinksAPhysicalDatasetToThePostgresTableItReads(t *testing.T) {
	result := discover(t, shop(), nil)

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://data-model-object/superset/public.orders", "FEEDS"),
		"the table end is the MRN the PostgreSQL plugin gives the table")
}

func TestDiscover_LinksAVirtualDatasetToTheTablesItsSQLReads(t *testing.T) {
	result := discover(t, shop(), nil)

	totals := assetMRN("Data Model Object", "public.order_totals")
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", totals, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", totals, "FEEDS"))
}

func TestDiscover_QualifiesTablesTheWayTheOwningPluginDoes(t *testing.T) {
	// Snowflake tables are database.schema.table in Marmot; the database
	// comes from the connection, the schema from the dataset.
	result := discover(t, newFakeSuperset().
		withDatabase(databaseEntry(2, "Warehouse", "snowflake"),
			connectionEntry(2, "Warehouse", "snowflake", "snowflake", "snowflake://user:XXXXXXXXXX@acct/SALES/PUBLIC?warehouse=wh", "", 0, "SALES")).
		withDataset(datasetEntry(5, "PUBLIC", "ORDERS", "physical", "", 2, "Warehouse"), "snowflake"), nil)

	assert.True(t, hasEdge(result, "mrn://table/snowflake/sales.public.orders", assetMRN("Data Model Object", "PUBLIC.ORDERS"), "FEEDS"))
}

func TestDiscover_TakesTheDatabaseFromTheURIWhenParametersLackIt(t *testing.T) {
	conn := connectionEntry(2, "Warehouse", "snowflake", "snowflake", "snowflake://user:XXXXXXXXXX@acct/SALES/PUBLIC?warehouse=wh", "", 0, "")
	conn["parameters"] = map[string]interface{}{}

	result := discover(t, newFakeSuperset().
		withDatabase(databaseEntry(2, "Warehouse", "snowflake"), conn).
		withDataset(datasetEntry(5, "PUBLIC", "ORDERS", "physical", "", 2, "Warehouse"), "snowflake"), nil)

	assert.True(t, hasEdge(result, "mrn://table/snowflake/sales.public.orders", assetMRN("Data Model Object", "PUBLIC.ORDERS"), "FEEDS"))
}

func TestDiscover_EmitsNoTableEdgeForABackendWithoutANamingRule(t *testing.T) {
	result := discover(t, newFakeSuperset().
		withDatabase(databaseEntry(3, "Legacy", "teradatasql"), nil).
		withDataset(datasetEntry(6, "dw", "facts", "physical", "", 3, "Legacy"), "teradatasql"), nil)

	require.NotNil(t, findAsset(result, "Data Model Object", "dw.facts"))
	for _, edge := range result.Lineage {
		assert.False(t, strings.HasPrefix(edge.Source, "mrn://table/"), "unexpected table edge %+v", edge)
	}
}

func TestDiscover_SkipsChartsWhenConfigured(t *testing.T) {
	result := discover(t, shop(), pluginsdk.RawConfig{"include_charts": false})

	assert.Nil(t, findAsset(result, "Chart", "Orders by Status"))
	assert.NotNil(t, findAsset(result, "Dashboard", "Shop Overview"))
	for _, edge := range result.Lineage {
		assert.False(t, strings.HasPrefix(edge.Target, "mrn://chart/"), "unexpected chart edge %+v", edge)
	}
}

func TestDiscover_SkipsDatasetsWhenConfigured(t *testing.T) {
	result := discover(t, shop(), pluginsdk.RawConfig{"include_datasets": false})

	assert.Nil(t, findAsset(result, "Data Model Object", "public.orders"))
	assert.NotNil(t, findAsset(result, "Chart", "Orders by Status"))
	assert.True(t, hasEdge(result, assetMRN("Dashboard", "Shop Overview"), assetMRN("Chart", "Orders by Status"), "CONTAINS"))
	for _, edge := range result.Lineage {
		assert.NotEqual(t, "mrn://table/postgresql/orders", edge.Source, "no table edge without the dataset it points at")
	}
}

func TestDiscover_SkipsDatabasesWhenConfiguredButStillLinksTables(t *testing.T) {
	result := discover(t, shop(), pluginsdk.RawConfig{"include_databases": false})

	assert.Nil(t, findAsset(result, "DataSource", "Shop"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", assetMRN("Data Model Object", "public.orders"), "FEEDS"),
		"the connection is still read for the backend even when it is not an asset")
}

func TestDiscover_SkipsLineageWhenConfigured(t *testing.T) {
	result := discover(t, shop(), pluginsdk.RawConfig{"discover_lineage": false})

	assert.NotEmpty(t, result.Assets)
	assert.Empty(t, result.Lineage)
}

func TestDiscover_FollowsPaginationToTheEnd(t *testing.T) {
	f := newFakeSuperset()
	for i := 1; i <= 7; i++ {
		f.withChart(chartEntry(i, "Chart "+string(rune('A'+i-1)), "table", 1, "public.orders"))
	}

	result := discover(t, f, pluginsdk.RawConfig{"page_size": 3})

	charts := 0
	for _, a := range result.Assets {
		if a.Type == "Chart" {
			charts++
		}
	}
	assert.Equal(t, 7, charts)

	chartPages := 0
	for _, r := range f.requests {
		if r.URL.Path == "/api/v1/chart/" {
			chartPages++
			assert.Contains(t, r.URL.RawQuery, "q=(page:", "pages are requested with a literal Rison query")
		}
	}
	assert.Equal(t, 3, chartPages, "7 charts in pages of 3 take exactly 3 requests")
}

func TestDiscover_SendsTheBearerTokenOnEveryCall(t *testing.T) {
	f := shop()
	discover(t, f, nil)

	for _, r := range f.requests {
		if r.URL.Path == "/api/v1/security/login" {
			continue
		}
		assert.Equal(t, "Bearer fake-access", r.Header.Get("Authorization"), "%s", r.URL.Path)
	}
}

// Marmot keeps the MRN a plugin declares and the UI rebuilds every link
// by splitting it back into type, service and name, which /assets/lookup
// feeds through mrn.New again. The MRN has to survive that unchanged or
// the asset exists but its page 404s.
func TestDiscover_MRNsSurviveTheServer(t *testing.T) {
	result := discover(t, shop(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.NotNil(t, asset.Name)
		require.NotEmpty(t, asset.Providers)

		parsed, err := mrn.Parse(*asset.MRN)
		require.NoError(t, err)
		assert.Equal(t, *asset.MRN, mrn.New(parsed.Type, parsed.Service, parsed.Name))
		assert.Equal(t, *asset.MRN, mrn.New(asset.Type, asset.Providers[0], *asset.Name),
			"the MRN must be what the server derives from the asset's own fields")
	}
}

// Every edge must end on an asset this run emitted or on a table another
// plugin catalogues; the server drops anything else.
func TestDiscover_LineagePointsAtRealAssets(t *testing.T) {
	result := discover(t, shop(), nil)
	require.NotEmpty(t, result.Lineage)

	known := make(map[string]bool, len(result.Assets))
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, edge := range result.Lineage {
		assert.True(t, known[edge.Source] || strings.HasPrefix(edge.Source, "mrn://table/"),
			"edge source %q is neither an emitted asset nor a native table", edge.Source)
		assert.True(t, known[edge.Target], "edge target %q is not an emitted asset", edge.Target)
	}
}

func TestDiscover_EmitsEachEdgeOnce(t *testing.T) {
	result := discover(t, shop(), nil)

	// LineageEdge holds a column-lineage slice and so cannot be a map key.
	type edgeKey struct{ source, target, edgeType string }

	seen := make(map[edgeKey]int)
	for _, edge := range result.Lineage {
		seen[edgeKey{edge.Source, edge.Target, edge.Type}]++
	}
	for edge, count := range seen {
		assert.Equal(t, 1, count, "edge %+v emitted more than once", edge)
	}
}

func TestChartIDsFromLayout_ReadsChartComponentsOnly(t *testing.T) {
	ids, err := chartIDsFromLayout(`{
		"DASHBOARD_VERSION_KEY": "v2",
		"ROOT_ID": {"type": "ROOT", "id": "ROOT_ID", "children": ["GRID_ID"]},
		"MARKDOWN-1": {"type": "MARKDOWN", "id": "MARKDOWN-1", "meta": {"code": "# hi"}},
		"CHART-abc": {"type": "CHART", "id": "CHART-abc", "meta": {"chartId": 12, "sliceName": "x"}},
		"CHART-def": {"type": "CHART", "id": "CHART-def", "meta": {"chartId": 34}}
	}`)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 34}, ids)
}

func TestChartIDsFromLayout_EmptyLayoutHasNoCharts(t *testing.T) {
	ids, err := chartIDsFromLayout("")
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestChartIDsFromLayout_RejectsMalformedJSON(t *testing.T) {
	_, err := chartIDsFromLayout("{not json")
	require.Error(t, err)
}
