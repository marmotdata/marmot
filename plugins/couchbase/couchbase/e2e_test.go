package couchbase_test

import (
	"encoding/json"
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Couchbase Server seeded
// with a bucket "shop" whose scope "sales" holds "orders" (primary index,
// 3 documents) and "customers" (secondary index on email, 2 documents),
// plus 2 documents in _default._default with no index at all.
//
// Set MARMOT_TEST_COUCHBASE_CONNECTION (for example couchbase://localhost),
// MARMOT_TEST_COUCHBASE_USERNAME and MARMOT_TEST_COUCHBASE_PASSWORD to run
// them.

const (
	bucketMRN    = "mrn://bucket/couchbase/shop"
	ordersMRN    = "mrn://collection/couchbase/shop.sales.orders"
	customersMRN = "mrn://collection/couchbase/shop.sales.customers"
	defaultMRN   = "mrn://collection/couchbase/shop._default._default"
)

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	connection := os.Getenv("MARMOT_TEST_COUCHBASE_CONNECTION")
	if connection == "" {
		t.Skip("MARMOT_TEST_COUCHBASE_CONNECTION not set, skipping Couchbase e2e tests")
	}

	return pluginsdk.RawConfig{
		"connection_string": connection,
		"username":          os.Getenv("MARMOT_TEST_COUCHBASE_USERNAME"),
		"password":          os.Getenv("MARMOT_TEST_COUCHBASE_PASSWORD"),
		"bucket":            "shop",
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, mrnValue string) pluginsdk.Asset {
	t.Helper()
	for _, a := range result.Assets {
		if a.MRN != nil && *a.MRN == mrnValue {
			return a
		}
	}
	t.Fatalf("asset %s not found in %d discovered assets", mrnValue, len(result.Assets))
	return pluginsdk.Asset{}
}

// columnsOf decodes the column list an asset carries in its schema.
func columnsOf(t *testing.T, a pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "asset %s has no columns", *a.MRN)

	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &list))

	columns := make(map[string]map[string]interface{}, len(list))
	for _, c := range list {
		columns[c["column_name"].(string)] = c
	}
	return columns
}

func statistic(result *pluginsdk.DiscoveryResult, mrnValue, metric string) (float64, bool) {
	for _, s := range result.Statistics {
		if s.AssetMRN == mrnValue && s.MetricName == metric {
			return s.Value, true
		}
	}
	return 0, false
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "couchbase", meta.ID)
	assert.Equal(t, "Couchbase", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Source implements DataFetcher")
}

func TestE2E_ValidateMissingConnectionStringFails(t *testing.T) {
	e2eConfig(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"username": "a", "password": "b"})
	require.Error(t, err)
}

