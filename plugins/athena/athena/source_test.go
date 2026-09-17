package athena

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/athena"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_AcceptsAnEmptyConfig(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{})

	require.NoError(t, err)
	assert.NotNil(t, s.config)
}

func TestValidate_AllocatesTheAWSSectionWhenAbsent(t *testing.T) {
	// Reading TagsToMetadata would panic on a nil section, and a config that
	// names no AWS setting at all is valid: credentials come from the
	// environment.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{})

	require.NoError(t, err)
	require.NotNil(t, s.config.AWSConfig)
	assert.False(t, s.config.TagsToMetadata)
}

func TestValidate_DefaultsTheIncludeFlagsToTrue(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{})

	require.NoError(t, err)
	assert.True(t, s.config.IncludeWorkGroups)
	assert.True(t, s.config.IncludeSavedQueries)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludePartitions)
	assert.True(t, s.config.DiscoverLineage)
}

func TestValidate_RespectsAnExplicitFalse(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"include_workgroups":    false,
		"include_saved_queries": false,
		"include_columns":       false,
		"include_partitions":    false,
		"discover_lineage":      false,
	})

	require.NoError(t, err)
	assert.False(t, s.config.IncludeWorkGroups)
	assert.False(t, s.config.IncludeSavedQueries)
	assert.False(t, s.config.IncludeColumns)
	assert.False(t, s.config.IncludePartitions)
	assert.False(t, s.config.DiscoverLineage)
}

func TestValidate_DefaultsTheMetadataAPIToAuto(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{})

	require.NoError(t, err)
	assert.Equal(t, metadataAPIAuto, s.config.MetadataAPI)
}

func TestValidate_RejectsAnUnknownMetadataAPI(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"metadata_api": "hive"})

	require.Error(t, err)
}

func TestValidate_AcceptsStaticCredentials(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]any{
			"region": "us-east-1",
			"id":     "AKIAIOSFODNN7EXAMPLE",
			"secret": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "us-east-1", s.config.Credentials.Region)
}

func TestValidate_RejectsANonURLEndpoint(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]any{"endpoint": "not a url"},
	})

	require.Error(t, err)
}

func TestDiscover_CatalogIsFiledUnderTheAthenaProvider(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	catalog := findAsset(t, assets, "AwsDataCatalog")

	assert.Equal(t, typeCatalog, catalog.Type)
	assert.Equal(t, []string{providerAthena}, catalog.Providers)
	assert.Equal(t, "GLUE", catalog.Metadata["type"])
}

func TestDiscover_CatalogCarriesTheDescriptionFromGetDataCatalog(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	catalog := findAsset(t, assets, "AwsDataCatalog")

	require.NotNil(t, catalog.Description)
	assert.Equal(t, "The account Glue Data Catalog", *catalog.Description)
}

func TestDiscover_DatabaseIsFiledUnderTheGlueProvider(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	database := findAsset(t, assets, "shop")

	assert.Equal(t, typeDatabase, database.Type)
	assert.Equal(t, []string{providerGlue}, database.Providers)
	assert.Equal(t, "AwsDataCatalog", athenaSection(t, database)["catalog"])
}

func TestDiscover_TableIsFiledUnderTheGlueProviderByItsBareName(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	orders := findAsset(t, assets, "orders")

	assert.Equal(t, typeTable, orders.Type)
	assert.Equal(t, []string{providerGlue}, orders.Providers)
	assert.Equal(t, "mrn://table/glue/orders", *orders.MRN)
}

func TestDiscover_TableCarriesTheAthenaStorageFacts(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	facts := athenaSection(t, findAsset(t, assets, "orders"))

	assert.Equal(t, "shop", facts["database"])
	assert.Equal(t, "EXTERNAL_TABLE", facts["table_type"])
	assert.Equal(t, "s3://marmot-lake/orders/", facts["location"])
	assert.Equal(t, "parquet", facts["classification"])
	assert.Equal(t, "dt", facts["partition_keys"])
	assert.Equal(t, true, facts["partition_projection"])
}

func TestDiscover_TableCommentBecomesTheDescription(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	orders := findAsset(t, assets, "orders")

	require.NotNil(t, orders.Description)
	assert.Equal(t, "One row per order", *orders.Description)
}

func TestDiscover_ColumnsKeepTheHiveTypeVerbatim(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	columns := columnsOf(t, findAsset(t, assets, "orders"))

	require.Len(t, columns, 3)
	assert.Equal(t, "order_id", columns[0].Name)
	assert.Equal(t, "bigint", columns[0].DataType)
	assert.Equal(t, "Order identifier", columns[0].Description)
	assert.Equal(t, "array<struct<sku:string,qty:int>>", columns[1].DataType)
}

