package metabase

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_RequiresHost(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"api_key": "k"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "metabase.example.com", "api_key": "k"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RequiresCredentials(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://metabase.example.com"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

func TestValidate_RequiresAPasswordAlongsideAUsername(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://metabase.example.com", "username": "admin"})

	require.Error(t, err)
}

func TestValidate_AcceptsAnAPIKey(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://metabase.example.com", "api_key": "k"})

	require.NoError(t, err)
}

func TestValidate_AcceptsUsernameAndPassword(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "https://metabase.example.com", "username": "admin", "password": "secret",
	})

	require.NoError(t, err)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://metabase.example.com", "api_key": "k"})
	require.NoError(t, err)

	assert.True(t, source.config.IncludeCharts)
	assert.True(t, source.config.IncludeModels)
	assert.True(t, source.config.DiscoverLineage)
	assert.False(t, source.config.IncludeArchived)
	assert.True(t, source.config.VerifySSL)
}

func TestValidate_KeepsExplicitValues(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://metabase.example.com", "api_key": "k",
		"include_charts": false, "include_archived": true, "verify_ssl": false,
	})
	require.NoError(t, err)

	assert.False(t, source.config.IncludeCharts)
	assert.True(t, source.config.IncludeArchived)
	assert.False(t, source.config.VerifySSL)
	assert.True(t, source.config.IncludeModels, "untouched flags keep their default")
}

func TestValidate_TrimsATrailingSlashFromTheHost(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://metabase.example.com/", "api_key": "k"})
	require.NoError(t, err)

	assert.Equal(t, "https://metabase.example.com", source.config.Host)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "metabase", meta.ID)
	assert.Equal(t, "Metabase", meta.Name)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestDiscover_NamesCardsByTheirCollectionPath(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Equal(t, "Marmot/Finance/Revenue by customer", *chart.Name)
	assert.Equal(t, "Chart", chart.Type)
	assert.Equal(t, []string{"Metabase"}, chart.Providers)
}

func TestDiscover_NamesRootItemsByTheirBareName(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(cardEntry(46, "Sample orders", nil, "question", "pie", 2, mbqlCardQuery(2, 10))).
		withDashboards(dashboardEntry(3, "Root board", nil, 46)),
		nil)

	chart := findAsset(result, "mrn://chart/metabase/sample-orders")
	require.NotNil(t, chart)
	assert.Equal(t, "Sample orders", *chart.Name)
	assert.Nil(t, chart.Metadata["collection"])
	assert.Nil(t, chart.Metadata["collection_id"])

	dash := findAsset(result, "mrn://dashboard/metabase/root-board")
	require.NotNil(t, dash)
	assert.Equal(t, "Root board", *dash.Name)
}

func TestDiscover_NamesDashboardsByTheirCollectionPath(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	dash := findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, dash)
	assert.Equal(t, "Marmot/Finance/Finance overview", *dash.Name)
	assert.Equal(t, "Dashboard", dash.Type)
}

func TestDiscover_CataloguesModelsAsDataModelObjects(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	model := findAsset(result, "mrn://data model object/metabase/marmot-finance-customer-orders")
	require.NotNil(t, model)
	assert.Equal(t, "Data Model Object", model.Type)
	assert.Equal(t, "model", model.Metadata["card_type"])
	require.NotNil(t, model.Query)
	assert.Contains(t, model.ExternalLinks[0].URL, "/model/42-customer-orders")
}

func TestDiscover_CataloguesMetricsAsCharts(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(cardEntry(9, "Revenue", 4, "metric", "line", 2, mbqlCardQuery(2, 10))),
		nil)

	metric := findAsset(result, "mrn://chart/metabase/marmot-revenue")
	require.NotNil(t, metric)
	assert.Equal(t, "Chart", metric.Type)
	assert.Equal(t, "metric", metric.Metadata["card_type"])
}

