package unitycatalog

import (
	"net/http"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_CreatesACatalogAsset(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	catalog := findAsset(result, "Catalog", "unity")
	require.NotNil(t, catalog)
	assert.Equal(t, "unity", *catalog.Name)
	assert.Equal(t, []string{"Unity Catalog"}, catalog.Providers)
	require.NotNil(t, catalog.Description)
	assert.Equal(t, "reference catalog using oss UC", *catalog.Description)
	assert.Equal(t, "reference catalog using oss UC", catalog.Metadata["comment"])
	assert.Equal(t, 1, catalog.Metadata["schema_count"])
	assert.Equal(t, 2, catalog.Metadata["table_count"])
	assert.Equal(t, "f029b870-9468-4f10-badc-630b41e5690d", catalog.Metadata["catalog_id"])
	assert.Equal(t, map[string]string{"description": "provides categorized schemas containing collections of tables"}, catalog.Metadata["properties"])
}

func TestDiscover_RendersTimestampsAsRFC3339(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	catalog := findAsset(result, "Catalog", "unity")
	require.NotNil(t, catalog)
	assert.Equal(t, "2024-07-17T18:40:05Z", catalog.Metadata["created_at"])
	assert.Equal(t, "2025-05-14T15:07:26Z", catalog.Metadata["updated_at"])
}

func TestDiscover_LeavesNullFieldsOutOfMetadata(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	// The system catalog has a null comment, null owner and null updated_at.
	result = discover(t, sampleWorld(), pluginsdk.RawConfig{"exclude_catalogs": []any{}})
	system := findAsset(result, "Catalog", "system")
	require.NotNil(t, system)
	assert.NotContains(t, system.Metadata, "comment")
	assert.NotContains(t, system.Metadata, "owner")
	assert.NotContains(t, system.Metadata, "updated_at")
	assert.NotContains(t, system.Metadata, "properties")
	assert.Nil(t, system.Description)
}

func TestDiscover_SkipsExcludedCatalogsByDefault(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	assert.Nil(t, findAsset(result, "Catalog", "system"))
	assert.NotNil(t, findAsset(result, "Catalog", "unity"))
}

func TestDiscover_KeepsOnlyTheListedCatalogs(t *testing.T) {
	result := discover(t, sampleWorld(), pluginsdk.RawConfig{"catalogs": []any{"shop"}})

	assert.NotNil(t, findAsset(result, "Catalog", "shop"))
	assert.Nil(t, findAsset(result, "Catalog", "unity"))
	assert.Nil(t, findAsset(result, "Table", "unity.default.numbers"))
}

func TestDiscover_ExcludeWinsOverTheAllowList(t *testing.T) {
	result := discover(t, sampleWorld(), pluginsdk.RawConfig{
		"catalogs":         []any{"shop", "unity"},
		"exclude_catalogs": []any{"unity"},
	})

	assert.NotNil(t, findAsset(result, "Catalog", "shop"))
	assert.Nil(t, findAsset(result, "Catalog", "unity"))
}

func TestDiscover_NamesTablesCatalogSchemaTable(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	table := findAsset(result, "Table", "unity.default.numbers")
	require.NotNil(t, table)
	assert.Equal(t, "unity.default.numbers", *table.Name)
	assert.Equal(t, "mrn://table/unity catalog/unity.default.numbers", *table.MRN)
	assert.Equal(t, "Table", table.Type)
}

func TestDiscover_CarriesTableMetadata(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	table := findAsset(result, "Table", "unity.default.numbers")
	require.NotNil(t, table)
	assert.Equal(t, "unity", table.Metadata["catalog"])
	assert.Equal(t, "default", table.Metadata["schema"])
	assert.Equal(t, "numbers", table.Metadata["table_name"])
	assert.Equal(t, "EXTERNAL", table.Metadata["table_type"])
	assert.Equal(t, "DELTA", table.Metadata["data_source_format"])
	assert.Equal(t, "file:///home/unitycatalog/etc/data/external/unity/default/tables/numbers", table.Metadata["storage_location"])
	assert.Equal(t, "External table", table.Metadata["comment"])
	assert.Equal(t, "32025924-be53-4d67-ac39-501a86046c01", table.Metadata["table_id"])
	assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, table.Metadata["properties"])
	require.NotNil(t, table.Description)
	assert.Equal(t, "External table", *table.Description)
}

