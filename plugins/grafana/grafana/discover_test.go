package grafana

import (
	"fmt"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	shopUID     = "ffxkla6ojkvlsf"
	testDataUID = "efxkla7l5962oe"
)

// Dashboards

func TestDiscover_NamesADashboardByItsFolderAndTitle(t *testing.T) {
	result := discover(t, seeded(), nil)

	dash := findAsset(result, "Dashboard", "Sales/Orders overview")
	require.NotNil(t, dash)
	assert.Equal(t, []string{"Grafana"}, dash.Providers)
	assert.Equal(t, "mrn://dashboard/grafana/sales-orders-overview", *dash.MRN)
}

func TestDiscover_NamesAGeneralFolderDashboardByItsTitleAlone(t *testing.T) {
	// Grafana reports the root as a folder called General with an empty
	// uid; it is not a real folder and stays out of the name.
	result := discover(t, seeded(), nil)

	assert.NotNil(t, findAsset(result, "Dashboard", "Ops home"))
	assert.Nil(t, findAsset(result, "Dashboard", "General/Ops home"))
}

func TestDiscover_CarriesDashboardMetadata(t *testing.T) {
	f := seeded()
	result := discover(t, f, nil)

	dash := findAsset(result, "Dashboard", "Sales/Orders overview")
	require.NotNil(t, dash)

	assert.Equal(t, "orders-overview", dash.Metadata["uid"])
	assert.Equal(t, int64(2896204983521280), dash.Metadata["id"])
	assert.Equal(t, "Sales", dash.Metadata["folder"])
	assert.Equal(t, "dfxkla5oojcw0c", dash.Metadata["folder_uid"])
	assert.Equal(t, []string{"sales"}, dash.Metadata["tags"])
	assert.Equal(t, 1, dash.Metadata["version"])
	assert.Equal(t, "2026-09-07T21:10:31Z", dash.Metadata["created"])
	assert.Equal(t, "2026-09-07T21:10:31Z", dash.Metadata["updated"])
	assert.Equal(t, "Anonymous", dash.Metadata["created_by"])
	assert.Equal(t, "Anonymous", dash.Metadata["updated_by"])
	assert.Equal(t, false, dash.Metadata["provisioned"])
	assert.Equal(t, "5m", dash.Metadata["refresh"])
	assert.Equal(t, "now-7d", dash.Metadata["time_from"])
	assert.Equal(t, "now", dash.Metadata["time_to"])
	assert.Equal(t, 3, dash.Metadata["panel_count"], "rows and text panels are not counted")
	assert.NotContains(t, dash.Metadata, "schema_version", "absent from the fixture, so absent here")

	url, _ := dash.Metadata["url"].(string)
	assert.True(t, strings.HasSuffix(url, "/d/orders-overview/orders-overview"), url)
	assert.True(t, strings.HasPrefix(url, "http://"), "the url is absolute")

	require.NotNil(t, dash.Description)
	assert.Equal(t, "Revenue and top customers", *dash.Description)
}

func TestDiscover_LeavesEmptyMetadataOut(t *testing.T) {
	result := discover(t, seeded(), nil)

	dash := findAsset(result, "Dashboard", "Ops home")
	require.NotNil(t, dash)

	assert.NotContains(t, dash.Metadata, "folder_uid")
	assert.NotContains(t, dash.Metadata, "tags")
	assert.NotContains(t, dash.Metadata, "refresh")
	assert.Nil(t, dash.Description)
}

func TestDiscover_AppendsDashboardTagsToConfiguredTags(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"tags": []string{"grafana", "sales"}})

	dash := findAsset(result, "Dashboard", "Sales/Orders overview")
	require.NotNil(t, dash)
	assert.Equal(t, []string{"grafana", "sales"}, dash.Tags, "a tag present in both appears once")

	chart := findAsset(result, "Chart", "Sales/Orders overview/Revenue")
	require.NotNil(t, chart)
	assert.Equal(t, []string{"grafana", "sales"}, chart.Tags, "charts carry the configured tags only")
}