func TestDiscover_PartitionKeysComeAfterTheRegularColumns(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	columns := columnsOf(t, findAsset(t, assets, "orders"))

	require.Len(t, columns, 3)
	assert.Equal(t, "dt", columns[2].Name)
	assert.True(t, columns[2].IsPartitionKey)
	assert.False(t, columns[0].IsPartitionKey)
}

func TestDiscover_ColumnsAreNullableBecauseAthenaHasNoNotNull(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	columns := columnsOf(t, findAsset(t, assets, "orders"))

	require.NotEmpty(t, columns)
	assert.True(t, columns[0].Nullable)
}

func TestDiscover_IncludeColumnsFalseLeavesTheSchemaEmpty(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"include_columns": false})
	orders := findAsset(t, assets, "orders")

	assert.Empty(t, orders.Schema)
}

func TestDiscover_IncludePartitionsFalseDropsThePartitionColumn(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"include_partitions": false})
	orders := findAsset(t, assets, "orders")

	columns := columnsOf(t, orders)
	require.Len(t, columns, 2)
	assert.NotContains(t, athenaSection(t, orders), "partition_keys")
}

func TestDiscover_VirtualViewBecomesAViewAsset(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	view := findAsset(t, assets, "order_totals")

	assert.Equal(t, typeView, view.Type)
	assert.Equal(t, "mrn://view/glue/order_totals", *view.MRN)
}

func TestDiscover_ViewCarriesTheDecodedSQL(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	view := findAsset(t, assets, "order_totals")

	require.NotNil(t, view.Query)
	assert.Equal(t, "SELECT sum(total) FROM shop.orders", *view.Query)
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
}

func TestDiscover_WorkGroupIsFiledUnderTheAthenaProvider(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	workGroup := findAsset(t, assets, "analytics")

	assert.Equal(t, typeWorkGroup, workGroup.Type)
	assert.Equal(t, []string{providerAthena}, workGroup.Providers)
	assert.Equal(t, "ENABLED", workGroup.Metadata["state"])
}

func TestDiscover_WorkGroupCarriesItsResultConfiguration(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	workGroup := findAsset(t, assets, "analytics")

	assert.Equal(t, "s3://marmot-results/analytics/", workGroup.Metadata["output_location"])
	assert.Equal(t, "SSE_S3", workGroup.Metadata["encryption"])
	assert.Equal(t, "Athena engine version 3", workGroup.Metadata["engine_version"])
	assert.Equal(t, int64(10_000_000), workGroup.Metadata["bytes_scanned_cutoff"])
	assert.Equal(t, true, workGroup.Metadata["enforce_configuration"])
}

func TestDiscover_WorkGroupLinksToTheAthenaConsole(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	workGroup := findAsset(t, assets, "analytics")

	require.Len(t, workGroup.ExternalLinks, 1)
	assert.Equal(t, "Open in Athena", workGroup.ExternalLinks[0].Name)
	assert.Contains(t, workGroup.ExternalLinks[0].URL, "#/workgroups/details/analytics")
}

func TestDiscover_SavedQueryIsNamedAfterItsWorkGroup(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	query := findAsset(t, assets, "analytics/daily-revenue")

	assert.Equal(t, typeSavedQuery, query.Type)
	assert.Equal(t, []string{providerAthena}, query.Providers)
	assert.Equal(t, "nq-1", query.Metadata["named_query_id"])
	assert.Equal(t, "shop", query.Metadata["database"])
}

func TestDiscover_SavedQueryCarriesItsSQL(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{})
	query := findAsset(t, assets, "analytics/daily-revenue")

	require.NotNil(t, query.Query)
	assert.Contains(t, *query.Query, "FROM orders o JOIN customers c")
	require.NotNil(t, query.QueryLanguage)
	assert.Equal(t, "SQL", *query.QueryLanguage)
}

func TestDiscover_CatalogContainsItsDatabase(t *testing.T) {
	lineage := discoverLineage(t, pluginsdk.RawConfig{})

	assert.Contains(t, lineage, edge(catalogMRN("AwsDataCatalog"), databaseMRN("shop"), "CONTAINS"))
}

func TestDiscover_DatabaseContainsItsTables(t *testing.T) {
	lineage := discoverLineage(t, pluginsdk.RawConfig{})

	assert.Contains(t, lineage, edge(databaseMRN("shop"), tableMRN(typeTable, "orders"), "CONTAINS"))
	assert.Contains(t, lineage, edge(databaseMRN("shop"), tableMRN(typeView, "order_totals"), "CONTAINS"))
}