func TestDiscover_IncludesColumnsWithNullableAndPartitionIndex(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	table := findAsset(result, "Table", "unity.default.user_countries")
	require.NotNil(t, table)
	columns := columnsOf(t, table)

	require.Contains(t, columns, "country")
	assert.Equal(t, "string", columns["country"]["data_type"])
	assert.Equal(t, "STRING", columns["country"]["type_name"])
	assert.Equal(t, false, columns["country"]["is_nullable"])
	assert.Equal(t, float64(0), columns["country"]["partition_index"])
	assert.Equal(t, "partition column", columns["country"]["description"])

	// Non-partition columns leave the index out entirely.
	assert.NotContains(t, columns["age"], "partition_index")
	assert.Equal(t, "bigint", columns["age"]["data_type"])
}

func TestDiscover_OrdersColumnsByPosition(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"c"}`).
		withSchema("c", `{"name":"s","catalog_name":"c"}`).
		withTable("c", "s", `{"name":"t","catalog_name":"c","schema_name":"s","table_type":"MANAGED","columns":[
			{"name":"second","type_text":"int","position":1,"nullable":true},
			{"name":"first","type_text":"int","position":0,"nullable":true}]}`)
	result := discover(t, world, nil)

	table := findAsset(result, "Table", "c.s.t")
	require.NotNil(t, table)
	assert.Equal(t, `[{"column_name":"first","data_type":"int","is_nullable":true},{"column_name":"second","data_type":"int","is_nullable":true}]`, table.Schema["columns"])
}

func TestDiscover_SkipsColumnsWhenDisabled(t *testing.T) {
	result := discover(t, sampleWorld(), pluginsdk.RawConfig{"include_columns": false})

	table := findAsset(result, "Table", "unity.default.numbers")
	require.NotNil(t, table)
	assert.NotContains(t, table.Schema, "columns")
}

func TestDiscover_FetchesColumnsWhenTheListOmitsThem(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"c"}`).
		withSchema("c", `{"name":"s","catalog_name":"c"}`).
		withTable("c", "s", `{"name":"t","catalog_name":"c","schema_name":"s","table_type":"MANAGED"}`).
		withSingleTable("c.s.t", `{"name":"t","catalog_name":"c","schema_name":"s","table_type":"MANAGED","columns":[{"name":"id","type_text":"int","position":0,"nullable":false}]}`)
	result := discover(t, world, nil)

	table := findAsset(result, "Table", "c.s.t")
	require.NotNil(t, table)
	columns := columnsOf(t, table)
	assert.Contains(t, columns, "id")
}