func TestDiscover_CarriesCardMetadata(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Equal(t, 40, chart.Metadata["id"])
	assert.Equal(t, "bar", chart.Metadata["display"])
	assert.Equal(t, "Bar", chart.Metadata["chart_type"])
	assert.Equal(t, "question", chart.Metadata["card_type"])
	assert.Equal(t, "native", chart.Metadata["query_type"])
	assert.Equal(t, "Shop", chart.Metadata["database"])
	assert.Equal(t, 2, chart.Metadata["database_id"])
	assert.Equal(t, "Marmot/Finance", chart.Metadata["collection"])
	assert.Equal(t, 5, chart.Metadata["collection_id"])
	assert.Equal(t, 2, chart.Metadata["creator_id"])
	assert.Equal(t, "2026-09-07T21:10:31.876103Z", chart.Metadata["created_at"])
	assert.Equal(t, "2026-09-07T21:10:31.876103Z", chart.Metadata["updated_at"])
	assert.Equal(t, false, chart.Metadata["archived"])
	assert.Contains(t, chart.Metadata["url"], "/question/40-revenue-by-customer")
	assert.Nil(t, chart.Metadata["table_id"], "a native card has no source table id")
}

func TestDiscover_CarriesTheSourceTableIDOfQueryBuilderCards(t *testing.T) {
	card := cardEntry(41, "Orders table", 4, "question", "table", 2, mbqlCardQuery(2, 10))
	card["table_id"] = 10
	result := discover(t, newFakeMetabase().
		withCollections(collectionEntry(4, "Marmot", "/")).
		withDatabases(databaseEntry(2, "Shop", "postgres", nil)).
		withTables(2, tableEntry(10, "public", "orders")).
		withCards(card),
		nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-orders-table")
	require.NotNil(t, chart)
	assert.Equal(t, 10, chart.Metadata["table_id"])
	assert.Equal(t, "query", chart.Metadata["query_type"])
	assert.Nil(t, chart.Query, "query builder cards carry no SQL")
}

func TestDiscover_CarriesDashboardMetadata(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	dash := findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, dash)
	assert.Equal(t, 2, dash.Metadata["id"])
	assert.Equal(t, "Marmot/Finance", dash.Metadata["collection"])
	assert.Equal(t, 5, dash.Metadata["collection_id"])
	assert.Equal(t, 2, dash.Metadata["creator_id"])
	assert.Equal(t, 3, dash.Metadata["card_count"])
	assert.Equal(t, false, dash.Metadata["archived"])
	assert.Contains(t, dash.Metadata["url"], "/dashboard/2-finance-overview")
	require.Len(t, dash.ExternalLinks, 1)
	assert.Equal(t, "Open in Metabase", dash.ExternalLinks[0].Name)
	assert.Equal(t, dash.Metadata["url"], dash.ExternalLinks[0].URL)
}

func TestDiscover_SetsTheSQLOfNativeCards(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	require.NotNil(t, chart.Query)
	assert.Contains(t, *chart.Query, "FROM public.customers c")
	assert.Equal(t, "SQL", *chart.QueryLanguage)
}

func TestDiscover_CarriesDescriptions(t *testing.T) {
	card := cardEntry(40, "Revenue by customer", 5, "question", "bar", 2, nativeCardQuery(2, "SELECT 1"))
	card["description"] = "Total order value per customer"
	dash := dashboardEntry(2, "Finance overview", 5, 40)
	dash["description"] = "Revenue and orders at a glance"
	result := discover(t, newFakeMetabase().
		withCollections(collectionEntry(4, "Marmot", "/"), collectionEntry(5, "Finance", "/4/")).
		withDatabases(databaseEntry(2, "Shop", "postgres", nil)).
		withCards(card).withDashboards(dash),
		nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Equal(t, "Total order value per customer", *chart.Description)

	dashboard := findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, dashboard)
	assert.Equal(t, "Revenue and orders at a glance", *dashboard.Description)
}

func TestDiscover_LeavesEmptyDescriptionsUnset(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Nil(t, chart.Description)
}