func TestE2E_ValidateWrongPasswordIsNotCheckedUntilDiscover(t *testing.T) {
	config := e2eConfig(t)
	config["password"] = "definitely-wrong"
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), config)
	require.NoError(t, err, "Validate only checks the shape of the config")

	_, err = bin.Discover(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_DiscoverBucketAsset(t *testing.T) {
	result := discover(t)

	bucket := findAsset(t, result, bucketMRN)
	assert.Equal(t, "Bucket", bucket.Type)
	assert.Equal(t, []string{"Couchbase"}, bucket.Providers)
	require.NotNil(t, bucket.Name)
	assert.Equal(t, "shop", *bucket.Name)

	assert.Equal(t, "couchbase", bucket.Metadata["bucket_type"])
	assert.EqualValues(t, 256, bucket.Metadata["ram_quota_mb"])
	assert.EqualValues(t, 1, bucket.Metadata["replicas"])
	assert.Equal(t, "couchstore", bucket.Metadata["storage_backend"])
	assert.Equal(t, "valueOnly", bucket.Metadata["eviction_policy"])
	assert.Equal(t, "seqno", bucket.Metadata["conflict_resolution"])
	assert.Equal(t, false, bucket.Metadata["flush_enabled"])
	// _default and sales; _system is skipped by default.
	assert.EqualValues(t, 2, bucket.Metadata["scope_count"])
	assert.EqualValues(t, 3, bucket.Metadata["collection_count"])
	// 3 orders + 2 customers + 2 products.
	assert.EqualValues(t, 7, bucket.Metadata["item_count"])
	assert.Contains(t, bucket.Metadata["cluster_version"], "7.6")
	assert.NotZero(t, bucket.Metadata["data_used_bytes"])
	assert.NotZero(t, bucket.Metadata["mem_used_bytes"])

	size, ok := statistic(result, bucketMRN, "asset.size_bytes")
	assert.True(t, ok, "expected asset.size_bytes for the bucket")
	assert.Greater(t, size, 0.0)
}

func TestE2E_DiscoverSkipsSystemScope(t *testing.T) {
	result := discover(t)

	for _, a := range result.Assets {
		assert.NotContains(t, *a.Name, "_system")
	}
}

func TestE2E_DiscoverCollectionWithPrimaryIndex(t *testing.T) {
	result := discover(t)

	orders := findAsset(t, result, ordersMRN)
	assert.Equal(t, "Collection", orders.Type)
	assert.Equal(t, []string{"Couchbase"}, orders.Providers)
	assert.Equal(t, "shop.sales.orders", *orders.Name)

	assert.Equal(t, "shop", orders.Metadata["bucket"])
	assert.Equal(t, "sales", orders.Metadata["scope"])
	assert.Equal(t, "orders", orders.Metadata["collection"])
	assert.EqualValues(t, 1, orders.Metadata["index_count"])
	assert.Equal(t, true, orders.Metadata["primary_index"])
	assert.Equal(t, []interface{}{"orders_primary"}, orders.Metadata["indexes"])
	assert.EqualValues(t, 3, orders.Metadata["document_count"])

	rows, ok := statistic(result, ordersMRN, "asset.row_count")
	assert.True(t, ok, "expected asset.row_count for orders")
	assert.Equal(t, 3.0, rows)
}

func TestE2E_DiscoverInfersColumnsFromSampledDocuments(t *testing.T) {
	result := discover(t)

	orders := findAsset(t, result, ordersMRN)
	columns := columnsOf(t, orders)

	// A field every document has, with one type.
	require.Contains(t, columns, "order_id")
	assert.Equal(t, "number", columns["order_id"]["data_type"])
	assert.Equal(t, false, columns["order_id"]["is_nullable"])
	assert.EqualValues(t, 1, columns["order_id"]["occurrence"])

	// One order stores its total as a string, so the type is mixed.
	require.Contains(t, columns, "total")
	assert.Equal(t, "number|string", columns["total"]["data_type"])

	// An object field is listed with its children one level deep.
	require.Contains(t, columns, "shipping")
	assert.Equal(t, "object", columns["shipping"]["data_type"])
	assert.Equal(t, true, columns["shipping"]["is_nullable"], "order 3 has no shipping")
	require.Contains(t, columns, "shipping.city")
	assert.Equal(t, "string", columns["shipping.city"]["data_type"])
	require.Contains(t, columns, "shipping.postcode")
	assert.Equal(t, true, columns["shipping.postcode"]["is_nullable"])

	require.Contains(t, columns, "items")
	assert.Equal(t, "array", columns["items"]["data_type"])
	require.Contains(t, columns, "note")
	assert.Equal(t, "null", columns["note"]["data_type"])

	// The document key is not a field of the document.
	assert.NotContains(t, columns, "_id")

	count, ok := statistic(result, ordersMRN, "asset.column_count")
	assert.True(t, ok, "expected asset.column_count for orders")
	assert.Equal(t, float64(len(columns)), count)
}

func TestE2E_DiscoverCollectionWithSecondaryIndexOnly(t *testing.T) {
	result := discover(t)

	customers := findAsset(t, result, customersMRN)
	assert.EqualValues(t, 1, customers.Metadata["index_count"])
	assert.Equal(t, false, customers.Metadata["primary_index"])
	assert.Equal(t, []interface{}{"customers_email"}, customers.Metadata["indexes"])

	columns := columnsOf(t, customers)
	require.Contains(t, columns, "email")
	assert.Equal(t, "string", columns["email"]["data_type"])
	require.Contains(t, columns, "active")
	assert.Equal(t, "boolean", columns["active"]["data_type"])
	require.Contains(t, columns, "address.city")
	assert.Equal(t, true, columns["address.city"]["is_nullable"])
}

func TestE2E_DiscoverCollectionWithoutAnyIndex(t *testing.T) {
	result := discover(t)

	products := findAsset(t, result, defaultMRN)
	assert.Equal(t, "shop._default._default", *products.Name)
	assert.EqualValues(t, 0, products.Metadata["index_count"])
	assert.Equal(t, false, products.Metadata["primary_index"])
	assert.NotContains(t, products.Metadata, "indexes")

	// Couchbase 7.6 answers the sample query with a sequential scan; older
	// servers fall back to INFER. Either way the fields come through.
	columns := columnsOf(t, products)
	require.Contains(t, columns, "sku")
	assert.Equal(t, "string", columns["sku"]["data_type"])
	require.Contains(t, columns, "price")
	assert.Equal(t, "number", columns["price"]["data_type"])
	require.Contains(t, columns, "tags")
	assert.Equal(t, "array", columns["tags"]["data_type"])
	assert.Equal(t, true, columns["tags"]["is_nullable"])
}

func TestE2E_DiscoverLinksBucketToCollections(t *testing.T) {
	result := discover(t)

	targets := make(map[string]bool)
	for _, e := range result.Lineage {
		if e.Type == "CONTAINS" && e.Source == bucketMRN {
			targets[e.Target] = true
		}
	}
	assert.True(t, targets[ordersMRN])
	assert.True(t, targets[customersMRN])
	assert.True(t, targets[defaultMRN])
	assert.Len(t, result.Lineage, 3)
}

func TestE2E_DiscoverEveryMRNAgreesWithItsOwnFields(t *testing.T) {
	result := discover(t)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestE2E_DiscoverWithoutColumnsOrStatistics(t *testing.T) {
	config := e2eConfig(t)
	config["include_columns"] = false
	config["include_statistics"] = false
	config["include_indexes"] = false
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	orders := findAsset(t, result, ordersMRN)
	assert.NotContains(t, orders.Schema, "columns")
	assert.NotContains(t, orders.Metadata, "document_count")
	assert.NotContains(t, orders.Metadata, "index_count")
	assert.Empty(t, result.Statistics)
}

func TestE2E_DiscoverExcludedBucketIsSkipped(t *testing.T) {
	config := e2eConfig(t)
	delete(config, "bucket")
	config["exclude_buckets"] = []interface{}{"shop"}
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	for _, a := range result.Assets {
		assert.NotEqual(t, bucketMRN, *a.MRN)
	}
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	name := "shop.sales.orders"
	columns, rows, err := bin.FetchSampleData(t.Context(), config, &pluginsdk.Asset{
		Name: &name,
		Type: "Collection",
		Metadata: map[string]interface{}{
			"bucket":     "shop",
			"scope":      "sales",
			"collection": "orders",
		},
	})
	require.NoError(t, err)

	require.NotEmpty(t, columns)
	assert.Equal(t, "id", columns[0], "the document key comes first")
	assert.Contains(t, columns, "order_id")
	assert.Contains(t, columns, "shipping")
	assert.Len(t, rows, 3)
	for _, row := range rows {
		assert.Len(t, row, len(columns))
	}
}

func TestE2E_FetchSampleDataRejectsBucketAssets(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	name := "shop"
	_, _, err := bin.FetchSampleData(t.Context(), config, &pluginsdk.Asset{
		Name:     &name,
		Type:     "Bucket",
		Metadata: map[string]interface{}{},
	})
	require.Error(t, err)
}
