package redash

import (
	"strconv"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shopDataSource is the Postgres connection the other fixtures query.
func shopDataSource(fake *fakeRedash) *fakeRedash {
	return fake.withDataSource(1, "shop", "pg", "sql", map[string]any{
		"host": "db.internal", "port": 5432, "dbname": "shop", "user": "marmot",
		// Redash redacts the password but still sends the key.
		"password": "--------",
	})
}

const ordersSQL = "SELECT date_trunc('day', o.created_at) AS day, count(*) FROM public.orders o JOIN public.customers c ON c.id = o.customer_id WHERE o.total > {{min_total}} GROUP BY 1"

// revenueDashboard is a published dashboard with a column chart, a counter
// and two text widgets.
func revenueDashboard(fake *fakeRedash) *fakeRedash {
	query := queryPayload(1, "Orders by day", "Daily order counts", ordersSQL, 1)
	return fake.withDashboard(
		dashboardPayload(1, "Revenue", "revenue"),
		visualizationWidget(1, 4, "CHART", "Orders by day", "Column chart of daily orders",
			map[string]any{"globalSeriesType": "column", "columnMapping": map[string]any{"day": "x"}}, query),
		textWidget(3, "Revenue overview for the shop database."),
		textWidget(4, "Updated nightly."),
	)
}

func fullFake() *fakeRedash {
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withQuery(queryPayload(1, "Orders by day", "Daily order counts", ordersSQL, 1))
	revenueDashboard(fake)
	return fake
}

// Validate

func TestValidate_RequiresHost(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"api_key": "k"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestValidate_RequiresAPIKey(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://redash.example.com"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestValidate_RejectsHostThatIsNotAURL(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"host": "not a url", "api_key": "k"})

	require.Error(t, err)
}

func TestValidate_TrimsTrailingSlashFromHost(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://redash.example.com/", "api_key": "k"})

	require.NoError(t, err)
	assert.Equal(t, "https://redash.example.com", source.config.Host)
}

func TestValidate_DefaultsPageSizeTo100(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://redash.example.com", "api_key": "k"})

	require.NoError(t, err)
	assert.Equal(t, 100, source.config.PageSize)
}

func TestValidate_DefaultsTheIncludeTogglesOn(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://redash.example.com", "api_key": "k"})

	require.NoError(t, err)
	assert.True(t, source.config.IncludeQueries)
	assert.True(t, source.config.IncludeDataSources)
	assert.True(t, source.config.IncludeDrafts)
	assert.True(t, source.config.DiscoverLineage)
	assert.True(t, source.config.VerifySSL)
}

func TestValidate_DefaultsIncludeArchivedOff(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://redash.example.com", "api_key": "k"})

	require.NoError(t, err)
	assert.False(t, source.config.IncludeArchived)
}

func TestValidate_KeepsAnExplicitFalseToggle(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://redash.example.com", "api_key": "k", "include_queries": false,
	})

	require.NoError(t, err)
	assert.False(t, source.config.IncludeQueries)
}

func TestValidate_RejectsPageSizeAboveTheRedashLimit(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://redash.example.com", "api_key": "k", "page_size": 251,
	})

	require.Error(t, err)
}

func TestValidate_AcceptsThePageSizeAtTheRedashLimit(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://redash.example.com", "api_key": "k", "page_size": 250,
	})

	require.NoError(t, err)
}

// Meta

func TestMeta_DeclaresTheRedashIdentity(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "redash", meta.ID)
	assert.Equal(t, "Redash", meta.Name)
	assert.Equal(t, "redash", meta.Icon)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
}

func TestMeta_DeclaresAssetsAndLineage(t *testing.T) {
	assert.Equal(t, []string{"Assets", "Lineage"}, Meta().Features)
}

func TestMeta_ExposesTheHostAndAPIKeyFields(t *testing.T) {
	var names []string
	for _, field := range Meta().ConfigSpec {
		names = append(names, field.Name)
	}

	assert.Contains(t, names, "host")
	assert.Contains(t, names, "api_key")
}

// Dashboards

