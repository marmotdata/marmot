package grafana

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenPanels_HoistsPanelsOutOfCollapsedRows(t *testing.T) {
	panels := []panel{
		{ID: 1, Type: "timeseries", Title: "Revenue"},
		{ID: 2, Type: "row", Title: "Details", Collapsed: true, Panels: []panel{
			{ID: 3, Type: "table", Title: "Top customers"},
		}},
		{ID: 4, Type: "stat", Title: "Random"},
	}

	flat := flattenPanels(panels)

	require.Len(t, flat, 3)
	assert.Equal(t, int64(1), flat[0].ID)
	assert.Equal(t, int64(3), flat[1].ID, "the collapsed row's panel takes the row's place")
	assert.Equal(t, int64(4), flat[2].ID)
}

func TestFlattenPanels_DropsExpandedRows(t *testing.T) {
	panels := []panel{
		{ID: 1, Type: "row", Title: "Overview"},
		{ID: 2, Type: "timeseries", Title: "Revenue"},
	}

	flat := flattenPanels(panels)

	require.Len(t, flat, 1)
	assert.Equal(t, "Revenue", flat[0].Title)
}

func TestIsChartPanel_SkipsTextRowsAndUntypedPanels(t *testing.T) {
	assert.True(t, isChartPanel(panel{Type: "timeseries"}))
	assert.False(t, isChartPanel(panel{Type: "text"}))
	assert.False(t, isChartPanel(panel{Type: "row"}))
	assert.False(t, isChartPanel(panel{Type: ""}))
}

func TestChartType_GraphsAreLines(t *testing.T) {
	assert.Equal(t, "Line", chartType("graph"))
	assert.Equal(t, "Line", chartType("timeseries"))
}

func TestChartType_TablesLogsAndAlertListsAreTables(t *testing.T) {
	assert.Equal(t, "Table", chartType("table"))
	assert.Equal(t, "Table", chartType("alertlist"))
	assert.Equal(t, "Table", chartType("logs"))
}

func TestChartType_StatsAreText(t *testing.T) {
	assert.Equal(t, "Text", chartType("stat"))
	assert.Equal(t, "Text", chartType("text"))
	assert.Equal(t, "Text", chartType("news"))
}

func TestChartType_Gauge(t *testing.T) {
	assert.Equal(t, "Gauge", chartType("gauge"))
}

func TestChartType_BarsInEveryForm(t *testing.T) {
	assert.Equal(t, "Bar", chartType("bargauge"))
	assert.Equal(t, "Bar", chartType("barchart"))
	assert.Equal(t, "Bar", chartType("bar"))
}

func TestChartType_Pie(t *testing.T) {
	assert.Equal(t, "Pie", chartType("piechart"))
}

func TestChartType_HeatmapAndHistogram(t *testing.T) {
	assert.Equal(t, "Heatmap", chartType("heatmap"))
	assert.Equal(t, "Histogram", chartType("histogram"))
}

func TestChartType_MapAndGraph(t *testing.T) {
	assert.Equal(t, "Map", chartType("geomap"))
	assert.Equal(t, "Graph", chartType("nodeGraph"))
}

func TestChartType_Timelines(t *testing.T) {
	assert.Equal(t, "Timeline", chartType("state-timeline"))
	assert.Equal(t, "Timeline", chartType("status-history"))
}

func TestChartType_UnknownPanelsAreOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("canvas"))
	assert.Equal(t, "Other", chartType("grafana-polystat-panel"))
}

func TestPanelName_FallsBackToThePanelID(t *testing.T) {
	assert.Equal(t, "Revenue", panelName(panel{ID: 1, Title: " Revenue "}))
	assert.Equal(t, "panel-7", panelName(panel{ID: 7, Title: ""}))
}

// Data source resolution

func shopAndTestData() *datasourceIndex {
	return indexDatasources([]datasource{
		{UID: "ffxkla6ojkvlsf", Name: "Shop", Type: "grafana-postgresql-datasource", IsDefault: true},
		{UID: "efxkla7l5962oe", Name: "TestData", Type: "grafana-testdata-datasource"},
	})
}