func TestDiscover_LinksDashboardsToTheirCards(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	dash := "mrn://dashboard/metabase/marmot-finance-finance-overview"
	assert.True(t, hasEdge(result, dash, "mrn://chart/metabase/marmot-finance-revenue-by-customer", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://data model object/metabase/marmot-finance-customer-orders", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://chart/metabase/marmot-finance-totals-from-model", "CONTAINS"))
	assert.False(t, hasEdge(result, dash, "mrn://chart/metabase/marmot-orders-table", "CONTAINS"), "not on the dashboard")
}

func TestDiscover_LinksNativeCardsToTheTablesTheyRead(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	// The tables are addressed as the PostgreSQL plugin addresses them:
	// bare table name under the PostgreSQL provider.
	chart := "mrn://chart/metabase/marmot-finance-revenue-by-customer"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", chart, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", chart, "FEEDS"))
}

func TestDiscover_LinksTablesToTheDashboardsShowingTheCard(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	dash := "mrn://dashboard/metabase/marmot-finance-finance-overview"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", dash, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", dash, "FEEDS"))
}

func TestDiscover_DoesNotCreateTableAssets(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	for _, a := range result.Assets {
		assert.NotEqual(t, "Table", a.Type, "tables belong to the warehouse's own plugin")
		assert.Equal(t, []string{"Metabase"}, a.Providers)
	}
}

func TestDiscover_LinksQueryBuilderCardsToTheirSourceTable(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://chart/metabase/marmot-orders-table", "FEEDS"))
}

func TestDiscover_WalksTheJoinsOfQueryBuilderCards(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(cardEntry(44, "Orders with regions", 4, "question", "table", 2, mbqlCardQuery(2, 10, 12))),
		nil)

	chart := "mrn://chart/metabase/marmot-orders-with-regions"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", chart, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/regions", chart, "FEEDS"))
}

func TestDiscover_LinksAQuestionToTheModelItBuildsOn(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	assert.True(t, hasEdge(result,
		"mrn://data model object/metabase/marmot-finance-customer-orders",
		"mrn://chart/metabase/marmot-finance-totals-from-model", "FEEDS"))
}

func TestDiscover_LinksANativeCardToTheCardItReferences(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(cardEntry(50, "On top of model", 4, "question", "table", 2,
			nativeCardQuery(2, "SELECT * FROM {{#42-customer-orders}} WHERE total > 10"))),
		nil)

	assert.True(t, hasEdge(result,
		"mrn://data model object/metabase/marmot-finance-customer-orders",
		"mrn://chart/metabase/marmot-on-top-of-model", "FEEDS"))
}

func TestDiscover_ReadsLegacyNativeQueries(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(cardEntry(60, "Legacy native", 4, "question", "table", 2,
			legacyNativeCardQuery(2, "SELECT * FROM public.customers"))),
		nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-legacy-native")
	require.NotNil(t, chart)
	assert.Equal(t, "native", chart.Metadata["query_type"])
	require.NotNil(t, chart.Query)
	assert.Equal(t, "SELECT * FROM public.customers", *chart.Query)
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", *chart.MRN, "FEEDS"))
}

func TestDiscover_ReadsLegacyQueryBuilderQueries(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(
			cardEntry(61, "Legacy joined", 4, "question", "table", 2, legacyMBQLCardQuery(2, 10, 12)),
			cardEntry(62, "Legacy on model", 4, "question", "table", 2, legacyMBQLCardQuery(2, "card__42")),
		),
		nil)

	joined := "mrn://chart/metabase/marmot-legacy-joined"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", joined, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/regions", joined, "FEEDS"))
	assert.True(t, hasEdge(result,
		"mrn://data model object/metabase/marmot-finance-customer-orders",
		"mrn://chart/metabase/marmot-legacy-on-model", "FEEDS"))
}

func TestDiscover_FallsBackToTheCardsTableIDWhenTheQueryNamesNoSource(t *testing.T) {
	card := cardEntry(63, "Bare", 4, "question", "table", 2, map[string]any{"database": 2, "type": "query", "query": map[string]any{}})
	card["table_id"] = 10
	result := discover(t, shopFixture().withCards(card), nil)

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://chart/metabase/marmot-bare", "FEEDS"))
}

