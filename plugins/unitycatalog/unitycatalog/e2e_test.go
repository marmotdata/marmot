package unitycatalog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Unity Catalog server:
//
//	/tmp/bin/docker run -d --name marmot-test-unitycatalog -p 18090:8080 unitycatalog/unitycatalog:latest
//	MARMOT_TEST_UNITYCATALOG_URL=http://localhost:18090 go test ./...

const provider = "Unity Catalog"

// serverURL returns the server under test, or skips when none is set.
func serverURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("MARMOT_TEST_UNITYCATALOG_URL")
	if url == "" {
		t.Skip("set MARMOT_TEST_UNITYCATALOG_URL to run the Unity Catalog e2e tests")
	}
	return strings.TrimSuffix(url, "/")
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// post sends one create request. Objects seeded by an earlier test run are
// still there, so a rejection saying the object exists is success.
func post(t *testing.T, baseURL, path, body string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		baseURL+"/api/2.1/unity-catalog"+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "seeding %s", path)
	defer resp.Body.Close()

	answer, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == http.StatusOK || strings.Contains(string(answer), "ALREADY_EXISTS") {
		return
	}
	t.Fatalf("seeding %s failed with status %d: %s", path, resp.StatusCode, answer)
}

// seed creates the objects the assertions below expect. The image also
// ships a sample "unity" catalog, which the tests read but never change.
func seed(t *testing.T, baseURL string) {
	t.Helper()

	post(t, baseURL, "/catalogs", `{"name":"shop","comment":"Web shop data","properties":{"team":"commerce"}}`)
	post(t, baseURL, "/schemas", `{"name":"sales","catalog_name":"shop","comment":"Sales facts"}`)

	post(t, baseURL, "/tables", `{
		"name":"customers","catalog_name":"shop","schema_name":"sales",
		"table_type":"EXTERNAL","data_source_format":"DELTA",
		"storage_location":"s3://marmot-lake/customers",
		"comment":"Customer master data","properties":{"owner_team":"crm"},
		"columns":[
			{"name":"id","type_text":"int","type_json":"{\"name\":\"id\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}","type_name":"INT","position":0,"comment":"Customer id","nullable":false},
			{"name":"email","type_text":"string","type_json":"{\"name\":\"email\",\"type\":\"string\",\"nullable\":true,\"metadata\":{}}","type_name":"STRING","position":1,"nullable":true}
		]}`)

	post(t, baseURL, "/tables", `{
		"name":"orders","catalog_name":"shop","schema_name":"sales",
		"table_type":"EXTERNAL","data_source_format":"DELTA",
		"storage_location":"s3://marmot-lake/orders","comment":"One row per order",
		"columns":[
			{"name":"id","type_text":"int","type_json":"{\"name\":\"id\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}","type_name":"INT","position":0,"comment":"Order id","nullable":false},
			{"name":"customer_id","type_text":"int","type_json":"{\"name\":\"customer_id\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}","type_name":"INT","position":1,"nullable":false},
			{"name":"amount","type_text":"decimal(10,2)","type_json":"{\"name\":\"amount\",\"type\":\"decimal(10,2)\",\"nullable\":true,\"metadata\":{}}","type_name":"DECIMAL","type_precision":10,"type_scale":2,"position":2,"nullable":true},
			{"name":"order_date","type_text":"date","type_json":"{\"name\":\"order_date\",\"type\":\"date\",\"nullable\":false,\"metadata\":{}}","type_name":"DATE","position":3,"nullable":false,"partition_index":0}
		]}`)

	post(t, baseURL, "/tables", `{
		"name":"payments","catalog_name":"shop","schema_name":"sales",
		"table_type":"EXTERNAL","data_source_format":"PARQUET",
		"storage_location":"gs://marmot-gcs-lake/payments","comment":"Payments taken per order",
		"columns":[
			{"name":"id","type_text":"int","type_json":"{\"name\":\"id\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}","type_name":"INT","position":0,"nullable":false},
			{"name":"order_id","type_text":"int","type_json":"{\"name\":\"order_id\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}","type_name":"INT","position":1,"nullable":false}
		]}`)

	post(t, baseURL, "/tables", `{
		"name":"big_orders","catalog_name":"shop","schema_name":"sales",
		"table_type":"VIEW","data_source_format":"DELTA","comment":"Orders above 100",
		"view_definition":"SELECT o.id FROM shop.sales.orders o JOIN customers c ON o.customer_id = c.id WHERE o.amount > 100",
		"columns":[
			{"name":"id","type_text":"int","type_json":"{\"name\":\"id\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}","type_name":"INT","position":0,"nullable":false}
		]}`)

	post(t, baseURL, "/volumes", `{
		"name":"raw_exports","catalog_name":"shop","schema_name":"sales","volume_type":"EXTERNAL",
		"storage_location":"s3://marmot-lake/exports","comment":"Nightly CSV exports"}`)

	post(t, baseURL, "/functions", `{"function_info":{
		"name":"net_amount","catalog_name":"shop","schema_name":"sales",
		"input_params":{"parameters":[
			{"name":"gross","type_text":"decimal(10,2)","type_json":"{\"name\":\"gross\",\"type\":\"decimal(10,2)\",\"nullable\":false,\"metadata\":{}}","type_name":"DECIMAL","type_precision":10,"type_scale":2,"position":0,"parameter_mode":"IN","parameter_type":"PARAM"},
			{"name":"rate","type_text":"double","type_json":"{\"name\":\"rate\",\"type\":\"double\",\"nullable\":false,\"metadata\":{}}","type_name":"DOUBLE","position":1,"parameter_mode":"IN","parameter_type":"PARAM"}]},
		"data_type":"DECIMAL","full_data_type":"decimal(10,2)",
		"routine_body":"SQL","routine_definition":"gross * (1 - rate)","parameter_style":"S",
		"is_deterministic":true,"sql_data_access":"CONTAINS_SQL","is_null_call":false,
		"security_type":"DEFINER","specific_name":"net_amount","comment":"Gross minus tax"}}`)

	// Registered models need a catalog with a storage root to write into.
	post(t, baseURL, "/catalogs", `{"name":"ml","comment":"Model registry","storage_root":"file:///tmp/uc-ml"}`)
	post(t, baseURL, "/schemas", `{"name":"models","catalog_name":"ml","comment":"Registered models"}`)
	post(t, baseURL, "/models", `{"name":"churn","catalog_name":"ml","schema_name":"models","comment":"Customer churn classifier"}`)
	seedModelVersions(t, baseURL)
}