func TestDiscover_LinksBackToGrafana(t *testing.T) {
	result := discover(t, seeded(), nil)

	dash := findAsset(result, "Dashboard", "Sales/Orders overview")
	require.NotNil(t, dash)
	require.Len(t, dash.ExternalLinks, 1)
	assert.Equal(t, "Open in Grafana", dash.ExternalLinks[0].Name)
	assert.True(t, strings.HasSuffix(dash.ExternalLinks[0].URL, "/d/orders-overview/orders-overview"))

	chart := findAsset(result, "Chart", "Sales/Orders overview/Revenue")
	require.NotNil(t, chart)
	require.Len(t, chart.ExternalLinks, 1)
	assert.True(t, strings.HasSuffix(chart.ExternalLinks[0].URL, "/d/orders-overview/orders-overview?viewPanel=1"))
}

// Charts

func TestDiscover_CreatesAChartPerPanel(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.NotNil(t, findAsset(result, "Chart", "Sales/Orders overview/Revenue"))
	assert.NotNil(t, findAsset(result, "Chart", "Sales/Orders overview/Random"))
	assert.NotNil(t, findAsset(result, "Chart", "Ops home/Customer totals"))
}

func TestDiscover_HoistsPanelsOutOfCollapsedRows(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.NotNil(t, findAsset(result, "Chart", "Sales/Orders overview/Top customers"),
		"the panel inside the collapsed Details row")
	assert.Nil(t, findAsset(result, "Chart", "Sales/Orders overview/Details"), "the row itself is not a chart")
}

func TestDiscover_SkipsTextPanels(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.Nil(t, findAsset(result, "Chart", "Sales/Orders overview/Notes"))
}

func TestDiscover_NamesAnUntitledPanelByItsID(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.NotNil(t, findAsset(result, "Chart", "Ops home/panel-1"))
}

func TestDiscover_KeepsPanelsWithTheSameTitleApart(t *testing.T) {
	f := newFakeGrafana().withDashboard(dashboardDoc("dup", "Dup",
		sqlPanel(1, "stat", "Total", dsRef("grafana-postgresql-datasource", shopUID), "SELECT 1"),
		sqlPanel(2, "stat", "Total", dsRef("grafana-postgresql-datasource", shopUID), "SELECT 2"),
	))

	result := discover(t, f, nil)

	assert.NotNil(t, findAsset(result, "Chart", "Dup/Total"))
	assert.NotNil(t, findAsset(result, "Chart", "Dup/Total (panel 2)"))
}

func TestDiscover_CarriesChartMetadata(t *testing.T) {
	result := discover(t, seeded(), nil)

	chart := findAsset(result, "Chart", "Sales/Orders overview/Revenue")
	require.NotNil(t, chart)

	assert.Equal(t, int64(1), chart.Metadata["panel_id"])
	assert.Equal(t, "timeseries", chart.Metadata["panel_type"])
	assert.Equal(t, "Line", chart.Metadata["chart_type"])
	assert.Equal(t, "Orders overview", chart.Metadata["dashboard"])
	assert.Equal(t, "orders-overview", chart.Metadata["dashboard_uid"])
	assert.Equal(t, "Shop", chart.Metadata["datasource"])
	assert.Equal(t, "grafana-postgresql-datasource", chart.Metadata["datasource_type"])
	assert.Equal(t, shopUID, chart.Metadata["datasource_uid"])
	assert.Equal(t, 1, chart.Metadata["target_count"])

	url, _ := chart.Metadata["url"].(string)
	assert.True(t, strings.HasSuffix(url, "/d/orders-overview/orders-overview?viewPanel=1"), url)

	require.NotNil(t, chart.Description)
	assert.Equal(t, "Daily revenue", *chart.Description)
}

func TestDiscover_RecordsSQLOnAChart(t *testing.T) {
	result := discover(t, seeded(), nil)

	chart := findAsset(result, "Chart", "Sales/Orders overview/Top customers")
	require.NotNil(t, chart)
	require.NotNil(t, chart.Query)
	require.NotNil(t, chart.QueryLanguage)
	assert.Equal(t, "SELECT * FROM customers", *chart.Query)
	assert.Equal(t, "SQL", *chart.QueryLanguage)
}

func TestDiscover_LeavesTheQueryEmptyForNonSQLPanels(t *testing.T) {
	result := discover(t, seeded(), nil)

	chart := findAsset(result, "Chart", "Sales/Orders overview/Random")
	require.NotNil(t, chart)
	assert.Nil(t, chart.Query)
	assert.Nil(t, chart.QueryLanguage)
}