func TestDiscover_DoesNotFetchTablesWhenColumnsAreDisabled(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"c"}`).
		withSchema("c", `{"name":"s","catalog_name":"c"}`).
		withTable("c", "s", `{"name":"t","catalog_name":"c","schema_name":"s","table_type":"MANAGED"}`)
	discover(t, world, pluginsdk.RawConfig{"include_columns": false})

	for _, r := range world.requests {
		assert.NotEqual(t, apiPrefix+"/tables/c.s.t", r.URL.Path)
	}
}

func TestDiscover_RecordsPartitionColumns(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	table := findAsset(result, "Table", "unity.default.user_countries")
	require.NotNil(t, table)
	assert.Equal(t, []string{"country"}, table.Metadata["partition_columns"])

	numbers := findAsset(result, "Table", "unity.default.numbers")
	require.NotNil(t, numbers)
	assert.NotContains(t, numbers.Metadata, "partition_columns")
}

func TestDiscover_ViewsBecomeViewAssetsWithTheirDefinition(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	view := findAsset(result, "View", "shop.sales.big_orders")
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "VIEW", view.Metadata["table_type"])
	require.NotNil(t, view.Query)
	assert.Equal(t, "SELECT o.id FROM shop.sales.orders o JOIN customers c ON o.customer_id = c.id WHERE o.amount > 100", *view.Query)
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
	assert.NotContains(t, view.Metadata, "storage_location")
}

func TestDiscover_MaterializedViewsAreViews(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"c"}`).
		withSchema("c", `{"name":"s","catalog_name":"c"}`).
		withTable("c", "s", `{"name":"mv","catalog_name":"c","schema_name":"s","table_type":"MATERIALIZED_VIEW","view_definition":"SELECT 1"}`)
	result := discover(t, world, nil)

	assert.NotNil(t, findAsset(result, "View", "c.s.mv"))
	assert.Nil(t, findAsset(result, "Table", "c.s.mv"))
}

func TestDiscover_TablesCarryNoQuery(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	table := findAsset(result, "Table", "shop.sales.orders")
	require.NotNil(t, table)
	assert.Nil(t, table.Query)
}

func TestDiscover_CatalogContainsEveryAsset(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	catalog := assetMRN("Catalog", "unity")
	assert.True(t, hasEdge(result, catalog, assetMRN("Table", "unity.default.numbers"), "CONTAINS"))
	assert.True(t, hasEdge(result, catalog, assetMRN("Volume", "unity.default.txt_files"), "CONTAINS"))
	assert.True(t, hasEdge(result, catalog, assetMRN("Function", "unity.default.sum"), "CONTAINS"))

	shop := assetMRN("Catalog", "shop")
	assert.True(t, hasEdge(result, shop, assetMRN("View", "shop.sales.big_orders"), "CONTAINS"))
}

func TestDiscover_ViewOfEdgesFromThreePartAndBareReferences(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	view := assetMRN("View", "shop.sales.big_orders")
	// "FROM shop.sales.orders" is fully qualified; "JOIN customers" is
	// bare and resolves inside the view's own catalog and schema.
	assert.True(t, hasEdge(result, assetMRN("Table", "shop.sales.orders"), view, "VIEW_OF"))
	assert.True(t, hasEdge(result, assetMRN("Table", "shop.sales.customers"), view, "VIEW_OF"))
}

func TestDiscover_ViewOfEdgesFromTwoPartReferences(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"shop"}`).
		withSchema("shop", `{"name":"sales","catalog_name":"shop"}`).
		withSchema("shop", `{"name":"reporting","catalog_name":"shop"}`).
		withTable("shop", "sales", `{"name":"orders","catalog_name":"shop","schema_name":"sales","table_type":"MANAGED"}`).
		withTable("shop", "reporting", `{"name":"daily","catalog_name":"shop","schema_name":"reporting","table_type":"VIEW","view_definition":"SELECT * FROM sales.orders"}`)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, assetMRN("Table", "shop.sales.orders"), assetMRN("View", "shop.reporting.daily"), "VIEW_OF"))
}

func TestDiscover_ViewOfEdgesFromRecordedDependencies(t *testing.T) {
	// Databricks fills view_dependencies from its own parser; when it is
	// there it is trusted even if the SQL scan finds nothing.
	world := newFakeUC().
		withCatalog(`{"name":"shop"}`).
		withSchema("shop", `{"name":"sales","catalog_name":"shop"}`).
		withTable("shop", "sales", `{"name":"orders","catalog_name":"shop","schema_name":"sales","table_type":"MANAGED"}`).
		withTable("shop", "sales", `{"name":"v","catalog_name":"shop","schema_name":"sales","table_type":"VIEW","view_definition":"SELECT 1","view_dependencies":{"dependencies":[{"table":{"table_full_name":"shop.sales.orders"}},{"function":{"function_full_name":"shop.sales.f"}}]}}`)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, assetMRN("Table", "shop.sales.orders"), assetMRN("View", "shop.sales.v"), "VIEW_OF"))
}

func TestDiscover_SkipsViewReferencesToUnknownTables(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"shop"}`).
		withSchema("shop", `{"name":"sales","catalog_name":"shop"}`).
		withTable("shop", "sales", `{"name":"v","catalog_name":"shop","schema_name":"sales","table_type":"VIEW","view_definition":"SELECT * FROM other.place.thing"}`)
	result := discover(t, world, nil)

	for _, edge := range result.Lineage {
		assert.NotEqual(t, "VIEW_OF", edge.Type)
	}
}