func TestDiscover_BucketFeedsTheTableItStores(t *testing.T) {
	// The bucket is the storage the table reads, so the data flows into the
	// table, not out of it.
	lineage := discoverLineage(t, pluginsdk.RawConfig{})

	assert.Contains(t, lineage, edge(bucketMRN("marmot-lake"), tableMRN(typeTable, "orders"), "FEEDS"))
}

func TestDiscover_WorkGroupProducesItsResultBucket(t *testing.T) {
	lineage := discoverLineage(t, pluginsdk.RawConfig{})

	assert.Contains(t, lineage, edge(workGroupMRN("analytics"), bucketMRN("marmot-results"), "PRODUCES"))
}

func TestDiscover_WorkGroupContainsItsSavedQueries(t *testing.T) {
	lineage := discoverLineage(t, pluginsdk.RawConfig{})

	assert.Contains(t, lineage,
		edge(workGroupMRN("analytics"), savedQueryMRN("analytics", "daily-revenue"), "CONTAINS"))
}

func TestDiscover_TablesASavedQueryReadsFeedIt(t *testing.T) {
	// The query names orders and customers unqualified, so both resolve
	// against its own database.
	lineage := discoverLineage(t, pluginsdk.RawConfig{})
	target := savedQueryMRN("analytics", "daily-revenue")

	assert.Contains(t, lineage, edge(tableMRN(typeTable, "orders"), target, "FEEDS"))
	assert.Contains(t, lineage, edge(tableMRN(typeTable, "customers"), target, "FEEDS"))
}

func TestDiscover_LineageOffEmitsNoEdges(t *testing.T) {
	lineage := discoverLineage(t, pluginsdk.RawConfig{"discover_lineage": false})

	assert.Empty(t, lineage)
}

func TestDiscover_WorkGroupsOffDropsTheWorkGroupAssetButKeepsSavedQueries(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"include_workgroups": false})

	assert.Nil(t, lookupAsset(assets, "analytics"))
	assert.NotNil(t, lookupAsset(assets, "analytics/daily-revenue"))
}

func TestDiscover_SavedQueriesOffDropsTheQueryAsset(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"include_saved_queries": false})

	assert.Nil(t, lookupAsset(assets, "analytics/daily-revenue"))
	assert.NotNil(t, lookupAsset(assets, "analytics"))
}

func TestDiscover_ExcludedCatalogIsSkipped(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"exclude_catalogs": []any{"AwsDataCatalog"}})

	assert.Nil(t, lookupAsset(assets, "AwsDataCatalog"))
	assert.Nil(t, lookupAsset(assets, "shop"))
	assert.NotNil(t, lookupAsset(assets, "hive-lab"))
}

func TestDiscover_CatalogAllowListKeepsOnlyTheNamedCatalogs(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"catalogs": []any{"AwsDataCatalog"}})

	assert.NotNil(t, lookupAsset(assets, "AwsDataCatalog"))
	assert.Nil(t, lookupAsset(assets, "hive-lab"))
}

func TestDiscover_ExcludedDatabaseIsSkippedWithItsTables(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"exclude_databases": []any{"shop"}})

	assert.Nil(t, lookupAsset(assets, "shop"))
	assert.Nil(t, lookupAsset(assets, "orders"))
}

func TestDiscover_WorkGroupAllowListKeepsOnlyTheNamedWorkGroups(t *testing.T) {
	assets := discoverAssets(t, pluginsdk.RawConfig{"workgroups": []any{"primary"}})

	assert.Nil(t, lookupAsset(assets, "analytics"))
}