func TestDiscover_IgnoresTablesItCannotResolve(t *testing.T) {
	result := discover(t, shopFixture().
		withCards(cardEntry(64, "Unknown table", 4, "question", "table", 2,
			nativeCardQuery(2, "SELECT * FROM public.customers c JOIN staging.invoices i ON i.customer_id = c.id"))),
		nil)

	chart := "mrn://chart/metabase/marmot-unknown-table"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", chart, "FEEDS"))
	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Source, "invoices", "a table Metabase has not synced produces no edge")
	}
}

func TestDiscover_ReadsDatabaseMetadataOncePerDatabase(t *testing.T) {
	server := shopFixture()
	discover(t, server, nil)

	assert.Equal(t, 1, server.requestCount("/database/2/metadata"), "four cards on one database, one metadata call")
}

func TestDiscover_KeepsOnlyContainsEdgesWhenLineageIsOff(t *testing.T) {
	server := shopFixture()
	result := discover(t, server, pluginsdk.RawConfig{"discover_lineage": false})

	// Dashboard membership is structure, not lineage: it stays on so the
	// Contents tree still works, while no table or model edge is read.
	require.NotEmpty(t, result.Lineage)
	for _, edge := range result.Lineage {
		assert.Equal(t, "CONTAINS", edge.Type)
	}
	assert.Equal(t, 0, server.requestCount("/database/2/metadata"))
	assert.NotNil(t, findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer"))
}

func TestDiscover_SkipsArchivedItemsByDefault(t *testing.T) {
	archivedCard := cardEntry(47, "Old revenue", 5, "question", "line", 2, nativeCardQuery(2, "SELECT 1"))
	archivedCard["archived"] = true
	archivedDash := dashboardEntry(4, "Old board", 5, 47)
	archivedDash["archived"] = true
	result := discover(t, shopFixture().withCards(archivedCard).withDashboards(archivedDash), nil)

	assert.Nil(t, findAsset(result, "mrn://chart/metabase/marmot-finance-old-revenue"))
	assert.Nil(t, findAsset(result, "mrn://dashboard/metabase/marmot-finance-old-board"))
}

func TestDiscover_IncludesArchivedItemsWhenAsked(t *testing.T) {
	archivedCard := cardEntry(47, "Old revenue", 5, "question", "line", 2, nativeCardQuery(2, "SELECT 1"))
	archivedCard["archived"] = true
	archivedDash := dashboardEntry(4, "Old board", 5, 47)
	archivedDash["archived"] = true
	result := discover(t, shopFixture().withCards(archivedCard).withDashboards(archivedDash),
		pluginsdk.RawConfig{"include_archived": true})

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-old-revenue")
	require.NotNil(t, chart)
	assert.Equal(t, true, chart.Metadata["archived"])

	dash := findAsset(result, "mrn://dashboard/metabase/marmot-finance-old-board")
	require.NotNil(t, dash, "an archived dashboard keeps the collection it was archived from, not the Trash")
	assert.Equal(t, true, dash.Metadata["archived"])
	assert.True(t, hasEdge(result, *dash.MRN, *chart.MRN, "CONTAINS"))
}

func TestDiscover_OmitsChartsWhenTurnedOff(t *testing.T) {
	result := discover(t, shopFixture(), pluginsdk.RawConfig{"include_charts": false})

	assert.Nil(t, findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer"))
	assert.NotNil(t, findAsset(result, "mrn://data model object/metabase/marmot-finance-customer-orders"))
	assert.NotNil(t, findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview"))

	// The dashboard still learns which tables feed it through the
	// charts that were left out.
	dash := "mrn://dashboard/metabase/marmot-finance-finance-overview"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", dash, "FEEDS"))
	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Target, "mrn://chart/", "no edge may point at a chart that was not emitted")
	}
}

func TestDiscover_OmitsModelsWhenTurnedOff(t *testing.T) {
	result := discover(t, shopFixture(), pluginsdk.RawConfig{"include_models": false})

	assert.Nil(t, findAsset(result, "mrn://data model object/metabase/marmot-finance-customer-orders"))
	assert.NotNil(t, findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer"))
	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Source, "mrn://data model object/")
		assert.NotContains(t, edge.Target, "mrn://data model object/")
	}
}