func TestDiscover_CreatesADashboardAsset(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	dashboard := assetNamed(t, result, "Dashboard", "Revenue")

	assert.Equal(t, []string{"Redash"}, dashboard.Providers)
	assert.Equal(t, "mrn://dashboard/redash/revenue", *dashboard.MRN)
	assert.Equal(t, 1, dashboard.Metadata["id"])
	assert.Equal(t, "revenue", dashboard.Metadata["slug"])
	assert.Equal(t, "Marmot Admin", dashboard.Metadata["owner"])
}

func TestDiscover_DashboardNotesComeFromTextWidgets(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	dashboard := assetNamed(t, result, "Dashboard", "Revenue")

	assert.Equal(t, "Revenue overview for the shop database.\n\nUpdated nightly.", dashboard.Metadata["notes"])
}

func TestDiscover_DashboardHasNoDescription(t *testing.T) {
	// Redash dashboards have no description field. The OpenMetadata
	// connector promotes the last text widget into one, which silently
	// drops the others and mislabels a note as documentation.
	result := discoverAgainst(t, fullFake(), nil)

	assert.Nil(t, assetNamed(t, result, "Dashboard", "Revenue").Description)
}

func TestDiscover_DashboardCountsWidgetsAndQueries(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	dashboard := assetNamed(t, result, "Dashboard", "Revenue")

	assert.Equal(t, 3, dashboard.Metadata["widget_count"])
	assert.Equal(t, 1, dashboard.Metadata["query_count"])
}

func TestDiscover_DashboardURLCarriesIDAndSlugOnCurrentRedash(t *testing.T) {
	fake := fullFake()
	fake.version = "25.1.0"
	host := fake.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": host, "api_key": "test-key"})
	require.NoError(t, err)

	assert.Equal(t, host+"/dashboards/1-revenue", assetNamed(t, result, "Dashboard", "Revenue").Metadata["url"])
}

func TestDiscover_DashboardURLIsTheBareSlugOnRedash8(t *testing.T) {
	fake := fullFake()
	fake.version = "8.0.0"
	host := fake.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": host, "api_key": "test-key"})
	require.NoError(t, err)

	assert.Equal(t, host+"/dashboards/revenue", assetNamed(t, result, "Dashboard", "Revenue").Metadata["url"])
}

func TestDiscover_DashboardRecordsTheDetectedVersion(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	assert.Equal(t, "25.1.0", assetNamed(t, result, "Dashboard", "Revenue").Metadata["redash_version"])
}

func TestDiscover_DashboardLinksBackToRedash(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	links := assetNamed(t, result, "Dashboard", "Revenue").ExternalLinks

	require.Len(t, links, 1)
	assert.Equal(t, "Open in Redash", links[0].Name)
}

func TestDiscover_ReadsDashboardDetailBySlugWhenTheIDIsRejected(t *testing.T) {
	// Redash 8 keys the detail endpoint on the slug and errors on a
	// numeric id, so the fetch falls back rather than losing the widgets.
	fake := fullFake()
	fake.detailByIDFails = true

	result := discoverAgainst(t, fake, nil)

	assert.Equal(t, 3, assetNamed(t, result, "Dashboard", "Revenue").Metadata["widget_count"])
}

func TestDiscover_QualifiesDuplicateDashboardNamesWithTheRedashID(t *testing.T) {
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withDashboard(dashboardPayload(1, "Revenue", "revenue"))
	fake.withDashboard(dashboardPayload(2, "Revenue", "revenue-2"))

	result := discoverAgainst(t, fake, nil)

	names := []string{*assetsOfType(result, "Dashboard")[0].Name, *assetsOfType(result, "Dashboard")[1].Name}
	assert.Equal(t, []string{"Revenue", "Revenue (2)"}, names)
}

// Charts

func TestDiscover_CreatesAChartNamedByDashboardAndVisualization(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	chart := assetNamed(t, result, "Chart", "Revenue/Orders by day")

	assert.Equal(t, "mrn://chart/redash/revenue-orders-by-day", *chart.MRN)
	assert.Equal(t, 4, chart.Metadata["visualization_id"])
	assert.Equal(t, 1, chart.Metadata["widget_id"])
}

