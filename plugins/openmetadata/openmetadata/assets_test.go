package openmetadata

import (
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The curated layer is the reason to import from OpenMetadata at all:
// descriptions, tags, glossary terms and ownership that nobody would get
// from reading the database itself.

func TestDiscover_CarriesTheDescription(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["description"] = "One row per customer order"

	result := discover(t, newFakeOM().with("tables", orders), nil)

	asset := findAsset(result, "Table", "orders")
	require.NotNil(t, asset)
	require.NotNil(t, asset.Description)
	assert.Equal(t, "One row per customer order", *asset.Description)
}

func TestDiscover_CarriesClassificationTags(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["tags"] = []map[string]interface{}{
		{"tagFQN": "PII.Sensitive", "source": "Classification", "state": "Confirmed"},
	}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	assert.Equal(t, []string{"PII.Sensitive"}, findAsset(result, "Table", "orders").Tags)
}

func TestDiscover_CarriesGlossaryTermsAsTags(t *testing.T) {
	// Marmot ingestion runs cannot create glossary terms, so the terms
	// curated onto an asset travel with it as tags instead.
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["tags"] = []map[string]interface{}{
		{"tagFQN": "Business.Customer", "source": "Glossary", "state": "Confirmed"},
	}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	asset := findAsset(result, "Table", "orders")
	assert.Equal(t, []string{"Business.Customer"}, asset.Tags)
	assert.Equal(t, []string{"Business.Customer"}, asset.Metadata["glossary_terms"])
}

func TestDiscover_SkipsSuggestedTags(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["tags"] = []map[string]interface{}{
		{"tagFQN": "PII.Sensitive", "source": "Classification", "state": "Suggested"},
	}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	assert.Empty(t, findAsset(result, "Table", "orders").Tags,
		"a suggestion nobody accepted is not a fact about the asset")
}

func TestDiscover_CanLeaveOpenMetadataTagsBehind(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["tags"] = []map[string]interface{}{
		{"tagFQN": "PII.Sensitive", "source": "Classification", "state": "Confirmed"},
		{"tagFQN": "Business.Customer", "source": "Glossary", "state": "Confirmed"},
	}

	result := discover(t, newFakeOM().with("tables", orders),
		pluginsdk.RawConfig{"tags_from_openmetadata": false, "glossary_terms_as_tags": false})

	assert.Empty(t, findAsset(result, "Table", "orders").Tags)
}

func TestDiscover_AppliesConfiguredTags(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")),
		pluginsdk.RawConfig{"tags": []string{"imported"}})

	assert.Contains(t, findAsset(result, "Table", "orders").Tags, "imported")
}

func TestDiscover_RecordsOwnersDomainsAndDataProducts(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["owners"] = []map[string]interface{}{{"name": "data-eng", "displayName": "Data Engineering"}}
	orders["domains"] = []map[string]interface{}{{"name": "Commerce"}}
	orders["dataProducts"] = []map[string]interface{}{{"name": "Orders API"}}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	asset := findAsset(result, "Table", "orders")
	assert.Equal(t, []string{"Data Engineering"}, asset.Metadata["owners"])
	assert.Equal(t, []string{"Commerce"}, asset.Metadata["domains"])
	assert.Equal(t, []string{"Orders API"}, asset.Metadata["data_products"])
}

func TestDiscover_RecordsWhereTheAssetCameFrom(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg_prod", "Postgres", "pg_prod.shop.public.orders", "Regular")),
		nil)

	om, ok := findAsset(result, "Table", "orders").Metadata["openmetadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "pg_prod.shop.public.orders", om["fqn"])
	assert.Equal(t, "pg_prod", om["service"])
	assert.Equal(t, "Postgres", om["service_type"])
}

func TestDiscover_RecordsOpenMetadataAsTheSource(t *testing.T) {
	// The source stays distinct from the provider so an asset that both
	// this plugin and the native plugin found shows both contributions.
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")),
		nil)

	sources := findAsset(result, "Table", "orders").Sources
	require.Len(t, sources, 1)
	assert.Equal(t, "OpenMetadata", sources[0].Name)
	assert.Equal(t, 2, sources[0].Priority)
}

func TestDiscover_LinksBackToOpenMetadata(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")),
		nil)

	links := findAsset(result, "Table", "orders").ExternalLinks
	require.NotEmpty(t, links)
	assert.Equal(t, "OpenMetadata", links[0].Name)
	assert.Contains(t, links[0].URL, "/table/pg.shop.public.orders")
}

