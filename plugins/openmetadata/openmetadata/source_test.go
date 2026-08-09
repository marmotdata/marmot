package openmetadata

import (
	"strings"
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

func TestValidate_RejectsAnUnknownNamingMode(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "https://om.example.com", "jwt_token": "t", "naming": "flat",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "naming")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://om.example.com", "jwt_token": "t"})
	require.NoError(t, err)

	assert.Equal(t, namingNative, source.config.Naming)
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
		"naming": namingQualified, "include_tables": false, "page_size": 10,
	})
	require.NoError(t, err)

	assert.Equal(t, namingQualified, source.config.Naming)
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

	orders := findAsset(result, "Table", "shop.public.orders")
	require.NotNil(t, orders)
	assert.Equal(t, []string{"PostgreSQL"}, orders.Providers,
		"a Postgres table in OpenMetadata is a PostgreSQL asset in Marmot")
	assert.Equal(t, "mrn://table/postgresql/shop.public.orders", *orders.MRN)
}

func TestDiscover_LinksATableToItsDatabase(t *testing.T) {
	result := discover(t, newFakeOM().
		with("databases", entity("postgres_prod", "Postgres", "postgres_prod.shop")).
		with("tables", tableEntity("postgres_prod", "Postgres", "postgres_prod.shop.public.orders", "Regular")),
		nil)

	assert.True(t, hasEdge(result, "mrn://database/postgresql/shop", "mrn://table/postgresql/shop.public.orders", "CONTAINS"))
}

func TestDiscover_SeparatesViewsFromTables(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular"),
			tableEntity("pg", "Postgres", "pg.shop.public.recent_orders", "View")),
		nil)

	assert.NotNil(t, findAsset(result, "Table", "shop.public.orders"))
	assert.NotNil(t, findAsset(result, "View", "shop.public.recent_orders"))
}

func TestDiscover_QualifiedNamingKeepsServicesApart(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular"),
			tableEntity("pg_staging", "Postgres", "pg_staging.shop.public.orders", "Regular")),
		pluginsdk.RawConfig{"naming": namingQualified})

	require.Len(t, result.Assets, 2)
	assert.NotNil(t, findAsset(result, "Table", "pg_prod.shop.public.orders"))
	assert.NotNil(t, findAsset(result, "Table", "pg_staging.shop.public.orders"))

	// Distinct MRNs, which is the whole point of the mode.
	assert.NotEqual(t, *result.Assets[0].MRN, *result.Assets[1].MRN)
}

func TestDiscover_NativeNamingMergesAndReportsIt(t *testing.T) {
	// Two Postgres services holding the same table name resolve to one
	// asset, because that is the name the PostgreSQL plugin would use.
	source := &Source{}
	config := pluginsdk.RawConfig{"host": "https://om.example.com", "jwt_token": "t"}
	_, err := source.Validate(config)
	require.NoError(t, err)

	c := newCollector(source.config)
	p := projectionFor("Postgres")
	base := entityBase{ID: "a", FullyQualifiedName: "pg_prod.shop.public.orders", ServiceType: "Postgres"}
	other := entityBase{ID: "b", FullyQualifiedName: "pg_staging.shop.public.orders", ServiceType: "Postgres"}

	c.add("a", c.newAsset(base, "table", "Table", p, "orders", nil))
	c.add("b", c.newAsset(other, "table", "Table", p, "orders", nil))

	require.Len(t, c.collisions, 1)
	assert.Equal(t, "mrn://table/postgresql/orders", c.collisions[0].MRN)
	assert.Equal(t, "pg_prod.shop.public.orders", openMetadataFQN(c.collisions[0].First))
	assert.Equal(t, "pg_staging.shop.public.orders", openMetadataFQN(c.collisions[0].Second))
}

func TestDiscover_SkipsExcludedServices(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables",
			tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular"),
			tableEntity("pg_staging", "Postgres", "pg_staging.shop.public.customers", "Regular")),
		pluginsdk.RawConfig{"exclude_services": []string{"pg_staging"}})

	require.Len(t, result.Assets, 1)
	assert.NotNil(t, findAsset(result, "Table", "shop.public.orders"))
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
	assert.NotNil(t, findAsset(result, "Table", "shop.public.orders"))
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
	assert.NotNil(t, findAsset(result, "Table", "shop.public.orders"))
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

// TestDiscover_MRNsSurviveTheServer is the invariant that makes an asset
// reachable. Marmot keeps the MRN a plugin declares, and the UI builds
// every link by splitting that MRN back into type, service and name,
// which /assets/lookup then feeds to mrn.New again. So the MRN has to
// survive that round trip unchanged, or the asset exists but its page
// 404s.
//
// Note this is NOT a constraint that Name reproduce the MRN. Name is the
// object's own name, for reading; the MRN carries the qualified path.
func TestDiscover_MRNsSurviveTheServer(t *testing.T) {
	result := discover(t, everyEntityKind(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.NotNil(t, asset.Name)
		require.NotEmpty(t, asset.Providers)

		parts := strings.SplitN(strings.TrimPrefix(*asset.MRN, "mrn://"), "/", 3)
		require.Len(t, parts, 3, "malformed MRN %q", *asset.MRN)

		assert.Equal(t, *asset.MRN, mrn.New(parts[0], parts[1], parts[2]),
			"asset %q is not reachable: its MRN changes when the server rebuilds it from its own parts", *asset.MRN)
	}
}

// TestDiscover_NamesAreTheObjectsOwnName pins the other half: what people
// read is the plain name, not the path that identifies it.
func TestDiscover_NamesAreTheObjectsOwnName(t *testing.T) {
	result := discover(t, newFakeOM().with("tables",
		tableEntity("sf", "Snowflake", "sf.sales.public.orders", "Regular")), nil)

	table := findAsset(result, "Table", "sales.public.orders")
	require.NotNil(t, table)

	assert.Equal(t, "mrn://table/snowflake/sales.public.orders", *table.MRN,
		"identity keeps the whole path, so two databases stay apart")
	assert.Equal(t, "orders", *table.Name,
		"but the catalog reads orders, the way ClickHouse and Iceberg already do")
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

func TestValidate_ReplacesExplicitZerosWithDefaults(t *testing.T) {
	// A zero concurrency would build an unbuffered semaphore and hang the
	// lineage pass. ApplyDefaults cannot catch it: the key is present.
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://om.example.com", "jwt_token": "t",
		"concurrency": 0, "page_size": 0, "timeout_seconds": 0,
		"source_priority": 0, "run_history_days": 0, "run_history_limit": 0,
	})
	require.NoError(t, err)

	assert.Equal(t, 8, source.config.Concurrency)
	assert.Equal(t, 250, source.config.PageSize)
	assert.Equal(t, 60, source.config.TimeoutSeconds)
	assert.Equal(t, 2, source.config.SourcePriority)
	assert.Equal(t, 7, source.config.RunHistoryDays)
	assert.Equal(t, 50, source.config.RunHistoryLimit)
}

func TestDiscover_CompletesWithAZeroConcurrency(t *testing.T) {
	result := discover(t, lineageFixture(), pluginsdk.RawConfig{"concurrency": 0})

	assert.NotEmpty(t, result.Lineage)
}