func TestDiscover_ChartTypeComesFromGlobalSeriesType(t *testing.T) {
	// A Redash CHART says nothing about its shape in the type field; the
	// real shape is in options.globalSeriesType, which the OpenMetadata
	// connector never reads.
	result := discoverAgainst(t, fullFake(), nil)

	chart := assetNamed(t, result, "Chart", "Revenue/Orders by day")

	assert.Equal(t, "CHART", chart.Metadata["visualization_type"])
	assert.Equal(t, "Bar", chart.Metadata["chart_type"])
}

func TestDiscover_ChartRecordsItsQueryAndDataSource(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	chart := assetNamed(t, result, "Chart", "Revenue/Orders by day")

	assert.Equal(t, 1, chart.Metadata["query_id"])
	assert.Equal(t, "Orders by day", chart.Metadata["query_name"])
	assert.Equal(t, "shop", chart.Metadata["data_source"])
}

func TestDiscover_ChartDescriptionComesFromTheVisualization(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	chart := assetNamed(t, result, "Chart", "Revenue/Orders by day")

	require.NotNil(t, chart.Description)
	assert.Equal(t, "Column chart of daily orders", *chart.Description)
}

func TestDiscover_ChartLinksToItsVisualizationNotTheDashboard(t *testing.T) {
	fake := fullFake()
	host := fake.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": host, "api_key": "test-key"})
	require.NoError(t, err)

	chart := assetNamed(t, result, "Chart", "Revenue/Orders by day")
	assert.Equal(t, host+"/queries/1#4", chart.Metadata["url"])
}

func TestDiscover_TextWidgetDoesNotBecomeAChart(t *testing.T) {
	// A text widget has no visualization key at all. Reading one as a chart
	// is what makes the OpenMetadata connector report a failure per note.
	result := discoverAgainst(t, fullFake(), nil)

	assert.Len(t, assetsOfType(result, "Chart"), 1)
}

func TestDiscover_ChartFallsBackToTheQueryNameWhenTheVisualizationIsUnnamed(t *testing.T) {
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withDashboard(
		dashboardPayload(1, "Revenue", "revenue"),
		visualizationWidget(1, 4, "TABLE", "", "", map[string]any{}, queryPayload(1, "Orders by day", "", ordersSQL, 1)),
	)

	result := discoverAgainst(t, fake, nil)

	assetNamed(t, result, "Chart", "Revenue/Orders by day")
}

func TestDiscover_ChartFallsBackToTheWidgetIDWhenNothingIsNamed(t *testing.T) {
	fake := newFakeRedash()
	fake.withDashboard(
		dashboardPayload(1, "Revenue", "revenue"),
		visualizationWidget(7, 4, "TABLE", "", "", map[string]any{}, nil),
	)

	result := discoverAgainst(t, fake, nil)

	assetNamed(t, result, "Chart", "Revenue/widget-7")
}

func TestDiscover_QualifiesTwoChartsWithTheSameNameOnOneDashboard(t *testing.T) {
	query := queryPayload(1, "Orders by day", "", ordersSQL, 1)
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withDashboard(
		dashboardPayload(1, "Revenue", "revenue"),
		visualizationWidget(1, 4, "TABLE", "Orders", "", map[string]any{}, query),
		visualizationWidget(2, 5, "COUNTER", "Orders", "", map[string]any{}, query),
	)

	result := discoverAgainst(t, fake, nil)

	assetNamed(t, result, "Chart", "Revenue/Orders")
	assetNamed(t, result, "Chart", "Revenue/Orders (2)")
}

// Queries

func TestDiscover_CreatesAQueryAsADataModelObject(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	query := assetNamed(t, result, "Data Model Object", "Orders by day")

	assert.Equal(t, "mrn://data-model-object/redash/orders-by-day", *query.MRN)
	assert.Equal(t, 1, query.Metadata["id"])
	assert.Equal(t, "shop", query.Metadata["data_source"])
}

func TestDiscover_QueryCarriesItsSQL(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	query := assetNamed(t, result, "Data Model Object", "Orders by day")

	require.NotNil(t, query.Query)
	assert.Equal(t, ordersSQL, *query.Query)
}

