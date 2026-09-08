package athena_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Athena and Glue API served by
// moto. plugintest.Build compiles the main package and every call spawns the
// process, runs one RPC and kills it again.
//
// Start the endpoint with:
//
//	docker run -d --name marmot-test-athena -p 15558:5000 motoserver/moto:latest
//	MARMOT_TEST_ATHENA_ENDPOINT=http://localhost:15558 go test ./athena/

const (
	testRegion    = "us-east-1"
	testAccessKey = "test"
	testSecretKey = "test"
)

func endpoint(t *testing.T) string {
	t.Helper()

	value := os.Getenv("MARMOT_TEST_ATHENA_ENDPOINT")
	if value == "" {
		t.Skip("MARMOT_TEST_ATHENA_ENDPOINT is not set, skipping the Athena end to end tests")
	}
	return value
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func pluginConfig(endpoint string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"credentials": map[string]any{
			"region":   testRegion,
			"id":       testAccessKey,
			"secret":   testSecretKey,
			"endpoint": endpoint,
		},
	}
}

func awsConfigFor(t *testing.T, endpoint string) aws.Config {
	t.Helper()

	cfg, err := awsconfig.LoadDefaultConfig(t.Context(),
		awsconfig.WithRegion(testRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(testAccessKey, testSecretKey, ""),
		),
	)
	require.NoError(t, err)

	cfg.BaseEndpoint = aws.String(endpoint)
	return cfg
}

