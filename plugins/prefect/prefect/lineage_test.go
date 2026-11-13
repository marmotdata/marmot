package prefect

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Asset materializations exist on Prefect Cloud only, so these tests run
// against fixtures rather than the end to end server.

func TestParseAssetKey_PostgresUsesDatabaseSchemaAndTable(t *testing.T) {
	// The PostgreSQL plugin names a table database.schema.table, so an edge has to use that shape or the server drops it.
	target, ok := parseAssetKey("postgres://warehouse.internal/shop/public/orders")

	require.True(t, ok)
	assert.Equal(t, "PostgreSQL", target.Provider)
	assert.Equal(t, "Table", target.Type)
	assert.Equal(t, []string{"shop", "public", "orders"}, target.Path)
}

func TestParseAssetKey_AcceptsThePostgresqlSpelling(t *testing.T) {
	target, ok := parseAssetKey("postgresql://warehouse.internal/shop/public/orders")

	require.True(t, ok)
	assert.Equal(t, "PostgreSQL", target.Provider)
}

func TestParseAssetKey_MySQLUsesDatabaseAndTable(t *testing.T) {
	target, ok := parseAssetKey("mysql://db.internal/shop/orders")

	require.True(t, ok)
	assert.Equal(t, "MySQL", target.Provider)
	assert.Equal(t, []string{"shop", "orders"}, target.Path)
}

func TestParseAssetKey_SnowflakeUsesDatabaseSchemaAndTable(t *testing.T) {
	// The Snowflake plugin names a table database.schema.table, unlike
	// PostgreSQL.
	target, ok := parseAssetKey("snowflake://acme.snowflakecomputing.com/SHOP/PUBLIC/ORDERS")

	require.True(t, ok)
	assert.Equal(t, "Snowflake", target.Provider)
	assert.Equal(t, []string{"SHOP", "PUBLIC", "ORDERS"}, target.Path)
}

func TestParseAssetKey_BigQueryUsesProjectDatasetAndTable(t *testing.T) {
	target, ok := parseAssetKey("bigquery://bigquery.googleapis.com/my-project/analytics/orders")

	require.True(t, ok)
	assert.Equal(t, "BigQuery", target.Provider)
	assert.Equal(t, []string{"my-project", "analytics", "orders"}, target.Path)
}

func TestParseAssetKey_S3UsesTheBucketNotTheObject(t *testing.T) {
	// Marmot catalogues S3 buckets, not the objects inside them.
	target, ok := parseAssetKey("s3://raw-events/orders/2026-09-08.json")

	require.True(t, ok)
	assert.Equal(t, "S3", target.Provider)
	assert.Equal(t, "Bucket", target.Type)
	assert.Equal(t, []string{"raw-events"}, target.Path)
}

func TestParseAssetKey_GCSUsesTheBucket(t *testing.T) {
	target, ok := parseAssetKey("gs://landing-zone/orders.parquet")

	require.True(t, ok)
	assert.Equal(t, "GCS", target.Provider)
	assert.Equal(t, []string{"landing-zone"}, target.Path)
}

func TestParseAssetKey_RejectsASchemeMarmotCannotName(t *testing.T) {
	// Guessing a provider here would produce an edge pointing at an asset
	// nothing owns, which the server silently drops.
	_, ok := parseAssetKey("kafka://broker/orders")

	assert.False(t, ok)
}

func TestParseAssetKey_RejectsSnowflakeWithoutAllThreeParts(t *testing.T) {
	_, ok := parseAssetKey("snowflake://acme.snowflakecomputing.com/ORDERS")

	assert.False(t, ok)
}

func TestParseAssetKey_RejectsAKeyWithNoScheme(t *testing.T) {
	_, ok := parseAssetKey("shop.public.orders")

	assert.False(t, ok)
}

func TestParseAssetKey_RejectsAKeyWithOnlyAHost(t *testing.T) {
	_, ok := parseAssetKey("postgres://warehouse.internal")

	assert.False(t, ok)
}