func TestDiscover_QueryLanguageComesFromTheDataSourceSyntax(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	query := assetNamed(t, result, "Data Model Object", "Orders by day")

	require.NotNil(t, query.QueryLanguage)
	assert.Equal(t, "SQL", *query.QueryLanguage)
}

func TestDiscover_QueryLanguageUppercasesANonSQLSyntax(t *testing.T) {
	fake := newFakeRedash()
	fake.withDataSource(1, "logs", "elasticsearch", "json", map[string]any{"server": "http://es:9200"})
	fake.withQuery(queryPayload(1, "Errors", "", `{"query":{"match_all":{}}}`, 1))

	result := discoverAgainst(t, fake, nil)

	query := assetNamed(t, result, "Data Model Object", "Errors")
	require.NotNil(t, query.QueryLanguage)
	assert.Equal(t, "JSON", *query.QueryLanguage)
}

func TestDiscover_QueryDescriptionIsSetOnTheAsset(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	query := assetNamed(t, result, "Data Model Object", "Orders by day")

	require.NotNil(t, query.Description)
	assert.Equal(t, "Daily order counts", *query.Description)
}

func TestDiscover_QueryRecordsItsParameters(t *testing.T) {
	payload := queryPayload(1, "Orders by day", "", ordersSQL, 1)
	payload["options"] = map[string]any{"parameters": []map[string]any{
		{"name": "min_total", "title": "Min total", "type": "number", "value": 0},
	}}
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withQuery(payload)

	result := discoverAgainst(t, fake, nil)

	parameters := assetNamed(t, result, "Data Model Object", "Orders by day").Metadata["parameters"]
	assert.Equal(t, []map[string]any{{"name": "min_total", "title": "Min total", "type": "number"}}, parameters)
}

func TestDiscover_QueryRecordsItsSchedule(t *testing.T) {
	payload := queryPayload(1, "Orders by day", "", ordersSQL, 1)
	payload["schedule"] = map[string]any{"interval": 86400, "time": "03:00", "day_of_week": nil, "until": nil}
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withQuery(payload)

	result := discoverAgainst(t, fake, nil)

	schedule := assetNamed(t, result, "Data Model Object", "Orders by day").Metadata["schedule"]
	assert.Equal(t, map[string]any{"interval": 86400, "time": "03:00"}, schedule)
}

func TestDiscover_QueryNeverRecordsItsAPIKey(t *testing.T) {
	// Redash returns a per-query api key on every query payload. It is a
	// credential and must not reach the catalog.
	result := discoverAgainst(t, fullFake(), nil)

	query := assetNamed(t, result, "Data Model Object", "Orders by day")

	for key, value := range query.Metadata {
		assert.NotContains(t, key, "api_key")
		assert.NotEqual(t, "aCtrbcSVjggRc7BitHVvl1M5GtBwt2tT8XVEK2r5", value)
	}
}

func TestDiscover_OmitsQueriesWhenTurnedOff(t *testing.T) {
	result := discoverAgainst(t, fullFake(), pluginsdk.RawConfig{"include_queries": false})

	assert.Empty(t, assetsOfType(result, "Data Model Object"))
}

// Data sources

func TestDiscover_CreatesADataSourceAsset(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	source := assetNamed(t, result, "DataSource", "shop")

	assert.Equal(t, "mrn://datasource/redash/shop", *source.MRN)
	assert.Equal(t, "pg", source.Metadata["type"])
	assert.Equal(t, "sql", source.Metadata["syntax"])
	assert.Equal(t, "db.internal", source.Metadata["host"])
	assert.Equal(t, "shop", source.Metadata["dbname"])
	assert.Equal(t, "marmot", source.Metadata["user"])
}

func TestDiscover_DataSourceNeverRecordsAPassword(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	source := assetNamed(t, result, "DataSource", "shop")

	for key, value := range source.Metadata {
		assert.NotContains(t, key, "password")
		assert.NotEqual(t, "--------", value)
	}
}