func TestDiscover_FallsBackToTheCollectionTreeWhenTheListFails(t *testing.T) {
	server := shopFixture()
	server.flatCollectionsFail = true
	// The tree nests children and omits the root, but carries location.
	marmot := collectionEntry(4, "Marmot", "/")
	marmot["children"] = []map[string]any{collectionEntry(5, "Finance", "/4/")}
	server.tree = []map[string]any{marmot}

	result := discover(t, server, nil)

	assert.NotNil(t, findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer"))
	assert.NotNil(t, findAsset(result, "mrn://chart/metabase/marmot-orders-table"))
	assert.Equal(t, 1, server.requestCount("/collection/tree"))
}

func TestDiscover_NamesByBareNameWhenNoCollectionsAreKnown(t *testing.T) {
	server := shopFixture()
	server.flatCollectionsFail = true
	server.tree = nil

	// The tree endpoint answers an empty list rather than an error, so
	// discovery proceeds and items fall back to their bare names.
	result := discover(t, server, nil)

	assert.NotNil(t, findAsset(result, "mrn://chart/metabase/revenue-by-customer"))
}

func TestDiscover_AcceptsABareDatabaseList(t *testing.T) {
	server := shopFixture()
	server.bareDatabaseList = true

	result := discover(t, server, nil)

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Equal(t, "Shop", chart.Metadata["database"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", *chart.MRN, "FEEDS"))
}

func TestDiscover_ReadsOrderedCardsFromOlderServers(t *testing.T) {
	server := shopFixture()
	server.orderedCards = true

	result := discover(t, server, nil)

	dash := findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, dash)
	assert.Equal(t, 3, dash.Metadata["card_count"])
	assert.True(t, hasEdge(result, *dash.MRN, "mrn://chart/metabase/marmot-finance-revenue-by-customer", "CONTAINS"))
}

func TestDiscover_KeepsADashboardWhoseDetailFails(t *testing.T) {
	server := shopFixture()
	server.failDashboardDetail[2] = true

	result := discover(t, server, nil)

	dash := findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, dash, "the list entry is enough to catalogue the dashboard")
	assert.Nil(t, dash.Metadata["card_count"])
	assert.False(t, hasEdge(result, *dash.MRN, "mrn://chart/metabase/marmot-finance-revenue-by-customer", "CONTAINS"))
}

func TestDiscover_IgnoresTextCardsOnDashboards(t *testing.T) {
	dash := dashboardEntry(2, "Finance overview", 5, 40)
	dash["dashcards"] = append(dash["dashcards"].([]map[string]any),
		map[string]any{"id": 200, "card_id": nil, "card": map[string]any{}, "row": 4, "col": 0, "size_x": 6, "size_y": 2})
	server := shopFixture()
	server.dashboards = []map[string]any{dash}

	result := discover(t, server, nil)

	asset := findAsset(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, asset)
	assert.Equal(t, 1, asset.Metadata["card_count"])
}

func TestDiscover_LogsInWithUsernameAndPassword(t *testing.T) {
	server := shopFixture()
	server.apiKey = ""
	server.username = "admin@marmot.test"
	server.password = "Marmot-Test-123"

	result := discover(t, server, pluginsdk.RawConfig{"username": "admin@marmot.test", "password": "Marmot-Test-123"})

	assert.NotNil(t, findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer"))
	assert.Equal(t, 1, server.requestCount("/session"))
}

func TestDiscover_FailsWithAWrongPassword(t *testing.T) {
	server := shopFixture()
	server.apiKey = ""
	server.username = "admin@marmot.test"
	server.password = "Marmot-Test-123"
	httpServer := server.start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"host": httpServer.URL, "username": "admin@marmot.test", "password": "wrong",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "logging in")
	assert.NotContains(t, err.Error(), "wrong", "the password never appears in an error")
}

func TestDiscover_FailsWithAWrongAPIKey(t *testing.T) {
	httpServer := shopFixture().start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": httpServer.URL, "api_key": "nope"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthorized")
	assert.NotContains(t, err.Error(), "nope", "the key never appears in an error")
}