func TestResolve_AMissingReferenceIsTheDefaultDataSource(t *testing.T) {
	ds := shopAndTestData().resolve(nil)

	require.NotNil(t, ds)
	assert.Equal(t, "Shop", ds.Name)
}

func TestResolve_AnEmptyReferenceIsTheDefaultDataSource(t *testing.T) {
	ds := shopAndTestData().resolve(&datasourceRef{})

	require.NotNil(t, ds)
	assert.Equal(t, "Shop", ds.Name)
}

func TestResolve_ByUID(t *testing.T) {
	ds := shopAndTestData().resolve(&datasourceRef{Type: "grafana-testdata-datasource", UID: "efxkla7l5962oe"})

	require.NotNil(t, ds)
	assert.Equal(t, "TestData", ds.Name)
}

func TestResolve_ByNameForDashboardsOlderThanGrafana83(t *testing.T) {
	// Before 8.3 a dashboard named its data source with a bare string
	// holding the name, which decodes into the uid slot.
	ds := shopAndTestData().resolve(&datasourceRef{UID: "TestData"})

	require.NotNil(t, ds)
	assert.Equal(t, "TestData", ds.Name)
}

func TestResolve_PseudoDataSourcesResolveToNothing(t *testing.T) {
	ix := shopAndTestData()

	assert.Nil(t, ix.resolve(&datasourceRef{Type: "datasource", UID: "-- Mixed --"}))
	assert.Nil(t, ix.resolve(&datasourceRef{Type: "datasource", UID: "-- Dashboard --"}))
	assert.Nil(t, ix.resolve(&datasourceRef{Type: "datasource", UID: "grafana"}))
	assert.Nil(t, ix.resolve(&datasourceRef{UID: "-- Grafana --"}))
	assert.Nil(t, ix.resolve(&datasourceRef{Type: "__expr__", UID: "__expr__"}))
}

func TestResolve_AnUnknownUIDResolvesToNothing(t *testing.T) {
	assert.Nil(t, shopAndTestData().resolve(&datasourceRef{Type: "prometheus", UID: "gone"}))
	assert.Nil(t, shopAndTestData().resolve(&datasourceRef{Type: "prometheus", UID: "${DS_PROM}"}))
}

func TestResolve_WithoutADefaultAMissingReferenceResolvesToNothing(t *testing.T) {
	ix := indexDatasources(nil)

	assert.Nil(t, ix.resolve(nil))
}

func TestDatasourceRef_DecodesBothTheObjectAndTheStringForm(t *testing.T) {
	var object datasourceRef
	require.NoError(t, json.Unmarshal([]byte(`{"type":"prometheus","uid":"abc"}`), &object))
	assert.Equal(t, datasourceRef{Type: "prometheus", UID: "abc"}, object)

	var str datasourceRef
	require.NoError(t, json.Unmarshal([]byte(`"Prometheus"`), &str))
	assert.Equal(t, datasourceRef{UID: "Prometheus"}, str)
}

func TestFlexString_ReadsRefreshFalseAsOff(t *testing.T) {
	var dash dashboard
	require.NoError(t, json.Unmarshal([]byte(`{"refresh": false, "title": "x"}`), &dash))
	assert.Equal(t, "", string(dash.Refresh))

	require.NoError(t, json.Unmarshal([]byte(`{"refresh": "30s", "title": "x"}`), &dash))
	assert.Equal(t, "30s", string(dash.Refresh))
}

func TestFlexString_ReadsAnObjectQueryAsEmpty(t *testing.T) {
	var tgt target
	require.NoError(t, json.Unmarshal([]byte(`{"refId":"A","query":{"bucket":"b"}}`), &tgt))
	assert.Equal(t, "", string(tgt.Query))
}

// Queries

func TestQueriesOf_ATargetsOwnDataSourceWinsOverThePanels(t *testing.T) {
	p := panel{
		Datasource: &datasourceRef{Type: "datasource", UID: "-- Mixed --"},
		Targets: []target{
			{RefID: "A", Datasource: &datasourceRef{Type: "grafana-postgresql-datasource", UID: "ffxkla6ojkvlsf"}},
			{RefID: "B", Datasource: &datasourceRef{Type: "grafana-testdata-datasource", UID: "efxkla7l5962oe"}},
		},
	}

	queries := queriesOf(p, shopAndTestData())

	require.Len(t, queries, 2)
	assert.Equal(t, "Shop", queries[0].ds.Name)
	assert.Equal(t, "TestData", queries[1].ds.Name)
}

