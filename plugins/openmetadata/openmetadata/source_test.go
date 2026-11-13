package openmetadata

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_RequiresHost(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"jwt_token": "t"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RequiresToken(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://om.example.com"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jwt_token")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "om.example.com", "jwt_token": "t"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://om.example.com", "jwt_token": "t"})
	require.NoError(t, err)

	assert.True(t, source.config.IncludeTables)
	assert.True(t, source.config.IncludeLineage)
	assert.True(t, source.config.IncludeColumns)
	assert.False(t, source.config.IncludeDeleted)
	assert.Equal(t, 250, source.config.PageSize)
	assert.Equal(t, 8, source.config.Concurrency)
}

func TestValidate_KeepsExplicitValues(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://om.example.com", "jwt_token": "t",
		"include_tables": false, "page_size": 10,
	})
	require.NoError(t, err)

	assert.False(t, source.config.IncludeTables)
	assert.Equal(t, 10, source.config.PageSize)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "openmetadata", meta.ID)
	assert.Equal(t, "OpenMetadata", meta.Name)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestDiscover_CataloguesATableAsItsOwnTechnology(t *testing.T) {
	result := discover(t, newFakeOM().
		with("databases", entity("postgres_prod", "Postgres", "postgres_prod.shop")).
		with("tables", tableEntity("postgres_prod", "Postgres", "postgres_prod.shop.public.orders", "Regular")),
		nil)

	orders := findAsset(result, "Table", "orders")
	require.NotNil(t, orders)
	assert.Equal(t, []string{"PostgreSQL"}, orders.Providers,
		"a Postgres table in OpenMetadata is a PostgreSQL asset in Marmot")
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders", *orders.MRN)
}

func TestDiscover_LinksATableToItsDatabase(t *testing.T) {
	result := discover(t, newFakeOM().
		with("databases", entity("postgres_prod", "Postgres", "postgres_prod.shop")).
		with("tables", tableEntity("postgres_prod", "Postgres", "postgres_prod.shop.public.orders", "Regular")),
		nil)

	assert.True(t, hasEdge(result, "mrn://asset/database/postgresql/shop", "mrn://asset/table/postgresql/shop.public.orders", "CONTAINS"))
}

func TestDiscover_SeparatesViewsFromTables(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular"),
			tableEntity("pg", "Postgres", "pg.shop.public.recent_orders", "View")),
		nil)

	assert.NotNil(t, findAsset(result, "Table", "orders"))
	assert.NotNil(t, findAsset(result, "View", "recent_orders"))
}

func TestDiscover_ServiceMapKeepsServicesApart(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular"),
			tableEntity("pg_staging", "Postgres", "pg_staging.shop.public.orders", "Regular")),
		pluginsdk.RawConfig{"service_map": map[string]any{"pg_prod": "prod", "pg_staging": "staging"}})

	require.Len(t, result.Assets, 2)
	assert.NotNil(t, findAsset(result, "Table", "orders"))
	assert.ElementsMatch(t,
		[]string{"mrn://asset/table/postgresql@prod/shop.public.orders", "mrn://asset/table/postgresql@staging/shop.public.orders"},
		[]string{*result.Assets[0].MRN, *result.Assets[1].MRN})
}

func TestDiscover_AnUnmappedServiceUsesItsConnectionAddressAsAHostHint(t *testing.T) {
	result := discover(t, newFakeOM().
		with("services/databaseServices", map[string]interface{}{
			"name": "pg_prod", "serviceType": "Postgres",
			"connection": map[string]interface{}{"config": map[string]interface{}{"hostPort": "PG.internal:6432"}},
		}).
		with("tables", tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular")),
		nil)

	orders := findAsset(result, "Table", "orders")
	require.NotNil(t, orders)
	assert.Equal(t, "mrn://asset/table/postgresql@host=pg.internal:6432/shop.public.orders", *orders.MRN)
}

func TestDiscover_AServiceMapEntryBeatsTheConnectionAddress(t *testing.T) {
	result := discover(t, newFakeOM().
		with("services/databaseServices", map[string]interface{}{
			"name": "pg_prod", "serviceType": "Postgres",
			"connection": map[string]interface{}{"config": map[string]interface{}{"hostPort": "pg.internal"}},
		}).
		with("tables", tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular")),
		pluginsdk.RawConfig{"service_map": map[string]any{"pg_prod": "prod-eu"}})

	orders := findAsset(result, "Table", "orders")
	require.NotNil(t, orders)
	assert.Equal(t, "mrn://asset/table/postgresql@prod-eu/shop.public.orders", *orders.MRN)
}

func TestDiscover_SameTableInTwoSchemasGetsDifferentMRNs(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular"),
			tableEntity("pg", "Postgres", "pg.shop.archive.orders", "Regular")),
		nil)

	require.Len(t, result.Assets, 2)
	assert.NotEqual(t, *result.Assets[0].MRN, *result.Assets[1].MRN)
}