// seedModelVersions adds two versions the first time it runs. Versions are
// numbered by the server, so they cannot be created twice.
func seedModelVersions(t *testing.T, baseURL string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		baseURL+"/api/2.1/unity-catalog/models/ml.models.churn/versions", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var page struct {
		ModelVersions []json.RawMessage `json:"model_versions"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	if len(page.ModelVersions) >= 2 {
		return
	}

	post(t, baseURL, "/models/versions", `{"model_name":"churn","catalog_name":"ml","schema_name":"models","source":"file:///tmp/uc-ml/src/churn-1","run_id":"run-1","comment":"first cut"}`)
	post(t, baseURL, "/models/versions", `{"model_name":"churn","catalog_name":"ml","schema_name":"models","source":"file:///tmp/uc-ml/src/churn-2","run_id":"run-2","comment":"second cut"}`)

	// A new version stays PENDING_REGISTRATION until it is finalised, so
	// finalise the second one to get two different statuses to assert on.
	finalize(t, baseURL)
}

// finalize marks model version 2 ready.
func finalize(t *testing.T, baseURL string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		baseURL+"/api/2.1/unity-catalog/models/ml.models.churn/versions/2/finalize",
		bytes.NewBufferString(`{"full_name":"ml.models.churn","version":2}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "finalising model version 2: %s", body)
}