func TestDiscover_LinksToTheUnderlyingSystem(t *testing.T) {
	dashboard := entity("looker", "Looker", "looker.sales")
	dashboard["sourceUrl"] = "https://looker.company.com/dashboards/7"

	result := discover(t, newFakeOM().with("dashboards", dashboard), nil)

	links := findAsset(result, "Dashboard", "sales").ExternalLinks
	require.Len(t, links, 2)
	assert.Equal(t, "Open in Looker", links[1].Name)
	assert.Equal(t, "https://looker.company.com/dashboards/7", links[1].URL)
}

func TestDiscover_CanLeaveOutTheOpenMetadataLink(t *testing.T) {
	result := discover(t, newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")),
		pluginsdk.RawConfig{"link_to_openmetadata": false})

	assert.Empty(t, findAsset(result, "Table", "orders").ExternalLinks)
}

func TestDiscover_RecordsColumns(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["columns"] = []map[string]interface{}{
		{"name": "id", "dataType": "INT", "constraint": "PRIMARY_KEY", "ordinalPosition": 1},
		{"name": "email", "dataType": "VARCHAR", "description": "Customer email", "ordinalPosition": 2},
	}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	columns := decodeColumns(t, findAsset(result, "Table", "orders"))
	require.Len(t, columns, 2)
	assert.Equal(t, "id", columns[0]["column_name"])
	assert.Equal(t, true, columns[0]["is_primary_key"])
	assert.Equal(t, "Customer email", columns[1]["comment"])
}

func TestDiscover_FlattensNestedColumns(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["columns"] = []map[string]interface{}{
		{"name": "customer", "dataType": "STRUCT", "children": []map[string]interface{}{
			{"name": "email", "dataType": "VARCHAR"},
		}},
	}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	columns := decodeColumns(t, findAsset(result, "Table", "orders"))
	children, ok := columns[0]["children"].([]interface{})
	require.True(t, ok)
	require.Len(t, children, 1)
	assert.Equal(t, "email", children[0].(map[string]interface{})["column_name"])
}

func TestDiscover_CanLeaveOutColumns(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["columns"] = []map[string]interface{}{{"name": "id", "dataType": "INT"}}

	result := discover(t, newFakeOM().with("tables", orders),
		pluginsdk.RawConfig{"include_columns": false})

	assert.Empty(t, findAsset(result, "Table", "orders").Schema["columns"])
}

func TestDiscover_RecordsTableProfileMetrics(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["profile"] = map[string]interface{}{"rowCount": 4200, "sizeInByte": 8192}

	result := discover(t, newFakeOM().with("tables", orders), nil)

	metadata := findAsset(result, "Table", "orders").Metadata
	assert.Equal(t, int64(4200), metadata["row_count"])
	assert.Equal(t, int64(8192), metadata["size"])
}

func TestDiscover_CarriesStoredProcedureCodeAsAQuery(t *testing.T) {
	procedure := entity("pg", "Postgres", "pg.shop.public.refresh_orders")
	procedure["storedProcedureCode"] = map[string]interface{}{"language": "SQL", "code": "BEGIN END"}

	result := discover(t, newFakeOM().with("storedProcedures", procedure), nil)

	asset := findAsset(result, "Function", "refresh_orders")
	require.NotNil(t, asset)
	require.NotNil(t, asset.Query)
	assert.Equal(t, "BEGIN END", *asset.Query)
	assert.Equal(t, "SQL", *asset.QueryLanguage)
}

func TestDiscover_CataloguesTopLevelContainersAsBuckets(t *testing.T) {
	child := entity("s3", "S3", "s3.raw.events")
	child["parent"] = map[string]interface{}{"fullyQualifiedName": "s3.raw"}

	result := discover(t, newFakeOM().with("containers", entity("s3", "S3", "s3.raw"), child), nil)

	assert.NotNil(t, findAsset(result, "Bucket", "raw"))
	assert.NotNil(t, findAsset(result, "Container", "raw/events"))
	assert.True(t, hasEdge(result, "mrn://bucket/s3/raw", "mrn://container/s3/raw-events", "CONTAINS"))
}