func TestDiscover_DataSourceDropsUnknownConnectionOptions(t *testing.T) {
	// Options pass through an allowlist, so an option a future runner adds
	// cannot leak a secret into the catalog.
	fake := newFakeRedash()
	fake.withDataSource(1, "shop", "pg", "sql", map[string]any{
		"host": "db.internal", "jsonKeyFile": "{\"private_key\":\"secret\"}", "aws_secret_key": "AKIA",
	})

	result := discoverAgainst(t, fake, nil)

	source := assetNamed(t, result, "DataSource", "shop")
	assert.NotContains(t, source.Metadata, "jsonKeyFile")
	assert.NotContains(t, source.Metadata, "aws_secret_key")
	assert.Equal(t, "db.internal", source.Metadata["host"])
}

func TestDiscover_DataSourcePausedDecodesFromTheRedisIntegerRedashSends(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	assert.Equal(t, false, assetNamed(t, result, "DataSource", "shop").Metadata["paused"])
}

func TestDiscover_OmitsDataSourcesWhenTurnedOff(t *testing.T) {
	result := discoverAgainst(t, fullFake(), pluginsdk.RawConfig{"include_data_sources": false})

	assert.Empty(t, assetsOfType(result, "DataSource"))
}

func TestDiscover_StillNamesTheDataSourceOnAQueryWhenDataSourceAssetsAreOff(t *testing.T) {
	// The data source is still read, because a query's language and the
	// identity of the tables it reads both come from it.
	result := discoverAgainst(t, fullFake(), pluginsdk.RawConfig{"include_data_sources": false})

	assert.Equal(t, "shop", assetNamed(t, result, "Data Model Object", "Orders by day").Metadata["data_source"])
}

// Drafts and archives

func TestDiscover_IncludesDraftDashboardsByDefault(t *testing.T) {
	fake := newFakeRedash()
	draft := dashboardPayload(2, "Scratch", "scratch")
	draft["is_draft"] = true
	fake.withDashboard(draft)

	result := discoverAgainst(t, fake, nil)

	assetNamed(t, result, "Dashboard", "Scratch")
}

func TestDiscover_SkipsDraftDashboardsWhenDraftsAreExcluded(t *testing.T) {
	fake := newFakeRedash()
	draft := dashboardPayload(2, "Scratch", "scratch")
	draft["is_draft"] = true
	fake.withDashboard(draft)

	result := discoverAgainst(t, fake, pluginsdk.RawConfig{"include_drafts": false})

	assert.Empty(t, assetsOfType(result, "Dashboard"))
}

func TestDiscover_SkipsDraftQueriesWhenDraftsAreExcluded(t *testing.T) {
	draft := queryPayload(2, "Customer totals", "", "SELECT 1 FROM customers", 1)
	draft["is_draft"] = true
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withQuery(draft)

	result := discoverAgainst(t, fake, pluginsdk.RawConfig{"include_drafts": false})

	assert.Empty(t, assetsOfType(result, "Data Model Object"))
}

func TestDiscover_SkipsArchivedQueriesByDefault(t *testing.T) {
	archived := queryPayload(3, "Old report", "", "SELECT 1 FROM public.customers", 1)
	archived["is_archived"] = true
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withArchivedQuery(archived)

	result := discoverAgainst(t, fake, nil)

	assert.Empty(t, assetsOfType(result, "Data Model Object"))
}

func TestDiscover_ReadsTheArchiveEndpointWhenArchivedIsIncluded(t *testing.T) {
	// Redash keeps archived queries out of /api/queries entirely and serves
	// them from /api/queries/archive.
	archived := queryPayload(3, "Old report", "", "SELECT 1 FROM public.customers", 1)
	archived["is_archived"] = true
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withArchivedQuery(archived)

	result := discoverAgainst(t, fake, pluginsdk.RawConfig{"include_archived": true})

	assetNamed(t, result, "Data Model Object", "Old report")
}

// Lineage

func TestDiscover_DashboardContainsItsCharts(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://dashboard/redash/revenue", "mrn://chart/redash/revenue-orders-by-day", "CONTAINS"))
}

func TestDiscover_QueryFeedsTheChartThatRendersIt(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://data-model-object/redash/orders-by-day", "mrn://chart/redash/revenue-orders-by-day", "FEEDS"))
}