func TestDiscover_RecordsPromQLOnAChart(t *testing.T) {
	f := newFakeGrafana().
		withDatasource(map[string]any{"id": 3, "uid": "prom", "name": "Prometheus", "type": "prometheus", "typeName": "Prometheus", "access": "proxy", "url": "http://prom:9090"}).
		withDashboard(dashboardDoc("mon", "Monitoring", map[string]any{
			"id": 1, "type": "timeseries", "title": "Requests", "datasource": dsRef("prometheus", "prom"),
			"targets": []map[string]any{{"refId": "A", "expr": "rate(http_requests_total[5m])"}},
		}))

	result := discover(t, f, nil)

	chart := findAsset(result, "Chart", "Monitoring/Requests")
	require.NotNil(t, chart)
	require.NotNil(t, chart.Query)
	assert.Equal(t, "rate(http_requests_total[5m])", *chart.Query)
	assert.Equal(t, "PromQL", *chart.QueryLanguage)
}

func TestDiscover_UsesTheDefaultDataSourceWhenAPanelNamesNone(t *testing.T) {
	result := discover(t, seeded(), nil)

	chart := findAsset(result, "Chart", "Ops home/Customer totals")
	require.NotNil(t, chart)
	assert.Equal(t, "Shop", chart.Metadata["datasource"])
	assert.True(t, hasEdge(result, "mrn://datasource/grafana/shop", *chart.MRN, "FEEDS"))
}