func TestDiscover_UnmappedServicesOfOneTechnologyMergeAndReportIt(t *testing.T) {
	source := &Source{}
	config := pluginsdk.RawConfig{"host": "https://om.example.com", "jwt_token": "t"}
	_, err := source.Validate(config)
	require.NoError(t, err)

	c := newCollector(source.config)
	p := projectionFor("Postgres")
	base := entityBase{ID: "a", FullyQualifiedName: "pg_prod.shop.public.orders", ServiceType: "Postgres"}
	other := entityBase{ID: "b", FullyQualifiedName: "pg_staging.shop.public.orders", ServiceType: "Postgres"}

	c.add("a", c.newAsset(base, "table", "Table", p, c.tableIdent(p, "shop", "public", "orders"), nil))
	c.add("b", c.newAsset(other, "table", "Table", p, c.tableIdent(p, "shop", "public", "orders"), nil))

	require.Len(t, c.collisions, 1)
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders", c.collisions[0].MRN)
	assert.Equal(t, "pg_prod.shop.public.orders", openMetadataFQN(c.collisions[0].First))
	assert.Equal(t, "pg_staging.shop.public.orders", openMetadataFQN(c.collisions[0].Second))
}

func TestMeta_Federates(t *testing.T) {
	assert.True(t, Meta().Federates)
}

func TestDiscover_SkipsExcludedServices(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular"),
			tableEntity("pg_staging", "Postgres", "pg_staging.shop.public.customers", "Regular")),
		pluginsdk.RawConfig{"exclude_services": []string{"pg_staging"}})

	require.Len(t, result.Assets, 1)
	assert.NotNil(t, findAsset(result, "Table", "orders"))
}

func TestDiscover_SkipsExcludedServiceTypes(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		with("topics", entity("kafka", "Kafka", "kafka.payments")),
		pluginsdk.RawConfig{"exclude_service_types": []string{"kafka"}})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Table", result.Assets[0].Type)
}

func TestDiscover_KeepsOnlyTheListedServiceTypes(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		with("topics", entity("kafka", "Kafka", "kafka.payments")),
		pluginsdk.RawConfig{"service_types": []string{"Kafka"}})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Topic", result.Assets[0].Type)
}

func TestDiscover_SkipsSoftDeletedEntities(t *testing.T) {
	deleted := tableEntity("pg", "Postgres", "pg.shop.public.old_orders", "Regular")
	deleted["deleted"] = true

	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular"), deleted),
		nil)

	require.Len(t, result.Assets, 1)
	assert.NotNil(t, findAsset(result, "Table", "orders"))
}

func TestDiscover_HonoursIncludeToggles(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		with("topics", entity("kafka", "Kafka", "kafka.payments")),
		pluginsdk.RawConfig{"include_tables": false})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Topic", result.Assets[0].Type)
}

func TestDiscover_SurvivesEntityKindsTheServerDoesNotHave(t *testing.T) {
	// An OpenMetadata older than the entity kinds this plugin reads
	// answers 404 for them; the rest of the import must still succeed.
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		without("apiCollections", "apiEndpoints", "searchIndexes", "storedProcedures", "containers", "mlmodels"),
		nil)

	require.Len(t, result.Assets, 1)
	assert.NotNil(t, findAsset(result, "Table", "orders"))
}

func TestDiscover_FailsWhenTheServerIsNotOpenMetadata(t *testing.T) {
	// A wrong host must be an error, not an empty import.
	f := newFakeOM().without("tables", "databases", "databaseSchemas")
	server := f.start(t)

	source := &Source{}
	_, err := source.Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL, "jwt_token": "t"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "databases")
}