func TestDiscover_ResolvesViewReferencesCaseInsensitively(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"shop"}`).
		withSchema("shop", `{"name":"sales","catalog_name":"shop"}`).
		withTable("shop", "sales", `{"name":"orders","catalog_name":"shop","schema_name":"sales","table_type":"MANAGED"}`).
		withTable("shop", "sales", `{"name":"v","catalog_name":"shop","schema_name":"sales","table_type":"VIEW","view_definition":"SELECT * FROM Shop.Sales.ORDERS"}`)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, assetMRN("Table", "shop.sales.orders"), assetMRN("View", "shop.sales.v"), "VIEW_OF"))
}

func TestDiscover_RecordsPrimaryKeyAndMarksColumns(t *testing.T) {
	world := sampleWorld().withTable("shop", "sales", paymentsTableJSON)
	result := discover(t, world, nil)

	table := findAsset(result, "Table", "shop.sales.payments")
	require.NotNil(t, table)
	assert.Equal(t, []string{"id"}, table.Metadata["primary_key"])
	assert.Equal(t, "alice@example.com", table.Metadata["owner"])

	columns := columnsOf(t, table)
	assert.Equal(t, true, columns["id"]["is_primary_key"])
	assert.NotContains(t, columns["order_id"], "is_primary_key")
}

func TestDiscover_RecordsForeignKeysInMetadata(t *testing.T) {
	world := sampleWorld().withTable("shop", "sales", paymentsTableJSON)
	result := discover(t, world, nil)

	table := findAsset(result, "Table", "shop.sales.payments")
	require.NotNil(t, table)
	assert.Equal(t, []map[string]any{{
		"name":           "fk_payments_order",
		"columns":        []string{"order_id"},
		"parent_table":   "shop.sales.orders",
		"parent_columns": []string{"id"},
	}}, table.Metadata["foreign_keys"])
}

func TestDiscover_EmitsForeignKeyEdgesToDiscoveredParents(t *testing.T) {
	world := sampleWorld().withTable("shop", "sales", paymentsTableJSON)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, assetMRN("Table", "shop.sales.payments"), assetMRN("Table", "shop.sales.orders"), "FOREIGN_KEY"))
}

func TestDiscover_SkipsForeignKeysToUnknownParents(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"shop"}`).
		withSchema("shop", `{"name":"sales","catalog_name":"shop"}`).
		withTable("shop", "sales", `{"name":"payments","catalog_name":"shop","schema_name":"sales","table_type":"MANAGED","table_constraints":[{"foreign_key_constraint":{"name":"fk","child_columns":["order_id"],"parent_table":"elsewhere.sales.orders","parent_columns":["id"]}}]}`)
	result := discover(t, world, nil)

	for _, edge := range result.Lineage {
		assert.NotEqual(t, "FOREIGN_KEY", edge.Type)
	}
}

func TestDiscover_S3StorageLocationFeedsTheTable(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	assert.True(t, hasEdge(result, mrn.New("Bucket", "S3", "marmot-lake"), assetMRN("Table", "shop.sales.orders"), "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-lake", assetMRN("Table", "shop.sales.customers"), "FEEDS"))
}

func TestDiscover_GCSStorageLocationFeedsTheTable(t *testing.T) {
	world := sampleWorld().withTable("shop", "sales", paymentsTableJSON)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, "mrn://bucket/gcs/marmot-gcs-lake", assetMRN("Table", "shop.sales.payments"), "FEEDS"))
}