func TestDiscover_FailsWhenTheServerIsUnreachable(t *testing.T) {
	httpServer := shopFixture().start(t)
	httpServer.Close()

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": httpServer.URL, "api_key": "test-key"})

	require.Error(t, err)
}

func TestDiscover_AddressesTablesByTheirEngine(t *testing.T) {
	result := discover(t, newFakeMetabase().
		withDatabases(
			databaseEntry(3, "Warehouse", "snowflake", map[string]any{"db": "ANALYTICS", "warehouse": "WH"}),
			databaseEntry(4, "Sample", "h2", map[string]any{"db": "file:/sample"}),
		).
		withTables(3, tableEntry(30, "PUBLIC", "ORDERS")).
		withTables(4, tableEntry(40, "PUBLIC", "PEOPLE"), tableEntry(41, "", "ACCOUNTS")).
		withCards(
			cardEntry(1, "Snowflake orders", nil, "question", "table", 3, mbqlCardQuery(3, 30)),
			cardEntry(2, "H2 people", nil, "question", "table", 4, mbqlCardQuery(4, 40)),
			cardEntry(3, "H2 accounts", nil, "question", "table", 4, mbqlCardQuery(4, 41)),
		),
		nil)

	assert.True(t, hasEdge(result, "mrn://table/snowflake/analytics.public.orders", "mrn://chart/metabase/snowflake-orders", "FEEDS"),
		"Snowflake tables are database.schema.table")
	assert.True(t, hasEdge(result, "mrn://table/h2/public.people", "mrn://chart/metabase/h2-people", "FEEDS"),
		"an engine without a plugin keeps its name as provider and is schema-qualified")
	assert.True(t, hasEdge(result, "mrn://table/h2/accounts", "mrn://chart/metabase/h2-accounts", "FEEDS"),
		"no schema, bare name")
}

func TestDiscover_ResolvesBareTableNamesAgainstTheDefaultSchema(t *testing.T) {
	result := discover(t, newFakeMetabase().
		withDatabases(databaseEntry(2, "Shop", "postgres", nil)).
		withTables(2, tableEntry(10, "public", "orders"), tableEntry(11, "archive", "orders"), tableEntry(12, "sales", "regions")).
		withCards(cardEntry(1, "Bare names", nil, "question", "table", 2,
			nativeCardQuery(2, "SELECT * FROM orders o JOIN regions r ON r.id = o.region_id"))),
		nil)

	chart := "mrn://chart/metabase/bare-names"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", chart, "FEEDS"), "two schemas hold orders; public wins")
	assert.True(t, hasEdge(result, "mrn://table/postgresql/regions", chart, "FEEDS"), "only one schema holds regions")
}

func TestDiscover_InterpolatesTags(t *testing.T) {
	result := discover(t, shopFixture(), pluginsdk.RawConfig{"tags": []any{"metabase", "db:${database}"}})

	chart := findAsset(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Equal(t, []string{"metabase", "db:Shop"}, chart.Tags)
}

func TestDiscover_EmitsEachEdgeOnce(t *testing.T) {
	// Two dashboards show the same card and a card reads a table twice.
	result := discover(t, shopFixture().
		withDashboards(dashboardEntry(3, "Second board", 5, 40)),
		nil)

	seen := make(map[pluginsdk.LineageEdge]int)
	for _, edge := range result.Lineage {
		seen[edge]++
	}
	for edge, n := range seen {
		assert.Equalf(t, 1, n, "edge %v emitted %d times", edge, n)
	}
}

func TestDiscover_EveryAssetHasSourcesAndAnEmptySchema(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.Len(t, a.Sources, 1)
		assert.Equal(t, "Metabase", a.Sources[0].Name)
		assert.Equal(t, 1, a.Sources[0].Priority)
		assert.NotNil(t, a.Schema)
	}
}