func TestMaterializationEdges_PipelineProducesWhatItWrote(t *testing.T) {
	edges := materializationEdges("mrn://asset/pipeline/prefect/nightly-etl", []AssetMaterialization{{
		AssetKey: "postgres://warehouse.internal/shop/public/order_rollups",
	}})

	assert.Contains(t, edges, pluginsdk.LineageEdge{
		Source: "mrn://asset/pipeline/prefect/nightly-etl",
		Target: "mrn://asset/table/postgresql@host=warehouse.internal:5432/shop.public.order_rollups",
		Type:   "PRODUCES",
	})
}

func TestMaterializationEdges_UpstreamAssetFeedsThePipeline(t *testing.T) {
	edges := materializationEdges("mrn://asset/pipeline/prefect/nightly-etl", []AssetMaterialization{{
		AssetKey:       "postgres://warehouse.internal/shop/public/order_rollups",
		UpstreamAssets: []string{"postgres://warehouse.internal/shop/public/orders"},
	}})

	assert.Contains(t, edges, pluginsdk.LineageEdge{
		Source: "mrn://asset/table/postgresql@host=warehouse.internal:5432/shop.public.orders",
		Target: "mrn://asset/pipeline/prefect/nightly-etl",
		Type:   "FEEDS",
	})
}

func TestMaterializationEdges_BucketUpstreamFeedsThePipeline(t *testing.T) {
	edges := materializationEdges("mrn://asset/pipeline/prefect/nightly-etl", []AssetMaterialization{{
		AssetKey:       "postgres://warehouse.internal/shop/public/order_rollups",
		UpstreamAssets: []string{"s3://raw-events/orders/2026-09-08.json"},
	}})

	assert.Contains(t, edges, pluginsdk.LineageEdge{
		Source: "mrn://asset/bucket/s3/raw-events",
		Target: "mrn://asset/pipeline/prefect/nightly-etl",
		Type:   "FEEDS",
	})
}

func TestMaterializationEdges_SkipsKeysMarmotCannotName(t *testing.T) {
	edges := materializationEdges("mrn://asset/pipeline/prefect/nightly-etl", []AssetMaterialization{{
		AssetKey:       "mystery://somewhere/thing",
		UpstreamAssets: []string{"also-not-a-uri"},
	}})

	assert.Empty(t, edges)
}

func TestMaterializationEdges_StatesEachPairOnce(t *testing.T) {
	edges := materializationEdges("mrn://asset/pipeline/prefect/nightly-etl", []AssetMaterialization{
		{
			AssetKey:       "postgres://warehouse.internal/shop/public/order_rollups",
			UpstreamAssets: []string{"postgres://warehouse.internal/shop/public/orders"},
		},
		{
			AssetKey:       "postgres://warehouse.internal/shop/public/order_rollups",
			UpstreamAssets: []string{"postgres://warehouse.internal/shop/public/orders"},
		},
	})

	assert.Len(t, edges, 2)
}

func TestDiscover_ReadsCloudAssetMaterializations(t *testing.T) {
	fake := seededPrefect(t).withMaterializations(t, nightlyRunID, materializationsFixture)

	result := discover(t, fake, nil)

	assert.True(t, hasEdge(result,
		"mrn://asset/pipeline/prefect/nightly-etl",
		"mrn://asset/table/postgresql@host=warehouse.internal:5432/shop.public.order_rollups",
		"PRODUCES"))
	assert.True(t, hasEdge(result,
		"mrn://asset/table/postgresql@host=warehouse.internal:5432/shop.public.orders",
		"mrn://asset/pipeline/prefect/nightly-etl",
		"FEEDS"))
	assert.True(t, hasEdge(result,
		"mrn://asset/bucket/s3/raw-events",
		"mrn://asset/pipeline/prefect/nightly-etl",
		"FEEDS"))
}

func TestDiscover_EmitsNoMaterializationLineageWithoutAnAssetsAPI(t *testing.T) {
	// A self-hosted server answers 404 for the Cloud-only Assets API,
	// which is not an error worth failing discovery over.
	result := discover(t, seededPrefect(t), nil)

	for _, edge := range result.Lineage {
		assert.NotEqual(t, "PRODUCES", edge.Type)
		assert.NotEqual(t, "FEEDS", edge.Type)
	}
}