func TestDiscover_AzureStorageLocationFeedsTheTable(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"c"}`).
		withSchema("c", `{"name":"s","catalog_name":"c"}`).
		withTable("c", "s", `{"name":"t","catalog_name":"c","schema_name":"s","table_type":"EXTERNAL","storage_location":"abfss://lake@acct.dfs.core.windows.net/t"}`)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, "mrn://container/azureblob/lake", assetMRN("Table", "c.s.t"), "FEEDS"))
}

func TestDiscover_LocalStorageLocationHasNoFeedsEdge(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	for _, edge := range result.Lineage {
		if edge.Type == "FEEDS" {
			assert.NotEqual(t, assetMRN("Table", "unity.default.numbers"), edge.Target)
		}
	}
}

func TestDiscover_StorageLocationFeedsVolumesToo(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"c"}`).
		withSchema("c", `{"name":"s","catalog_name":"c"}`).
		withVolume("c", "s", `{"name":"exports","catalog_name":"c","schema_name":"s","volume_type":"EXTERNAL","storage_location":"s3://marmot-lake/exports"}`)
	result := discover(t, world, nil)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-lake", assetMRN("Volume", "c.s.exports"), "FEEDS"))
}

func TestDiscover_EmitsColumnCountStatistics(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN("Table", "unity.default.user_countries") {
			stats[st.MetricName] = st.Value
		}
	}
	assert.Equal(t, float64(3), stats["asset.column_count"])
}

func TestDiscover_DiscoversVolumes(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	volume := findAsset(result, "Volume", "unity.default.txt_files")
	require.NotNil(t, volume)
	assert.Equal(t, "unity.default.txt_files", *volume.Name)
	assert.Equal(t, "MANAGED", volume.Metadata["volume_type"])
	assert.Equal(t, "file:///home/unitycatalog/etc/data/managed/unity/default/volumes/txt_files", volume.Metadata["storage_location"])
	assert.Equal(t, "74695d77-d48b-4f8e-9894-54a3e110b1ae", volume.Metadata["volume_id"])
	assert.Nil(t, volume.Description)
}

func TestDiscover_SkipsVolumesWhenDisabled(t *testing.T) {
	result := discover(t, sampleWorld(), pluginsdk.RawConfig{"include_volumes": false})

	assert.Nil(t, findAsset(result, "Volume", "unity.default.txt_files"))
}

func TestDiscover_DiscoversFunctionsWithParameters(t *testing.T) {
	result := discover(t, sampleWorld(), nil)

	function := findAsset(result, "Function", "unity.default.sum")
	require.NotNil(t, function)
	assert.Equal(t, []string{"x int", "y int", "z int"}, function.Metadata["parameters"])
	assert.Equal(t, "INT", function.Metadata["return_type"])
	assert.Equal(t, "EXTERNAL", function.Metadata["routine_body"])
	assert.Equal(t, "python", function.Metadata["language"])
	assert.Equal(t, true, function.Metadata["is_deterministic"])
	assert.Equal(t, "NO_SQL", function.Metadata["sql_data_access"])
	require.NotNil(t, function.Description)
	assert.Equal(t, "Adds two numbers.", *function.Description)
	require.NotNil(t, function.Query)
	assert.Equal(t, "t = x + y + z\\nreturn t", *function.Query)
	require.NotNil(t, function.QueryLanguage)
	assert.Equal(t, "python", *function.QueryLanguage)
}

func TestDiscover_SQLFunctionsUseTheFullReturnType(t *testing.T) {
	world := sampleWorld().withFunction("shop", "sales", netAmountFunctionJSON)
	result := discover(t, world, nil)

	function := findAsset(result, "Function", "shop.sales.net_amount")
	require.NotNil(t, function)
	assert.Equal(t, []string{"gross decimal(10,2)", "rate double"}, function.Metadata["parameters"])
	assert.Equal(t, "decimal(10,2)", function.Metadata["return_type"])
	assert.Equal(t, "SQL", function.Metadata["language"])
	require.NotNil(t, function.QueryLanguage)
	assert.Equal(t, "SQL", *function.QueryLanguage)
}