func TestQueriesOf_ATargetWithoutADataSourceUsesThePanels(t *testing.T) {
	p := panel{
		Datasource: &datasourceRef{Type: "grafana-testdata-datasource", UID: "efxkla7l5962oe"},
		Targets:    []target{{RefID: "A"}},
	}

	queries := queriesOf(p, shopAndTestData())

	require.Len(t, queries, 1)
	assert.Equal(t, "TestData", queries[0].ds.Name)
}

func TestPanelQuery_TypeComesFromTheReferenceWhenTheSourceIsUnknown(t *testing.T) {
	q := panelQuery{ref: &datasourceRef{Type: "mysql", UID: "gone"}}

	assert.Equal(t, "mysql", q.datasourceType())
}

func TestQueryOf_AllSQLTargetsRecordTheFirstQuery(t *testing.T) {
	queries := []panelQuery{
		{target: target{RawSQL: "SELECT 1"}},
		{target: target{RawSQL: "SELECT 2"}},
	}

	query, language := queryOf(queries)

	assert.Equal(t, "SELECT 1", query)
	assert.Equal(t, "SQL", language)
}

func TestQueryOf_PrometheusExpressionsArePromQL(t *testing.T) {
	queries := []panelQuery{{
		target: target{Expr: "rate(http_requests_total[5m])"},
		ref:    &datasourceRef{Type: "prometheus", UID: "p"},
	}}

	query, language := queryOf(queries)

	assert.Equal(t, "rate(http_requests_total[5m])", query)
	assert.Equal(t, "PromQL", language)
}

func TestQueryOf_LokiExpressionsAreLogQL(t *testing.T) {
	queries := []panelQuery{{
		target: target{Expr: `{app="api"} |= "error"`},
		ds:     &datasource{Type: "loki"},
	}}

	query, language := queryOf(queries)

	assert.Equal(t, `{app="api"} |= "error"`, query)
	assert.Equal(t, "LogQL", language)
}

func TestQueryOf_AnExpressionOfAnUnknownSourceHasNoLanguage(t *testing.T) {
	queries := []panelQuery{{target: target{Expr: "x"}, ref: &datasourceRef{Type: "grafana-testdata-datasource"}}}

	_, language := queryOf(queries)

	assert.Equal(t, "", language)
}

func TestQueryOf_MixedLanguagesRecordNothing(t *testing.T) {
	queries := []panelQuery{
		{target: target{RawSQL: "SELECT 1"}},
		{target: target{Expr: "up"}, ref: &datasourceRef{Type: "prometheus"}},
	}

	query, language := queryOf(queries)

	assert.Equal(t, "", query)
	assert.Equal(t, "", language)
}

func TestQueryOf_NoTargetsRecordNothing(t *testing.T) {
	query, language := queryOf(nil)

	assert.Equal(t, "", query)
	assert.Equal(t, "", language)
}

func TestQueryOf_ClickHouseQueryFieldIsSQL(t *testing.T) {
	queries := []panelQuery{{
		target: target{Query: "SELECT count() FROM events"},
		ref:    &datasourceRef{Type: "vertamedia-clickhouse-datasource", UID: "ch"},
	}}

	query, language := queryOf(queries)

	assert.Equal(t, "SELECT count() FROM events", query)
	assert.Equal(t, "SQL", language)
}

func TestQueryOf_QueryFieldOfOtherSourcesIsNotSQL(t *testing.T) {
	// Elasticsearch and InfluxDB write query too, but not SQL.
	queries := []panelQuery{{
		target: target{Query: "status:500"},
		ref:    &datasourceRef{Type: "elasticsearch", UID: "es"},
	}}

	query, language := queryOf(queries)

	assert.Equal(t, "", query)
	assert.Equal(t, "", language)
}