func TestDiscover_LinksDashboardsToTheirCharts(t *testing.T) {
	dashboard := entity("superset", "Superset", "superset.sales")
	dashboard["charts"] = []map[string]interface{}{{"id": "id-superset.revenue"}}

	result := discover(t, newFakeOM().
		with("dashboards", dashboard).
		with("charts", entity("superset", "Superset", "superset.revenue")),
		nil)

	assert.True(t, hasEdge(result, "mrn://dashboard/superset/sales", "mrn://chart/superset/revenue", "CONTAINS"))
}

func TestDiscover_LinksPipelineTasksInOrder(t *testing.T) {
	pipeline := entity("airflow", "Airflow", "airflow.orders_etl")
	pipeline["tasks"] = []map[string]interface{}{
		{"name": "extract", "downstreamTasks": []string{"load"}},
		{"name": "load"},
	}

	result := discover(t, newFakeOM().with("pipelines", pipeline), nil)

	// Task naming matches the Airflow plugin: <dag>.<task>.
	assert.NotNil(t, findAsset(result, "Task", "orders_etl.extract"))
	assert.True(t, hasEdge(result,
		"mrn://pipeline/airflow/orders_etl", "mrn://task/airflow/orders_etl.extract", "CONTAINS"))
	assert.True(t, hasEdge(result,
		"mrn://task/airflow/orders_etl.extract", "mrn://task/airflow/orders_etl.load", "DEPENDS_ON"))
}

func TestDiscover_CanLeaveOutPipelineTasks(t *testing.T) {
	pipeline := entity("airflow", "Airflow", "airflow.orders_etl")
	pipeline["tasks"] = []map[string]interface{}{{"name": "extract"}}

	result := discover(t, newFakeOM().with("pipelines", pipeline),
		pluginsdk.RawConfig{"include_tasks": false})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Pipeline", result.Assets[0].Type)
}

func decodeColumns(t *testing.T, asset *pluginsdk.Asset) []map[string]interface{} {
	t.Helper()
	require.NotNil(t, asset)

	encoded, ok := asset.Schema["columns"]
	require.True(t, ok, "asset has no columns")

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(encoded), &columns))
	return columns
}

func TestDiscover_DecodesATableThatHasARetentionPeriod(t *testing.T) {
	// OpenMetadata returns retentionPeriod as an ISO-8601 duration
	// string. Typing it as anything else fails the whole page decode and
	// takes every table on it down.
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["retentionPeriod"] = "P23DT23H"

	result := discover(t, newFakeOM().with("tables", orders), nil)

	assert.NotNil(t, findAsset(result, "Table", "orders"))
}

func TestDiscover_LinksATableWhoseDatabaseNameContainsADot(t *testing.T) {
	// OpenMetadata quotes a name containing a dot, so the container
	// lookup has to compare parsed parts rather than raw names.
	db := entity("pg", "Postgres", `pg."my.shop"`)
	db["name"] = "my.shop"
	orders := tableEntity("pg", "Postgres", `pg."my.shop".public.orders`, "Regular")

	result := discover(t, newFakeOM().with("databases", db).with("tables", orders), nil)

	assert.True(t, hasEdge(result,
		"mrn://database/postgresql/my.shop", "mrn://table/postgresql/orders", "CONTAINS"))
}

func TestDiscover_LinksToTheUIEvenWhenTheHostIsTheAPIRoot(t *testing.T) {
	// apiBaseURL accepts a host ending in /api, but the UI lives above it.
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://om.example.com/api", "jwt_token": "t",
	})
	require.NoError(t, err)

	c := newCollector(source.config)
	url := c.entityURL(entityBase{FullyQualifiedName: "pg.shop.public.orders"}, "table")

	assert.Equal(t, "https://om.example.com/table/pg.shop.public.orders", url)
}

func TestDiscover_NamesAnIcebergNamespaceTheWayTheIcebergPluginDoes(t *testing.T) {
	// OpenMetadata puts a placeholder database above an Iceberg
	// namespace; the Iceberg plugin names a namespace by its path alone.
	result := discover(t, newFakeOM().
		with("databaseSchemas", entity("iceberg", "Iceberg", "iceberg.default.analytics")),
		nil)

	namespace := findAsset(result, "Namespace", "analytics")
	require.NotNil(t, namespace)
	assert.Equal(t, "mrn://namespace/iceberg/analytics", *namespace.MRN)
}