func TestDiscover_ResolvesStringDataSourceReferencesByName(t *testing.T) {
	// Dashboards saved before Grafana 8.3 name the data source with a
	// bare string.
	f := seeded().withDashboard(dashboardDoc("old", "Old style",
		sqlPanel(1, "graph", "Legacy", "Shop", "SELECT * FROM orders")))

	result := discover(t, f, nil)

	chart := findAsset(result, "Chart", "Old style/Legacy")
	require.NotNil(t, chart)
	assert.Equal(t, "Shop", chart.Metadata["datasource"])
	assert.Equal(t, shopUID, chart.Metadata["datasource_uid"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", *chart.MRN, "FEEDS"))
}

func TestDiscover_ATargetsDataSourceOverridesThePanels(t *testing.T) {
	f := seeded().withDashboard(dashboardDoc("mixed", "Mixed", map[string]any{
		"id": 1, "type": "timeseries", "title": "Both", "datasource": dsRef("datasource", "-- Mixed --"),
		"targets": []map[string]any{
			{"refId": "A", "datasource": dsRef("grafana-postgresql-datasource", shopUID), "rawSql": "SELECT * FROM orders"},
			{"refId": "B", "datasource": dsRef("grafana-testdata-datasource", testDataUID), "scenarioId": "random_walk"},
		},
	}))

	result := discover(t, f, nil)

	chart := findAsset(result, "Chart", "Mixed/Both")
	require.NotNil(t, chart)
	assert.Equal(t, "Shop", chart.Metadata["datasource"], "the first resolvable target stands in for the mixed panel")
	assert.True(t, hasEdge(result, "mrn://datasource/grafana/shop", *chart.MRN, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://datasource/grafana/testdata", *chart.MRN, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", *chart.MRN, "FEEDS"))
	assert.Nil(t, chart.Query, "one SQL and one non-SQL target record no query")
}

func TestDiscover_ResolvesLibraryPanels(t *testing.T) {
	f := seeded().
		withLibraryPanel("lib-1", map[string]any{
			"type": "barchart", "title": "Shared revenue", "description": "From the library",
			"datasource": dsRef("grafana-postgresql-datasource", shopUID),
			"targets":    []map[string]any{{"refId": "A", "format": "table", "rawSql": "SELECT country, count(*) FROM customers c GROUP BY 1"}},
		}).
		withDashboard(dashboardDoc("lib", "Library test", map[string]any{
			"id": 4, "gridPos": map[string]any{"h": 8, "w": 12, "x": 0, "y": 0},
			"libraryPanel": map[string]any{"uid": "lib-1", "name": "Shared revenue"},
		}))

	result := discover(t, f, nil)

	chart := findAsset(result, "Chart", "Library test/Shared revenue")
	require.NotNil(t, chart)
	assert.Equal(t, int64(4), chart.Metadata["panel_id"], "the id is the dashboard's, not the library's")
	assert.Equal(t, "barchart", chart.Metadata["panel_type"])
	assert.Equal(t, "Shop", chart.Metadata["datasource"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", *chart.MRN, "FEEDS"))
}

func TestDiscover_SkipsALibraryPanelItCannotRead(t *testing.T) {
	f := seeded().withDashboard(dashboardDoc("lib", "Library test", map[string]any{
		"id": 4, "libraryPanel": map[string]any{"uid": "missing", "name": "Gone"},
	}))

	result := discover(t, f, nil)

	dash := findAsset(result, "Dashboard", "Library test")
	require.NotNil(t, dash, "the dashboard itself is still discovered")
	assert.Equal(t, 0, dash.Metadata["panel_count"])
	assert.Nil(t, findAsset(result, "Chart", "Library test/Gone"))
}

func TestDiscover_LiftsPanelsOutOfLegacyRows(t *testing.T) {
	doc := dashboardDoc("legacy", "Legacy")
	dashboardOf(doc)["rows"] = []map[string]any{{
		"title":  "Row 1",
		"panels": []map[string]any{sqlPanel(1, "graph", "Old graph", "Shop", "SELECT * FROM orders")},
	}}

	result := discover(t, newFakeGrafana().withJSON(nil, datasourcesJSON).withDashboard(doc), nil)

	assert.NotNil(t, findAsset(result, "Chart", "Legacy/Old graph"))
}

// Data sources

func TestDiscover_CreatesADataSourcePerGrafanaDataSource(t *testing.T) {
	result := discover(t, seeded(), nil)

	shop := findAsset(result, "DataSource", "Shop")
	require.NotNil(t, shop)
	assert.Equal(t, "mrn://datasource/grafana/shop", *shop.MRN)
	assert.Equal(t, shopUID, shop.Metadata["uid"])
	assert.Equal(t, int64(1), shop.Metadata["id"])
	assert.Equal(t, "grafana-postgresql-datasource", shop.Metadata["type"])
	assert.Equal(t, "PostgreSQL", shop.Metadata["type_name"])
	assert.Equal(t, "marmot-test-grafana-pg:5432", shop.Metadata["url"])
	assert.Equal(t, "shop", shop.Metadata["database"])
	assert.Equal(t, true, shop.Metadata["is_default"])
	assert.Equal(t, false, shop.Metadata["read_only"])
	assert.Equal(t, "proxy", shop.Metadata["access"])

	require.Len(t, shop.ExternalLinks, 1)
	assert.True(t, strings.HasSuffix(shop.ExternalLinks[0].URL, "/connections/datasources/edit/"+shopUID))

	assert.NotNil(t, findAsset(result, "DataSource", "TestData"))
}

func TestDiscover_NeverRecordsDataSourceSecrets(t *testing.T) {
	// The listing does not include secrets today. Should a Grafana ever
	// add them, the typed struct still leaves them behind.
	f := seeded().withDatasource(map[string]any{
		"id": 9, "uid": "leaky", "name": "Leaky", "type": "mysql", "access": "proxy",
		"user": "root", "password": "hunter2", "basicAuthPassword": "hunter2",
		"secureJsonData":   map[string]any{"password": "hunter2"},
		"secureJsonFields": map[string]any{"password": true},
	})

	result := discover(t, f, nil)

	leaky := findAsset(result, "DataSource", "Leaky")
	require.NotNil(t, leaky)
	assert.NotContains(t, fmt.Sprint(leaky.Metadata), "hunter2")
	assert.NotContains(t, fmt.Sprint(leaky.Metadata), "root")
	assert.NotContains(t, leaky.Metadata, "user")
	assert.NotContains(t, fmt.Sprint(leaky.Sources[0].Properties), "hunter2")
}

func TestDiscover_ReadsTheDatabaseFromJSONDataWhenTheFieldIsEmpty(t *testing.T) {
	f := seeded().withDatasource(map[string]any{
		"id": 9, "uid": "new", "name": "Newer", "type": "mssql", "access": "proxy",
		"database": "", "jsonData": map[string]any{"database": "sales"},
	}).withDashboard(dashboardDoc("ms", "MSSQL",
		sqlPanel(1, "table", "Orders", dsRef("mssql", "new"), "SELECT * FROM orders")))

	result := discover(t, f, nil)

	ds := findAsset(result, "DataSource", "Newer")
	require.NotNil(t, ds)
	assert.Equal(t, "sales", ds.Metadata["database"])

	chart := findAsset(result, "Chart", "MSSQL/Orders")
	require.NotNil(t, chart)
	assert.True(t, hasEdge(result, "mrn://table/sql-server/sales.dbo.orders", *chart.MRN, "FEEDS"))
}

// Lineage

func TestDiscover_ADashboardContainsItsCharts(t *testing.T) {
	result := discover(t, seeded(), nil)

	dash := "mrn://dashboard/grafana/sales-orders-overview"
	assert.True(t, hasEdge(result, dash, "mrn://chart/grafana/sales-orders-overview-revenue", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://chart/grafana/sales-orders-overview-top-customers", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://chart/grafana/sales-orders-overview-random", "CONTAINS"))
}

func TestDiscover_ADataSourceFeedsTheChartsThatQueryIt(t *testing.T) {
	result := discover(t, seeded(), nil)

	assert.True(t, hasEdge(result, "mrn://datasource/grafana/shop", "mrn://chart/grafana/sales-orders-overview-revenue", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://datasource/grafana/testdata", "mrn://chart/grafana/sales-orders-overview-random", "FEEDS"))
	assert.False(t, hasEdge(result, "mrn://datasource/grafana/testdata", "mrn://chart/grafana/sales-orders-overview-revenue", "FEEDS"))
}

func TestDiscover_ATableFeedsTheChartAndTheDashboardThatReadIt(t *testing.T) {
	result := discover(t, seeded(), nil)

	revenue := "mrn://chart/grafana/sales-orders-overview-revenue"
	dash := "mrn://dashboard/grafana/sales-orders-overview"

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", revenue, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", revenue, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", dash, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", dash, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", "mrn://chart/grafana/sales-orders-overview-top-customers", "FEEDS"))
}

func TestDiscover_ATemplateVariableProducesNoTableEdge(t *testing.T) {
	result := discover(t, seeded(), nil)

	chart := "mrn://chart/grafana/ops-home-customer-totals"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customer_totals", chart, "FEEDS"))
	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Source, "$")
		assert.NotContains(t, edge.Source, "customer}")
	}
}

func TestDiscover_NonSQLDataSourcesProduceNoTableEdges(t *testing.T) {
	result := discover(t, seeded(), nil)

	for _, edge := range result.Lineage {
		if strings.HasSuffix(edge.Target, "sales-orders-overview-random") {
			assert.False(t, strings.HasPrefix(edge.Source, "mrn://table/"), edge.Source)
		}
	}
}

func TestDiscover_EmitsEachEdgeOnce(t *testing.T) {
	result := discover(t, seeded(), nil)

	// LineageEdge holds a column-lineage slice and so cannot be a map key.
	// Source, target and type are what make an edge unique here.
	type edgeKey struct{ source, target, edgeType string }

	seen := make(map[edgeKey]bool)
	for _, edge := range result.Lineage {
		key := edgeKey{edge.Source, edge.Target, edge.Type}
		assert.False(t, seen[key], "duplicate edge %v", edge)
		seen[key] = true
	}
}

func TestDiscover_EveryEdgeTouchesAnEmittedAssetOrANativeTable(t *testing.T) {
	result := discover(t, seeded(), nil)
	require.NotEmpty(t, result.Lineage)

	known := make(map[string]bool, len(result.Assets))
	for _, a := range result.Assets {
		known[*a.MRN] = true
	}

	for _, edge := range result.Lineage {
		assert.True(t, known[edge.Target], "edge target %q is not an emitted asset", edge.Target)
		if !strings.HasPrefix(edge.Source, "mrn://table/") {
			assert.True(t, known[edge.Source], "edge source %q is not an emitted asset", edge.Source)
		}
	}
}

// Toggles

func TestDiscover_IncludePanelsOffKeepsDashboardsAndTheirTableEdges(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"include_panels": false})

	for _, a := range result.Assets {
		assert.NotEqual(t, "Chart", a.Type)
	}
	dash := findAsset(result, "Dashboard", "Sales/Orders overview")
	require.NotNil(t, dash)
	assert.Equal(t, 3, dash.Metadata["panel_count"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", *dash.MRN, "FEEDS"))
}

func TestDiscover_IncludeDatasourcesOffDropsTheAssetsAndTheirEdges(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"include_datasources": false})

	for _, a := range result.Assets {
		assert.NotEqual(t, "DataSource", a.Type)
	}
	for _, edge := range result.Lineage {
		assert.False(t, strings.HasPrefix(edge.Source, "mrn://datasource/"), edge.Source)
	}

	chart := findAsset(result, "Chart", "Sales/Orders overview/Revenue")
	require.NotNil(t, chart)
	assert.Equal(t, "Shop", chart.Metadata["datasource"], "charts still say which source they query")
}

func TestDiscover_DiscoverLineageOffDropsTableEdgesOnly(t *testing.T) {
	result := discover(t, seeded(), pluginsdk.RawConfig{"discover_lineage": false})

	for _, edge := range result.Lineage {
		assert.False(t, strings.HasPrefix(edge.Source, "mrn://table/"), edge.Source)
	}
	assert.True(t, hasEdge(result, "mrn://dashboard/grafana/sales-orders-overview", "mrn://chart/grafana/sales-orders-overview-revenue", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://datasource/grafana/shop", "mrn://chart/grafana/sales-orders-overview-revenue", "FEEDS"))
}

// Resilience

func TestDiscover_FollowsSearchPaginationToTheEnd(t *testing.T) {
	f := seeded()

	result := discover(t, f, pluginsdk.RawConfig{"page_size": 1})

	assert.NotNil(t, findAsset(result, "Dashboard", "Ops home"))
	assert.NotNil(t, findAsset(result, "Dashboard", "Sales/Orders overview"))
	assert.Equal(t, 3, f.countRequests("/api/search?"), "two full pages and the empty one that ends the listing")
}

func TestDiscover_SurvivesADashboardThatCannotBeRead(t *testing.T) {
	f := seeded().broken("ops-home")

	result := discover(t, f, nil)

	assert.Nil(t, findAsset(result, "Dashboard", "Ops home"))
	assert.NotNil(t, findAsset(result, "Dashboard", "Sales/Orders overview"))
}

func TestDiscover_SurvivesDataSourcesTheTokenMayNotList(t *testing.T) {
	f := seeded().withoutDatasourceAccess()

	result := discover(t, f, nil)

	assert.Nil(t, findAsset(result, "DataSource", "Shop"))
	chart := findAsset(result, "Chart", "Sales/Orders overview/Revenue")
	require.NotNil(t, chart)
	assert.NotContains(t, chart.Metadata, "datasource", "the name is unknown without the listing")
	assert.Equal(t, "grafana-postgresql-datasource", chart.Metadata["datasource_type"], "the reference still carries the type")
	assert.Equal(t, shopUID, chart.Metadata["datasource_uid"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", *chart.MRN, "FEEDS"),
		"SQL lineage only needs the type, which the reference carries")
}

func TestDiscover_FailsOnAWrongToken(t *testing.T) {
	server := seeded().start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL, "api_key": "glsa_wrong"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
	assert.NotContains(t, err.Error(), "glsa_wrong", "the token never appears in an error")
}

func TestDiscover_FailsWhenTheHostIsNotGrafana(t *testing.T) {
	server := seeded().start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL + "/not-grafana", "api_key": fakeToken})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing dashboards")
}

func TestDiscover_TrimsATrailingSlashOffTheHost(t *testing.T) {
	// Without the trim every request would go to //api/..., which the
	// fake (like Grafana) does not serve.
	f := seeded()
	server := f.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL + "/", "api_key": fakeToken})

	require.NoError(t, err)
	assert.NotNil(t, findAsset(result, "Dashboard", "Ops home"))
}

func TestDiscover_AnEmptyGrafanaYieldsOnlyDataSources(t *testing.T) {
	result := discover(t, newFakeGrafana().withJSON(nil, datasourcesJSON), nil)

	require.Len(t, result.Assets, 2)
	assert.Equal(t, "DataSource", result.Assets[0].Type)
	assert.Empty(t, result.Lineage)
}