// seed puts the fixture into moto. It resets the endpoint first so a rerun
// starts from the same state, then writes the catalog through Glue and the
// Athena-only objects through Athena.
func seed(t *testing.T, endpoint string) {
	t.Helper()

	reset(t, endpoint)

	cfg := awsConfigFor(t, endpoint)
	glueClient := glue.NewFromConfig(cfg)
	athenaClient := athena.NewFromConfig(cfg)
	ctx := t.Context()

	_, err := glueClient.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &gluetypes.DatabaseInput{
			Name:        aws.String("shop"),
			Description: aws.String("Storefront tables"),
			LocationUri: aws.String("s3://marmot-lake/shop/"),
		},
	})
	require.NoError(t, err)

	_, err = glueClient.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String("shop"),
		TableInput: &gluetypes.TableInput{
			Name:        aws.String("orders"),
			TableType:   aws.String("EXTERNAL_TABLE"),
			Description: aws.String("One row per order"),
			StorageDescriptor: &gluetypes.StorageDescriptor{
				Location:     aws.String("s3://marmot-lake/orders/"),
				InputFormat:  aws.String("org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat"),
				OutputFormat: aws.String("org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat"),
				SerdeInfo: &gluetypes.SerDeInfo{
					SerializationLibrary: aws.String("org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe"),
				},
				Columns: []gluetypes.Column{
					{Name: aws.String("order_id"), Type: aws.String("bigint"), Comment: aws.String("Order identifier")},
					{Name: aws.String("customer_id"), Type: aws.String("bigint")},
					{Name: aws.String("total"), Type: aws.String("double")},
					{Name: aws.String("items"), Type: aws.String("array<struct<sku:string,qty:int>>")},
				},
			},
			PartitionKeys: []gluetypes.Column{
				{Name: aws.String("dt"), Type: aws.String("string"), Comment: aws.String("Ingest date")},
			},
			Parameters: map[string]string{
				"classification":     "parquet",
				"projection.enabled": "true",
			},
		},
	})
	require.NoError(t, err)

	_, err = glueClient.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String("shop"),
		TableInput: &gluetypes.TableInput{
			Name:      aws.String("customers"),
			TableType: aws.String("EXTERNAL_TABLE"),
			StorageDescriptor: &gluetypes.StorageDescriptor{
				Location: aws.String("s3://marmot-lake/customers/"),
				Columns: []gluetypes.Column{
					{Name: aws.String("customer_id"), Type: aws.String("bigint")},
					{Name: aws.String("name"), Type: aws.String("string")},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = glueClient.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String("shop"),
		TableInput: &gluetypes.TableInput{
			Name:             aws.String("order_totals"),
			TableType:        aws.String("VIRTUAL_VIEW"),
			ViewOriginalText: aws.String(prestoView("SELECT sum(total) FROM shop.orders")),
			StorageDescriptor: &gluetypes.StorageDescriptor{
				Columns: []gluetypes.Column{
					{Name: aws.String("total"), Type: aws.String("double")},
				},
			},
			Parameters: map[string]string{"presto_view": "true"},
		},
	})
	require.NoError(t, err)

	_, err = athenaClient.CreateDataCatalog(ctx, &athena.CreateDataCatalogInput{
		Name:        aws.String("AwsDataCatalog"),
		Type:        athenatypes.DataCatalogTypeGlue,
		Description: aws.String("The account Glue Data Catalog"),
	})
	require.NoError(t, err)

	_, err = athenaClient.CreateDataCatalog(ctx, &athena.CreateDataCatalogInput{
		Name:        aws.String("hive_lab"),
		Type:        athenatypes.DataCatalogTypeHive,
		Description: aws.String("A federated Hive metastore"),
		Parameters:  map[string]string{"metadata-function": "arn:aws:lambda:us-east-1:123456789012:function:hive"},
	})
	require.NoError(t, err)

	_, err = athenaClient.CreateWorkGroup(ctx, &athena.CreateWorkGroupInput{
		Name:        aws.String("analytics"),
		Description: aws.String("Analyst queries"),
		Tags:        []athenatypes.Tag{{Key: aws.String("team"), Value: aws.String("analytics")}},
		Configuration: &athenatypes.WorkGroupConfiguration{
			EnforceWorkGroupConfiguration:   aws.Bool(true),
			PublishCloudWatchMetricsEnabled: aws.Bool(true),
			BytesScannedCutoffPerQuery:      aws.Int64(10_000_000),
			EngineVersion: &athenatypes.EngineVersion{
				SelectedEngineVersion:  aws.String("AUTO"),
				EffectiveEngineVersion: aws.String("Athena engine version 3"),
			},
			ResultConfiguration: &athenatypes.ResultConfiguration{
				OutputLocation: aws.String("s3://marmot-results/analytics/"),
				EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
					EncryptionOption: athenatypes.EncryptionOptionSseS3,
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = athenaClient.CreateNamedQuery(ctx, &athena.CreateNamedQueryInput{
		Name:        aws.String("daily-revenue"),
		Description: aws.String("Revenue per day"),
		Database:    aws.String("shop"),
		WorkGroup:   aws.String("analytics"),
		QueryString: aws.String("SELECT c.name, sum(o.total) FROM orders o JOIN customers c ON c.customer_id = o.customer_id GROUP BY 1"),
	})
	require.NoError(t, err)
}

// reset clears every backend so a rerun starts from a known state.
func reset(t *testing.T, endpoint string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint+"/moto-api/reset", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// prestoView wraps SQL the way Athena stores a view definition.
func prestoView(sql string) string {
	document, err := json.Marshal(map[string]any{
		"originalSql": sql,
		"catalog":     "awsdatacatalog",
		"schema":      "shop",
	})
	if err != nil {
		panic(err)
	}
	return "/* Presto View: " + base64.StdEncoding.EncodeToString(document) + " */"
}

func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	url := endpoint(t)
	seed(t, url)

	result, err := buildBinary(t).Discover(t.Context(), pluginConfig(url))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "athena", meta.ID)
	assert.Equal(t, "AWS Athena", meta.Name)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestE2E_ValidateRejectsAnUnknownMetadataAPI(t *testing.T) {
	_, err := buildBinary(t).Validate(context.Background(), pluginsdk.RawConfig{"metadata_api": "hive"})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheEndpointConfig(t *testing.T) {
	url := endpoint(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginConfig(url))

	require.NoError(t, err)
}

func TestE2E_DiscoversTheGlueDatabaseAndTables(t *testing.T) {
	result := discover(t)

	assert.Equal(t, "mrn://database/glue/shop", mrnOf(t, result, "shop"))
	assert.Equal(t, "mrn://table/glue/orders", mrnOf(t, result, "orders"))
	assert.Equal(t, "mrn://table/glue/customers", mrnOf(t, result, "customers"))
}

func TestE2E_TablesAreFiledUnderTheGlueProvider(t *testing.T) {
	// An Athena run and a Glue run have to land on one asset, so the tables
	// carry the Glue provider even though Athena discovered them.
	result := discover(t)

	assert.Equal(t, []string{"Glue"}, findAsset(t, result, "orders").Providers)
	assert.Equal(t, []string{"Athena"}, findAsset(t, result, "analytics").Providers)
}

func TestE2E_TableCarriesItsStorageFacts(t *testing.T) {
	result := discover(t)
	facts := athenaFacts(t, findAsset(t, result, "orders"))

	assert.Equal(t, "shop", facts["database"])
	assert.Equal(t, "AwsDataCatalog", facts["catalog"])
	assert.Equal(t, "EXTERNAL_TABLE", facts["table_type"])
	assert.Equal(t, "s3://marmot-lake/orders/", facts["location"])
	assert.Equal(t, "parquet", facts["classification"])
	assert.Equal(t, "dt", facts["partition_keys"])
}

func TestE2E_ColumnsKeepTheHiveTypeAndPartitionKeysComeLast(t *testing.T) {
	result := discover(t)
	columns := columns(t, findAsset(t, result, "orders"))

	require.Len(t, columns, 5)
	assert.Equal(t, "order_id", columns[0].ColumnName)
	assert.Equal(t, "bigint", columns[0].DataType)
	assert.Equal(t, "Order identifier", columns[0].Description)
	assert.Equal(t, "array<struct<sku:string,qty:int>>", columns[3].DataType)
	assert.Equal(t, "dt", columns[4].ColumnName)
	assert.True(t, columns[4].IsPartitionKey)
}

func TestE2E_VirtualViewBecomesAViewWithItsSQL(t *testing.T) {
	result := discover(t)
	view := findAsset(t, result, "order_totals")

	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "mrn://view/glue/order_totals", *view.MRN)
	require.NotNil(t, view.Query)
	assert.Equal(t, "SELECT sum(total) FROM shop.orders", *view.Query)
}

func TestE2E_WorkGroupCarriesItsResultConfiguration(t *testing.T) {
	result := discover(t)
	workGroup := findAsset(t, result, "analytics")

	assert.Equal(t, "WorkGroup", workGroup.Type)
	assert.Equal(t, "s3://marmot-results/analytics/", workGroup.Metadata["output_location"])
	assert.Equal(t, "Athena engine version 3", workGroup.Metadata["engine_version"])
	assert.Equal(t, "ENABLED", workGroup.Metadata["state"])
}

func TestE2E_SavedQueryCarriesItsSQL(t *testing.T) {
	result := discover(t)
	query := findAsset(t, result, "analytics/daily-revenue")

	assert.Equal(t, "Data Model Object", query.Type)
	assert.Equal(t, "mrn://data-model-object/athena/analytics-daily-revenue", *query.MRN)
	require.NotNil(t, query.Query)
	assert.Contains(t, *query.Query, "JOIN customers c")
	assert.Equal(t, "shop", query.Metadata["database"])
}

func TestE2E_DataCatalogsAreCatalogued(t *testing.T) {
	result := discover(t)

	assert.Equal(t, "mrn://catalog/athena/awsdatacatalog", mrnOf(t, result, "AwsDataCatalog"))
	assert.Equal(t, "mrn://catalog/athena/hive_lab", mrnOf(t, result, "hive_lab"))
	assert.Equal(t, "HIVE", findAsset(t, result, "hive_lab").Metadata["type"])
}

func TestE2E_ContainmentLineage(t *testing.T) {
	result := discover(t)

	assertEdge(t, result, "mrn://catalog/athena/awsdatacatalog", "mrn://database/glue/shop", "CONTAINS")
	assertEdge(t, result, "mrn://database/glue/shop", "mrn://table/glue/orders", "CONTAINS")
	assertEdge(t, result, "mrn://workgroup/athena/analytics", "mrn://data-model-object/athena/analytics-daily-revenue", "CONTAINS")
}

func TestE2E_BucketFeedsTheTableItStores(t *testing.T) {
	result := discover(t)

	assertEdge(t, result, "mrn://bucket/s3/marmot-lake", "mrn://table/glue/orders", "FEEDS")
}

func TestE2E_WorkGroupProducesItsResultBucket(t *testing.T) {
	result := discover(t)

	assertEdge(t, result, "mrn://workgroup/athena/analytics", "mrn://bucket/s3/marmot-results", "PRODUCES")
}

func TestE2E_TablesASavedQueryReadsFeedIt(t *testing.T) {
	result := discover(t)
	target := "mrn://data-model-object/athena/analytics-daily-revenue"

	assertEdge(t, result, "mrn://table/glue/orders", target, "FEEDS")
	assertEdge(t, result, "mrn://table/glue/customers", target, "FEEDS")
}

func TestE2E_TagsToMetadataReadsWorkGroupTags(t *testing.T) {
	url := endpoint(t)
	seed(t, url)

	config := pluginConfig(url)
	config["tags_to_metadata"] = true

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.Equal(t, "analytics", findAsset(t, result, "analytics").Metadata["tag_team"])
}

func TestE2E_ExcludedDatabaseIsSkipped(t *testing.T) {
	url := endpoint(t)
	seed(t, url)

	config := pluginConfig(url)
	config["exclude_databases"] = []any{"shop"}

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.Nil(t, lookupAsset(result, "shop"))
	assert.Nil(t, lookupAsset(result, "orders"))
	assert.NotNil(t, lookupAsset(result, "analytics"))
}

// helpers

func lookupAsset(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	a := lookupAsset(result, name)
	require.NotNil(t, a, "no asset named %s", name)
	return *a
}

func mrnOf(t *testing.T, result *pluginsdk.DiscoveryResult, name string) string {
	t.Helper()

	a := findAsset(t, result, name)
	require.NotNil(t, a.MRN)
	return *a.MRN
}

func athenaFacts(t *testing.T, a pluginsdk.Asset) map[string]any {
	t.Helper()

	facts, ok := a.Metadata["athena"].(map[string]any)
	require.True(t, ok, "asset %s has no athena metadata section", *a.Name)
	return facts
}

// e2eColumn is the column shape as it arrives back over the wire.
type e2eColumn struct {
	ColumnName     string `json:"column_name"`
	DataType       string `json:"data_type"`
	IsNullable     bool   `json:"is_nullable"`
	IsPartitionKey bool   `json:"is_partition_key"`
	Description    string `json:"description"`
}

func columns(t *testing.T, a pluginsdk.Asset) []e2eColumn {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "asset %s has no columns", *a.Name)

	var parsed []e2eColumn
	require.NoError(t, json.Unmarshal([]byte(raw), &parsed))
	return parsed
}

func assertEdge(t *testing.T, result *pluginsdk.DiscoveryResult, source, target, edgeType string) {
	t.Helper()

	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return
		}
	}
	t.Errorf("no %s edge from %s to %s in %v", edgeType, source, target, result.Lineage)
}