func TestDiscover_DataSourceFeedsTheQueriesThatRunOnIt(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://datasource/redash/shop", "mrn://data-model-object/redash/orders-by-day", "FEEDS"))
}

func TestDiscover_TablesFeedTheQueryThatReadsThem(t *testing.T) {
	// The table MRNs are the ones plugins/postgresql produces, so the edge
	// lands on that plugin's asset instead of minting a second one.
	result := discoverAgainst(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://data-model-object/redash/orders-by-day", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", "mrn://data-model-object/redash/orders-by-day", "FEEDS"))
}

func TestDiscover_EmitsNoLineageWhenTurnedOff(t *testing.T) {
	result := discoverAgainst(t, fullFake(), pluginsdk.RawConfig{"discover_lineage": false})

	assert.Empty(t, result.Lineage)
}

func TestDiscover_EmitsNoTableEdgeForAnUnknownDataSourceType(t *testing.T) {
	fake := newFakeRedash()
	fake.withDataSource(1, "sheet", "google_spreadsheets", "sql", map[string]any{})
	fake.withQuery(queryPayload(1, "Sheet rows", "", "SELECT * FROM orders", 1))

	result := discoverAgainst(t, fake, nil)

	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Source, "mrn://table/")
	}
}

func TestDiscover_EveryLineageEndpointIsAnAssetOfThisRunOrAnotherPluginsTable(t *testing.T) {
	// The Marmot server drops an edge whose endpoint does not exist, so an
	// edge may only point at an asset this run created or at a table named
	// the way its owning plugin names it.
	result := discoverAgainst(t, fullFake(), nil)

	own := make(map[string]bool)
	for _, asset := range result.Assets {
		own[*asset.MRN] = true
	}

	require.NotEmpty(t, result.Lineage)
	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			if own[endpoint] {
				continue
			}
			assert.Contains(t, endpoint, "mrn://table/postgresql/", "edge endpoint %q is neither ours nor a known table", endpoint)
		}
	}
}

// Transport

func TestDiscover_FollowsPaginationToTheEnd(t *testing.T) {
	fake := newFakeRedash()
	shopDataSource(fake)
	for id := 1; id <= 7; id++ {
		fake.withDashboard(dashboardPayload(id, "Dashboard "+strconv.Itoa(id), "dashboard-"+strconv.Itoa(id)))
	}

	result := discoverAgainst(t, fake, pluginsdk.RawConfig{"page_size": 2})

	assert.Len(t, assetsOfType(result, "Dashboard"), 7)
}

func TestDiscover_FailsWhenTheDashboardListIsUnreachable(t *testing.T) {
	fake := fullFake()
	fake.dashboardsFail = true

	_, err := tryDiscoverAgainst(t, fake, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing dashboards")
}

func TestDiscover_FailsWithAHelpfulMessageOnABadAPIKey(t *testing.T) {
	fake := fullFake()

	_, err := tryDiscoverAgainst(t, fake, pluginsdk.RawConfig{"api_key": "wrong"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check the api_key")
}

func TestDiscover_ContinuesWhenTheVersionProbeFails(t *testing.T) {
	// Only the dashboard URL shape depends on the version, so a Redash that
	// will not answer /api/session still gets catalogued.
	fake := fullFake()
	fake.sessionFails = true

	result := discoverAgainst(t, fake, nil)

	assetNamed(t, result, "Dashboard", "Revenue")
}

func TestDiscover_NeverPutsTheAPIKeyInAnError(t *testing.T) {
	fake := fullFake()
	fake.dashboardsFail = true

	_, err := tryDiscoverAgainst(t, fake, nil)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "test-key")
}

// Chart type normalisation

func TestChartType_TableIsATable(t *testing.T) {
	assert.Equal(t, "Table", chartType("TABLE", nil))
}

func TestChartType_PivotIsATable(t *testing.T) {
	assert.Equal(t, "Table", chartType("PIVOT", nil))
}

func TestChartType_DetailsIsATable(t *testing.T) {
	assert.Equal(t, "Table", chartType("DETAILS", nil))
}

func TestChartType_CounterIsText(t *testing.T) {
	assert.Equal(t, "Text", chartType("COUNTER", nil))
}

func TestChartType_MapIsAMap(t *testing.T) {
	assert.Equal(t, "Map", chartType("MAP", nil))
}

func TestChartType_ChoroplethIsAMap(t *testing.T) {
	assert.Equal(t, "Map", chartType("CHOROPLETH", nil))
}

func TestChartType_BoxplotIsABoxPlot(t *testing.T) {
	assert.Equal(t, "BoxPlot", chartType("BOXPLOT", nil))
}

func TestChartType_SankeyIsASanKey(t *testing.T) {
	assert.Equal(t, "SanKey", chartType("SANKEY", nil))
}

func TestChartType_FunnelIsOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("FUNNEL", nil))
}