func TestDiscover_FallsBackToTheDefaultCatalogWhenNoneAreRegistered(t *testing.T) {
	// An account that registered no catalog of its own still queries the
	// Glue Data Catalog through AwsDataCatalog.
	fake := seedAthena()
	fake.catalogs = nil
	s := newTestSource(t, fake, seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	assert.NotNil(t, lookupAsset(result.Assets, "AwsDataCatalog"))
	assert.NotNil(t, lookupAsset(result.Assets, "shop"))
}

func TestDiscover_ReadsTheCatalogThroughGlueWhenListTableMetadataFails(t *testing.T) {
	// Not every Athena compatible endpoint implements the metadata API. A
	// GLUE catalog is the Glue Data Catalog, so Glue answers the same
	// question.
	fake := seedAthena()
	fake.tableMetadataErr = errNotImplemented
	s := newTestSource(t, fake, seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	orders := lookupAsset(result.Assets, "orders")
	require.NotNil(t, orders)
	assert.Equal(t, "mrn://table/glue/orders", *orders.MRN)
	assert.Equal(t, "s3://marmot-lake/orders/", athenaSection(t, *orders)["location"])
}

func TestDiscover_GlueFallbackStillReadsPartitionKeys(t *testing.T) {
	fake := seedAthena()
	fake.tableMetadataErr = errNotImplemented
	s := newTestSource(t, fake, seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	columns := columnsOf(t, *lookupAsset(result.Assets, "orders"))
	require.Len(t, columns, 2)
	assert.Equal(t, "dt", columns[1].Name)
	assert.True(t, columns[1].IsPartitionKey)
}

func TestDiscover_MetadataAPIGlueSkipsNonGlueCatalogs(t *testing.T) {
	s := newTestSource(t, seedAthena(), seedGlue(), pluginsdk.RawConfig{"metadata_api": "glue"})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	// The catalog itself is still catalogued, only its contents are not read.
	assert.NotNil(t, lookupAsset(result.Assets, "hive-lab"))
	assert.NotNil(t, lookupAsset(result.Assets, "orders"))
}

func TestDiscover_FallsBackToGetNamedQueryWhenTheBatchCallIsUnavailable(t *testing.T) {
	fake := seedAthena()
	fake.batchErr = errNotImplemented
	s := newTestSource(t, fake, seedGlue(), pluginsdk.RawConfig{})

	result, err := s.discover(t.Context())
	require.NoError(t, err)

	assert.NotNil(t, lookupAsset(result.Assets, "analytics/daily-revenue"))
	assert.Equal(t, 1, fake.getNamedQueryCalls)
}

func TestDiscover_ReportsAnUnreachableAthenaAsAnError(t *testing.T) {
	s := newTestSource(t, &failingAthena{}, seedGlue(), pluginsdk.RawConfig{})

	_, err := s.discover(t.Context())

	require.Error(t, err)
}

// helpers

func discoverAssets(t *testing.T, raw pluginsdk.RawConfig) []pluginsdk.Asset {
	t.Helper()

	s := newTestSource(t, seedAthena(), seedGlue(), raw)
	result, err := s.discover(t.Context())
	require.NoError(t, err)

	return result.Assets
}

func discoverLineage(t *testing.T, raw pluginsdk.RawConfig) []pluginsdk.LineageEdge {
	t.Helper()

	s := newTestSource(t, seedAthena(), seedGlue(), raw)
	result, err := s.discover(t.Context())
	require.NoError(t, err)

	return result.Lineage
}

func lookupAsset(assets []pluginsdk.Asset, name string) *pluginsdk.Asset {
	for i, a := range assets {
		if a.Name != nil && *a.Name == name {
			return &assets[i]
		}
	}
	return nil
}

func findAsset(t *testing.T, assets []pluginsdk.Asset, name string) pluginsdk.Asset {
	t.Helper()

	a := lookupAsset(assets, name)
	require.NotNil(t, a, "no asset named %s", name)
	return *a
}

// athenaSection reads the athena block of an asset's metadata, where the
// plugin keeps what it knows about a Glue owned asset.
func athenaSection(t *testing.T, a pluginsdk.Asset) map[string]any {
	t.Helper()

	section, ok := a.Metadata["athena"].(map[string]any)
	require.True(t, ok, "asset %s has no athena metadata section", *a.Name)
	return section
}

func columnsOf(t *testing.T, a pluginsdk.Asset) []athenaColumn {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "asset %s has no columns", *a.Name)

	var columns []athenaColumn
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))
	return columns
}

// failingAthena stands in for an Athena endpoint that cannot be reached.
type failingAthena struct{ fakeAthena }

func (f *failingAthena) ListDataCatalogs(ctx context.Context, in *athena.ListDataCatalogsInput, opts ...func(*athena.Options)) (*athena.ListDataCatalogsOutput, error) {
	return nil, errors.New("dial tcp 127.0.0.1:443: connection refused")
}

func TestDiscover_EveryAssetRecordsAthenaAsItsSource(t *testing.T) {
	// A table carries the Glue provider, but the source entry has to stay
	// distinct or a Glue run and an Athena run erase each other's properties
	// instead of both being recorded on the asset.
	assets := discoverAssets(t, pluginsdk.RawConfig{})

	for _, a := range assets {
		require.Len(t, a.Sources, 1, "asset %s", *a.Name)
		assert.Equal(t, "Athena", a.Sources[0].Name, "asset %s", *a.Name)
	}
}