func TestDiscover_SkipsFunctionsWhenDisabled(t *testing.T) {
	result := discover(t, sampleWorld(), pluginsdk.RawConfig{"include_functions": false})

	assert.Nil(t, findAsset(result, "Function", "unity.default.sum"))
}

func modelWorld() *fakeUC {
	return newFakeUC().
		withCatalog(`{"name":"ml","comment":"Model registry"}`).
		withSchema("ml", `{"name":"models","catalog_name":"ml"}`).
		withModel("ml", "models", churnModelJSON).
		withVersions("ml.models.churn", churnVersion1JSON, churnVersion2JSON)
}

func TestDiscover_DiscoversModelsWithVersions(t *testing.T) {
	result := discover(t, modelWorld(), nil)

	model := findAsset(result, "Model", "ml.models.churn")
	require.NotNil(t, model)
	assert.Equal(t, "ml.models.churn", *model.Name)
	assert.Equal(t, 2, model.Metadata["version_count"])
	assert.Equal(t, 2, model.Metadata["latest_version"])
	assert.Equal(t, map[string]string{"1": "PENDING_REGISTRATION", "2": "READY"}, model.Metadata["versions"])
	assert.Equal(t, "a89ad57e-58e2-4c72-89fa-52f382eaeb0f", model.Metadata["model_id"])
	require.NotNil(t, model.Description)
	assert.Equal(t, "Customer churn classifier", *model.Description)
	assert.True(t, hasEdge(result, assetMRN("Catalog", "ml"), *model.MRN, "CONTAINS"))
}