func TestChartType_WordCloudIsOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("WORD_CLOUD", nil))
}

func TestChartType_LineSeriesIsALine(t *testing.T) {
	assert.Equal(t, "Line", chartType("CHART", map[string]any{"globalSeriesType": "line"}))
}

func TestChartType_ColumnSeriesIsABar(t *testing.T) {
	assert.Equal(t, "Bar", chartType("CHART", map[string]any{"globalSeriesType": "column"}))
}

func TestChartType_BarSeriesIsABar(t *testing.T) {
	assert.Equal(t, "Bar", chartType("CHART", map[string]any{"globalSeriesType": "bar"}))
}

func TestChartType_PieSeriesIsAPie(t *testing.T) {
	assert.Equal(t, "Pie", chartType("CHART", map[string]any{"globalSeriesType": "pie"}))
}

func TestChartType_AreaSeriesIsAnArea(t *testing.T) {
	assert.Equal(t, "Area", chartType("CHART", map[string]any{"globalSeriesType": "area"}))
}

func TestChartType_ScatterSeriesIsAScatter(t *testing.T) {
	assert.Equal(t, "Scatter", chartType("CHART", map[string]any{"globalSeriesType": "scatter"}))
}

func TestChartType_BoxSeriesIsABoxPlot(t *testing.T) {
	assert.Equal(t, "BoxPlot", chartType("CHART", map[string]any{"globalSeriesType": "box"}))
}

func TestChartType_ChartWithNoSeriesTypeIsOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("CHART", map[string]any{}))
}

func TestChartType_UnknownVisualizationIsOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("SUNBURST_SEQUENCE", nil))
}

// Version parsing

func TestParseVersion_ReadsTheMajorNumber(t *testing.T) {
	assert.Equal(t, 25, parseVersion("25.1.0").major)
}

func TestParseVersion_KeepsTheRawString(t *testing.T) {
	assert.Equal(t, "10.1.0+b50633", parseVersion("10.1.0+b50633").raw)
}

func TestParseVersion_TreatsAnEmptyVersionAsUndetected(t *testing.T) {
	assert.Equal(t, 0, parseVersion("").major)
}

func TestVersion_UndetectedCountsAsCurrent(t *testing.T) {
	assert.True(t, version{}.atLeast(10))
}

func TestVersion_Redash8IsNotAtLeast10(t *testing.T) {
	assert.False(t, parseVersion("8.0.0").atLeast(10))
}

// Query language

func TestQueryLanguage_SQLIsUppercased(t *testing.T) {
	assert.Equal(t, "SQL", queryLanguage("sql"))
}

func TestQueryLanguage_OtherSyntaxesAreUppercased(t *testing.T) {
	assert.Equal(t, "CYPHER", queryLanguage("cypher"))
}

func TestQueryLanguage_AnUnknownSyntaxYieldsNothing(t *testing.T) {
	assert.Equal(t, "", queryLanguage(""))
}

func TestDiscover_DoesNotCatalogueAQueryTwiceWhenBothListsReturnIt(t *testing.T) {
	archived := queryPayload(3, "Old report", "", "SELECT 1 FROM public.customers", 1)
	archived["is_archived"] = true
	fake := newFakeRedash()
	shopDataSource(fake)
	fake.withQuery(archived)
	fake.withArchivedQuery(archived)

	result := discoverAgainst(t, fake, pluginsdk.RawConfig{"include_archived": true})

	assert.Len(t, assetsOfType(result, "Data Model Object"), 1)
}