// discoverAll runs the plugin binary against the seeded server.
func discoverAll(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	url := serverURL(t)
	seed(t, url)

	result, err := buildBinary(t).Discover(t.Context(), pluginsdk.RawConfig{"host": url})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	want := mrn.New(assetType, provider, name)
	for i := range result.Assets {
		if result.Assets[i].MRN != nil && *result.Assets[i].MRN == want {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

// stringsOf reads a metadata list. Metadata crosses the plugin wire as
// JSON, so a []string arrives as []any of strings.
func stringsOf(t *testing.T, value any) []string {
	t.Helper()

	items, ok := value.([]any)
	require.True(t, ok, "expected a list, got %T", value)

	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		require.True(t, ok, "expected a string, got %T", item)
		result = append(result, text)
	}
	return result
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column list on %s", *a.Name)

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]any, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func TestE2E_Meta(t *testing.T) {
	serverURL(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "unitycatalog", meta.ID)
	assert.Equal(t, "Unity Catalog", meta.Name)
	assert.Equal(t, "catalog", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	serverURL(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestE2E_ValidateAcceptsTheServerURL(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"host": serverURL(t)})
	require.NoError(t, err)
}

func TestE2E_DiscoversTheSampleCatalog(t *testing.T) {
	result := discoverAll(t)

	catalog := findAsset(result, "Catalog", "unity")
	require.NotNil(t, catalog)
	assert.Equal(t, []string{provider}, catalog.Providers)
	require.NotNil(t, catalog.Description)
	assert.Equal(t, "reference catalog using oss UC", *catalog.Description)
	assert.Equal(t, "f029b870-9468-4f10-badc-630b41e5690d", catalog.Metadata["catalog_id"])
}

func TestE2E_SkipsTheSystemCatalogByDefault(t *testing.T) {
	result := discoverAll(t)

	assert.Nil(t, findAsset(result, "Catalog", "system"))
}

func TestE2E_DiscoversTheSampleTables(t *testing.T) {
	result := discoverAll(t)

	for _, name := range []string{"numbers", "marksheet", "marksheet_uniform", "user_countries"} {
		assert.NotNil(t, findAsset(result, "Table", "unity.default."+name), name)
	}
}

func TestE2E_SampleTableCarriesColumnsAndComments(t *testing.T) {
	result := discoverAll(t)

	table := findAsset(result, "Table", "unity.default.numbers")
	require.NotNil(t, table)
	assert.Equal(t, "EXTERNAL", table.Metadata["table_type"])
	assert.Equal(t, "DELTA", table.Metadata["data_source_format"])

	columns := columnsOf(t, table)
	require.Contains(t, columns, "as_int")
	assert.Equal(t, "int", columns["as_int"]["data_type"])
	assert.Equal(t, "INT", columns["as_int"]["type_name"])
	assert.Equal(t, false, columns["as_int"]["is_nullable"])
	assert.Equal(t, "Int column", columns["as_int"]["description"])
}

func TestE2E_RecordsPartitionColumns(t *testing.T) {
	result := discoverAll(t)

	table := findAsset(result, "Table", "unity.default.user_countries")
	require.NotNil(t, table)
	assert.Equal(t, []string{"country"}, stringsOf(t, table.Metadata["partition_columns"]))

	columns := columnsOf(t, table)
	assert.Equal(t, float64(0), columns["country"]["partition_index"])
	assert.NotContains(t, columns["first_name"], "partition_index")
}

func TestE2E_DiscoversASeededTableWithItsMetadata(t *testing.T) {
	result := discoverAll(t)

	table := findAsset(result, "Table", "shop.sales.orders")
	require.NotNil(t, table)
	assert.Equal(t, "mrn://table/unity-catalog/shop.sales.orders", *table.MRN)
	assert.Equal(t, "shop", table.Metadata["catalog"])
	assert.Equal(t, "sales", table.Metadata["schema"])
	assert.Equal(t, "orders", table.Metadata["table_name"])
	assert.Equal(t, "s3://marmot-lake/orders", table.Metadata["storage_location"])
	require.NotNil(t, table.Description)
	assert.Equal(t, "One row per order", *table.Description)
	assert.Equal(t, []string{"order_date"}, stringsOf(t, table.Metadata["partition_columns"]))

	columns := columnsOf(t, table)
	assert.Equal(t, "decimal(10,2)", columns["amount"]["data_type"])
	assert.Equal(t, true, columns["amount"]["is_nullable"])
	assert.Equal(t, false, columns["id"]["is_nullable"])
	assert.Equal(t, "Order id", columns["id"]["description"])
}

func TestE2E_DiscoversTheSeededViewWithItsDefinition(t *testing.T) {
	result := discoverAll(t)

	view := findAsset(result, "View", "shop.sales.big_orders")
	require.NotNil(t, view)
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "VIEW", view.Metadata["table_type"])
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "FROM shop.sales.orders")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
}

func TestE2E_DiscoversVolumes(t *testing.T) {
	result := discoverAll(t)

	sample := findAsset(result, "Volume", "unity.default.txt_files")
	require.NotNil(t, sample)
	assert.Equal(t, "MANAGED", sample.Metadata["volume_type"])

	seeded := findAsset(result, "Volume", "shop.sales.raw_exports")
	require.NotNil(t, seeded)
	assert.Equal(t, "EXTERNAL", seeded.Metadata["volume_type"])
	assert.Equal(t, "s3://marmot-lake/exports", seeded.Metadata["storage_location"])
}

func TestE2E_DiscoversFunctionsWithParametersAndBodies(t *testing.T) {
	result := discoverAll(t)

	sample := findAsset(result, "Function", "unity.default.sum")
	require.NotNil(t, sample)
	assert.Equal(t, []string{"x int", "y int", "z int"}, stringsOf(t, sample.Metadata["parameters"]))
	assert.Equal(t, "EXTERNAL", sample.Metadata["routine_body"])
	assert.Equal(t, "python", sample.Metadata["language"])
	require.NotNil(t, sample.Query)
	assert.Contains(t, *sample.Query, "x + y + z")

	seeded := findAsset(result, "Function", "shop.sales.net_amount")
	require.NotNil(t, seeded)
	assert.Equal(t, []string{"gross decimal(10,2)", "rate double"}, stringsOf(t, seeded.Metadata["parameters"]))
	assert.Equal(t, "decimal(10,2)", seeded.Metadata["return_type"])
	assert.Equal(t, "SQL", seeded.Metadata["language"])
	assert.Equal(t, "CONTAINS_SQL", seeded.Metadata["sql_data_access"])
	require.NotNil(t, seeded.Query)
	assert.Equal(t, "gross * (1 - rate)", *seeded.Query)
}

func TestE2E_DiscoversRegisteredModelsWithVersions(t *testing.T) {
	result := discoverAll(t)

	model := findAsset(result, "Model", "ml.models.churn")
	require.NotNil(t, model)
	require.NotNil(t, model.Description)
	assert.Equal(t, "Customer churn classifier", *model.Description)
	// Numbers cross the wire as JSON, so they arrive as float64.
	assert.Equal(t, float64(2), model.Metadata["version_count"])
	assert.Equal(t, float64(2), model.Metadata["latest_version"])

	versions, ok := model.Metadata["versions"].(map[string]any)
	require.True(t, ok, "expected a version map, got %T", model.Metadata["versions"])
	assert.Contains(t, versions, "1")
	assert.Equal(t, "READY", versions["2"])
}

func TestE2E_CatalogsContainTheirObjects(t *testing.T) {
	result := discoverAll(t)

	unity := mrn.New("Catalog", provider, "unity")
	assert.True(t, hasEdge(result, unity, mrn.New("Table", provider, "unity.default.numbers"), "CONTAINS"))
	assert.True(t, hasEdge(result, unity, mrn.New("Volume", provider, "unity.default.txt_files"), "CONTAINS"))
	assert.True(t, hasEdge(result, unity, mrn.New("Function", provider, "unity.default.sum"), "CONTAINS"))

	shop := mrn.New("Catalog", provider, "shop")
	assert.True(t, hasEdge(result, shop, mrn.New("View", provider, "shop.sales.big_orders"), "CONTAINS"))
	assert.True(t, hasEdge(result, mrn.New("Catalog", provider, "ml"), mrn.New("Model", provider, "ml.models.churn"), "CONTAINS"))
}

func TestE2E_ViewOfEdgesPointAtTheViewsBaseTables(t *testing.T) {
	result := discoverAll(t)

	view := mrn.New("View", provider, "shop.sales.big_orders")
	// "FROM shop.sales.orders" is fully qualified, "JOIN customers" is bare.
	assert.True(t, hasEdge(result, mrn.New("Table", provider, "shop.sales.orders"), view, "VIEW_OF"))
	assert.True(t, hasEdge(result, mrn.New("Table", provider, "shop.sales.customers"), view, "VIEW_OF"))
}

func TestE2E_CloudStorageFeedsTheObjectsItHolds(t *testing.T) {
	result := discoverAll(t)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-lake", mrn.New("Table", provider, "shop.sales.orders"), "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-lake", mrn.New("Volume", provider, "shop.sales.raw_exports"), "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://bucket/gcs/marmot-gcs-lake", mrn.New("Table", provider, "shop.sales.payments"), "FEEDS"))
}

func TestE2E_LocalStorageLocationsGetNoFeedsEdge(t *testing.T) {
	result := discoverAll(t)

	// The sample tables sit on file:// paths, which are not cloud storage.
	numbers := mrn.New("Table", provider, "unity.default.numbers")
	for _, edge := range result.Lineage {
		if edge.Type == "FEEDS" {
			assert.NotEqual(t, numbers, edge.Target)
		}
	}
}

func TestE2E_EmitsColumnCountStatistics(t *testing.T) {
	result := discoverAll(t)

	counts := make(map[string]float64)
	for _, stat := range result.Statistics {
		assert.Equal(t, "asset.column_count", stat.MetricName)
		counts[stat.AssetMRN] = stat.Value
	}

	assert.Equal(t, float64(4), counts[mrn.New("Table", provider, "shop.sales.orders")])
	assert.Equal(t, float64(3), counts[mrn.New("Table", provider, "unity.default.user_countries")])
}

func TestE2E_EveryAssetMRNAgreesWithItsOwnFields(t *testing.T) {
	result := discoverAll(t)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestE2E_KeepsOnlyTheListedCatalogs(t *testing.T) {
	url := serverURL(t)
	seed(t, url)

	result, err := buildBinary(t).Discover(t.Context(), pluginsdk.RawConfig{
		"host":     url,
		"catalogs": []any{"shop"},
	})
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, "Catalog", "shop"))
	assert.Nil(t, findAsset(result, "Catalog", "unity"))
}

func TestE2E_FailsAgainstAServerThatIsNotThere(t *testing.T) {
	serverURL(t)

	_, err := buildBinary(t).Discover(context.Background(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing catalogs")
}