func TestDiscover_ModelWithoutVersionsHasNoLatest(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"ml"}`).
		withSchema("ml", `{"name":"models","catalog_name":"ml"}`).
		withModel("ml", "models", churnModelJSON).
		withVersions("ml.models.churn")
	result := discover(t, world, nil)

	model := findAsset(result, "Model", "ml.models.churn")
	require.NotNil(t, model)
	assert.Equal(t, 0, model.Metadata["version_count"])
	assert.NotContains(t, model.Metadata, "latest_version")
	assert.NotContains(t, model.Metadata, "versions")
}

func TestDiscover_TreatsAMissingVersionsEndpointAsUnsupported(t *testing.T) {
	world := newFakeUC().
		withCatalog(`{"name":"ml"}`).
		withSchema("ml", `{"name":"models","catalog_name":"ml"}`).
		withModel("ml", "models", churnModelJSON)
	result := discover(t, world, nil)

	model := findAsset(result, "Model", "ml.models.churn")
	require.NotNil(t, model)
	assert.Equal(t, 0, model.Metadata["version_count"])
}

func TestDiscover_SurvivesAServerWithoutModels(t *testing.T) {
	result := discover(t, sampleWorld().without("/models"), nil)

	assert.NotNil(t, findAsset(result, "Table", "unity.default.numbers"))
}

func TestDiscover_SkipsModelsWhenDisabled(t *testing.T) {
	result := discover(t, modelWorld(), pluginsdk.RawConfig{"include_models": false})

	assert.Nil(t, findAsset(result, "Model", "ml.models.churn"))
}

func TestDiscover_FollowsPaginationToTheEnd(t *testing.T) {
	world := newFakeUC().withCatalog(`{"name":"c"}`).withSchema("c", `{"name":"s","catalog_name":"c"}`)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		world.withTable("c", "s", `{"name":"`+name+`","catalog_name":"c","schema_name":"s","table_type":"MANAGED"}`)
	}
	result := discover(t, world, pluginsdk.RawConfig{"page_size": 2})

	for _, name := range []string{"a", "b", "c", "d", "e"} {
		assert.NotNil(t, findAsset(result, "Table", "c.s."+name), name)
	}

	pages := 0
	for _, r := range world.requests {
		if r.URL.Path == apiPrefix+"/tables" {
			pages++
			assert.Equal(t, "2", r.URL.Query().Get("max_results"))
		}
	}
	assert.Equal(t, 3, pages)
}

func TestDiscover_StopsWhenAServerRepeatsThePageToken(t *testing.T) {
	// A server that answers every page with the same token would loop
	// forever; the client notices the repeat and stops on the second page.
	loop := &loopingServer{}
	server := loop.start(t)

	s := &Source{}
	result, err := s.Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL})
	require.NoError(t, err)

	assert.Equal(t, 2, loop.pages)
	assert.NotEmpty(t, result.Assets)
}

func TestDiscover_SendsTheBearerToken(t *testing.T) {
	world := sampleWorld()
	discover(t, world, pluginsdk.RawConfig{"token": "secret"})

	require.NotEmpty(t, world.requests)
	for _, r := range world.requests {
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
	}
}

func TestDiscover_SendsNoAuthorizationWithoutAToken(t *testing.T) {
	world := sampleWorld()
	discover(t, world, nil)

	for _, r := range world.requests {
		assert.Empty(t, r.Header.Get("Authorization"))
	}
}

func TestDiscover_FailsWhenTheServerIsUnreachable(t *testing.T) {
	s := &Source{}
	_, err := s.Discover(t.Context(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing catalogs")
}

func TestDiscover_FailsWhenTheServerErrorsOnCatalogs(t *testing.T) {
	world := sampleWorld()
	world.status = http.StatusInternalServerError
	server := world.start(t)

	s := &Source{}
	_, err := s.Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "boom")
}

func TestDiscover_OneBrokenSchemaDoesNotStopTheRun(t *testing.T) {
	world := sampleWorld().without("/volumes", "/functions")
	result := discover(t, world, nil)

	assert.NotNil(t, findAsset(result, "Table", "unity.default.numbers"))
	assert.NotNil(t, findAsset(result, "Catalog", "unity"))
	assert.Nil(t, findAsset(result, "Volume", "unity.default.txt_files"))
}

func TestDiscover_ACatalogWhoseSchemasFailStillAppears(t *testing.T) {
	result := discover(t, sampleWorld().without("/schemas"), nil)

	catalog := findAsset(result, "Catalog", "unity")
	require.NotNil(t, catalog)
	assert.Equal(t, 0, catalog.Metadata["schema_count"])
}

func TestDiscover_InterpolatesTags(t *testing.T) {
	result := discover(t, sampleWorld(), pluginsdk.RawConfig{"tags": []any{"uc", "catalog:${catalog}"}})

	table := findAsset(result, "Table", "unity.default.numbers")
	require.NotNil(t, table)
	assert.Equal(t, []string{"uc", "catalog:unity"}, table.Tags)
}

func TestDiscover_EveryAssetMRNAgreesWithItsOwnFields(t *testing.T) {
	world := sampleWorld().withTable("shop", "sales", paymentsTableJSON).withFunction("shop", "sales", netAmountFunctionJSON)
	result := discover(t, world, nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestDiscover_EveryEdgeEndsOnARunAssetOrAnotherPluginsIdentity(t *testing.T) {
	world := sampleWorld().withTable("shop", "sales", paymentsTableJSON)
	result := discover(t, world, nil)

	own := make(map[string]struct{})
	for _, a := range result.Assets {
		own[*a.MRN] = struct{}{}
	}
	for _, edge := range result.Lineage {
		_, targetKnown := own[edge.Target]
		assert.True(t, targetKnown, "edge target %s is not an asset of this run", edge.Target)

		if _, sourceKnown := own[edge.Source]; !sourceKnown {
			assert.Equal(t, "FEEDS", edge.Type, "only storage edges may start outside the run: %s", edge.Source)
		}
	}
}