// TestDiscover_MRNsAreCanonical pins that every MRN is in its kinded form and is its own canonical spelling, so the server stores it as written and every link built from it resolves.
func TestDiscover_MRNsAreCanonical(t *testing.T) {
	result := discover(t, everyEntityKind(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.NotEmpty(t, asset.Providers)

		parsed, err := mrn.ParseMRN(*asset.MRN)
		require.NoError(t, err, *asset.MRN)
		assert.False(t, parsed.Legacy, *asset.MRN)
		assert.Equal(t, *asset.MRN, parsed.String())
	}
}

// A table is named by its own name and its MRN carries the whole path, so a Snowflake table reads "orders" in the catalog and lands on database.schema.table.
func TestDiscover_NameIsTheObjectsOwnNameAndTheMRNCarriesThePath(t *testing.T) {
	result := discover(t, newFakeOM().with("tables",
		tableEntity("sf", "Snowflake", "sf.sales.public.orders", "Regular")), nil)

	table := findAsset(result, "Table", "orders")
	require.NotNil(t, table)

	assert.Equal(t, "mrn://asset/table/snowflake/sales.public.orders", *table.MRN)
	assert.NotContains(t, table.PreviousMRNs, *table.MRN)
	assert.Equal(t, []string{"mrn://asset/table/snowflake/sf.sales.public.orders"}, table.PreviousMRNs, "only the qualified naming mode's MRN differs")
}

// Earlier releases named a Postgres table by its bare name, so the old MRN is listed for the catalog to upgrade.
func TestDiscover_ListsTheOldMRNOfAnAssetWhoseMRNChanged(t *testing.T) {
	result := discover(t, newFakeOM().with("tables",
		tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")), nil)

	table := findAsset(result, "Table", "orders")
	require.NotNil(t, table)

	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders", *table.MRN)
	assert.Contains(t, table.PreviousMRNs, "mrn://asset/table/postgresql/orders")
	assert.Contains(t, table.PreviousMRNs, "mrn://asset/table/postgresql/pg.shop.public.orders", "the qualified naming mode's MRN")
}

// TestDiscover_LineagePointsAtRealAssets guards the other half of the
// same invariant: every edge must reference an asset the run emitted.
func TestDiscover_LineagePointsAtRealAssets(t *testing.T) {
	result := discover(t, everyEntityKind(), nil)
	require.NotEmpty(t, result.Lineage)

	known := make(map[string]bool, len(result.Assets))
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, edge := range result.Lineage {
		assert.True(t, known[edge.Source], "edge source %q is not an emitted asset", edge.Source)
		assert.True(t, known[edge.Target], "edge target %q is not an emitted asset", edge.Target)
	}
}

// everyEntityKind is a fake OpenMetadata holding one of every entity
// this plugin reads.
func everyEntityKind() *fakeOM {
	dashboard := entity("looker", "Looker", "looker.sales_overview")
	dashboard["charts"] = []map[string]interface{}{{"id": "id-looker.revenue"}}
	dashboard["dataModels"] = []map[string]interface{}{{"id": "id-looker.sales_model"}}

	pipeline := entity("airflow", "Airflow", "airflow.orders_etl")
	pipeline["tasks"] = []map[string]interface{}{
		{"name": "extract", "downstreamTasks": []string{"load"}},
		{"name": "load"},
	}

	child := entity("s3", "S3", "s3.raw.events")
	child["parent"] = map[string]interface{}{"fullyQualifiedName": "s3.raw"}

	return newFakeOM().
		with("databases", entity("pg", "Postgres", "pg.shop")).
		with("databaseSchemas", entity("bq", "BigQuery", "bq.project.analytics")).
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular"),
			tableEntity("bq", "BigQuery", "bq.project.analytics.events", "Regular")).
		with("storedProcedures", entity("pg", "Postgres", "pg.shop.public.refresh_orders")).
		with("topics", entity("kafka", "Kafka", "kafka.payments")).
		with("containers", entity("s3", "S3", "s3.raw"), child).
		with("dashboards", dashboard).
		with("charts", entity("looker", "Looker", "looker.revenue")).
		with("dashboard/datamodels", entity("looker", "Looker", "looker.sales_model")).
		with("pipelines", pipeline).
		with("mlmodels", entity("mlflow", "Mlflow", "mlflow.churn")).
		with("searchIndexes", entity("es", "ElasticSearch", "es.orders_index")).
		with("apiCollections", entity("rest", "Rest", "rest.pets")).
		with("apiEndpoints", entity("rest", "Rest", "rest.pets.addPet"))
}

func TestMeta_AdvertisesEverythingThePluginProduces(t *testing.T) {
	assert.Equal(t, []string{"Assets", "Lineage", "Run History", "Glossary"}, Meta().Features)
}

func TestValidate_RejectsAnExplicitZeroRatherThanRewritingIt(t *testing.T) {
	// These fields all carry a default, so an absent value is filled in.
	// A value someone actually typed is theirs: rewriting a 0 behind their
	// back hides the mistake, and 0 concurrency used to deadlock the run.
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://om.example.com", "jwt_token": "t", "concurrency": 0,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "concurrency must be at least 1")
}

func TestDiscover_NeverBuildsAnUnbufferedSemaphore(t *testing.T) {
	// A zero here made make(chan struct{}, 0), whose only receiver runs
	// after a successful send, so the run hung forever with no timeout
	// around it. Validation now refuses the value instead.
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://om.example.com", "jwt_token": "t", "concurrency": 0,
	})
	require.Error(t, err)

	_, err = source.Validate(pluginsdk.RawConfig{"host": "https://om.example.com", "jwt_token": "t"})
	require.NoError(t, err)
	assert.Equal(t, 8, source.config.Concurrency, "an absent value still defaults")
}
