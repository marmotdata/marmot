package e2e

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"testing"
// 	"time"
//
// 	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
// 	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

//TODO: fix this test
// 2025/03/14 20:59:23 Container 75e1bf9969a087f7b6b95ea1a35c3045db4a9301e4b29a90f4cb417ebee78213 cleaned up successfully
//     iceberg_test.go:124: Waiting for ingestion to complete and assets to be indexed...
//     iceberg_test.go:131: Found 0 assets
//     iceberg_test.go:138:
//         	Error Trace:	/home/charlie/Code/marmot/test/e2e/iceberg_test.go:138
//         	Error:      	Expected value not to be nil.
//         	Test:       	TestIcebergRESTIngestion
//         	Messages:   	asset test-table-basic not found
// --- FAIL: TestIcebergRESTIngestion (31.35s)

// TestIcebergRESTIngestion tests the ingestion of Iceberg tables using the REST catalog
// func TestIcebergRESTIngestion(t *testing.T) {
// 	// Ensure Iceberg REST server is started
// 	err := env.EnsureIcebergRESTStarted(context.Background())
// 	require.NoError(t, err)
//
// 	// Verify REST server connectivity before creating tables
// 	client := &http.Client{Timeout: 5 * time.Second}
// 	namespaceURL := fmt.Sprintf("http://localhost:%s/v1/namespaces", env.IcebergRESTPort)
// 	resp, err := client.Get(namespaceURL)
// 	require.NoError(t, err, "Failed to connect to REST server")
// 	defer resp.Body.Close()
//
// 	body, _ := io.ReadAll(resp.Body)
// 	t.Logf("Namespaces response: %s", string(body))
//
// 	// Create test tables in REST catalog
// 	testTables := []utils.TestIcebergTable{
// 		{
// 			Name:      "test-table-basic",
// 			Namespace: "default",
// 			Fields: []utils.TestIcebergField{
// 				{ID: 1, Name: "id", Type: "int", Required: true},
// 				{ID: 2, Name: "data", Type: "string", Required: false},
// 				{ID: 3, Name: "timestamp", Type: "timestamp", Required: false},
// 				{ID: 4, Name: "amount", Type: "double", Required: false},
// 				{ID: 5, Name: "active", Type: "boolean", Required: true},
// 			},
// 			Tags: map[string]string{
// 				"Environment": "prod",
// 				"Team":        "platform",
// 				"CostCenter":  "data-eng-101",
// 				"SLA":         "tier-1",
// 			},
// 		},
// 		{
// 			Name:        "test-table-partitioned",
// 			Namespace:   "default",
// 			Partitioned: true,
// 			Fields: []utils.TestIcebergField{
// 				{ID: 1, Name: "id", Type: "int", Required: true},
// 				{ID: 2, Name: "data", Type: "string", Required: false},
// 				{ID: 3, Name: "category", Type: "string", Required: false},
// 				{ID: 4, Name: "created_at", Type: "timestamp", Required: true},
// 				{ID: 5, Name: "updated_at", Type: "timestamp", Required: false},
// 				{ID: 6, Name: "processing_date", Type: "date", Required: true},
// 				{ID: 7, Name: "amount", Type: "decimal(10,2)", Required: false},
// 				{ID: 8, Name: "metadata", Type: "map<string, string>", Required: false},
// 			},
// 			Tags: map[string]string{
// 				"Environment": "staging",
// 				"Team":        "orders",
// 				"CostCenter":  "finance-42",
// 				"DataSource":  "order-processing",
// 				"Retention":   "3-years",
// 			},
// 		},
// 	}
//
// 	err = utils.CreateTestIcebergTablesInREST(context.Background(), fmt.Sprintf("localhost:%s", env.IcebergRESTPort), testTables)
// 	require.NoError(t, err, "Failed to create test tables in REST catalog")
//
// 	// Verify tables were created
// 	tablesURL := fmt.Sprintf("http://localhost:%s/v1/namespaces/default/tables", env.IcebergRESTPort)
// 	tresp, err := client.Get(tablesURL)
// 	require.NoError(t, err, "Failed to get tables from REST server")
// 	defer tresp.Body.Close()
//
// 	tbody, _ := io.ReadAll(tresp.Body)
// 	t.Logf("Tables response: %s", string(tbody))
//
// 	// Create test config file with REST container name (rest-test) for Docker network access
// 	configContent := fmt.Sprintf(`
// runs:
//   - iceberg:
//       catalog_type: "rest"
//       rest:
//         uri: "http://rest-test:%s"
//         auth:
//           type: "none"
//       include_schema_info: true
//       include_partition_info: true
//       include_snapshot_info: true
//       include_properties: true
//       include_statistics: true
//       tags:
//         - "iceberg"
//         - "test"
//         - "rest-catalog"
//         - "env-prod"
//         - "team-platform"
//       table_filter:
//         include:
//           - ".*"
//         exclude:
//           - "^_.*"
// `, env.IcebergRESTPort)
//
// 	// Run ingest command with verbosity
// 	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
// 		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080", "-v", "--debug"},
// 		configContent,
// 	)
// 	require.NoError(t, err, "Ingest command failed")
//
// 	// Allow ample time for ingestion to complete
// 	t.Log("Waiting for ingestion to complete and assets to be indexed...")
// 	time.Sleep(30 * time.Second)
//
// 	// Fetch all assets
// 	params := assets.NewGetAssetsListParams()
// 	assetsResp, err := env.APIClient.Assets.GetAssetsList(params)
// 	require.NoError(t, err)
// 	t.Logf("Found %d assets", len(assetsResp.Payload.Assets))
// 	for i, asset := range assetsResp.Payload.Assets {
// 		t.Logf("Asset %d: ID=%s, Name=%s, Type=%s", i, asset.ID, asset.Name, asset.Type)
// 	}
//
// 	// Verify first table
// 	table1 := utils.FindAssetByName(assetsResp.Payload.Assets, "test-table-basic")
// 	require.NotNil(t, table1, "asset test-table-basic not found")
//
// 	// Now check for specific fields
// 	assert.Equal(t, "Table", table1.Type)
// 	assert.Contains(t, table1.Providers, "Iceberg")
// 	assert.Contains(t, table1.Tags, "iceberg")
// 	assert.Contains(t, table1.Tags, "test")
// 	assert.Contains(t, table1.Tags, "rest-catalog")
// 	assert.Contains(t, table1.Tags, "env-prod")
// 	assert.Contains(t, table1.Tags, "team-platform")
//
// 	// Verify metadata
// 	metadata1 := table1.Metadata.(map[string]interface{})
// 	t.Logf("Table metadata: %+v", metadata1)
//
// 	// Check basic table metadata
// 	assert.Equal(t, "test-table-basic", metadata1["table_name"])
// 	assert.Equal(t, "default", metadata1["namespace"])
// 	assert.Equal(t, "default.test-table-basic", metadata1["identifier"])
// 	assert.Equal(t, "rest", metadata1["catalog_type"])
//
// 	// Check format version
// 	formatVersion, ok := metadata1["format_version"].(json.Number)
// 	require.True(t, ok, "format_version not found in metadata or not a number")
// 	assert.Equal(t, "2", formatVersion.String())
//
// 	// Check schema info
// 	schemaJSON, ok := metadata1["schema_json"].(string)
// 	require.True(t, ok, "schema_json not found in metadata")
// 	assert.Contains(t, schemaJSON, "\"type\":\"struct\"")
// 	assert.Contains(t, schemaJSON, "\"fields\":")
//
// 	// Verify second table with partitioning
// 	table2 := utils.FindAssetByName(assetsResp.Payload.Assets, "test-table-partitioned")
// 	require.NotNil(t, table2, "asset test-table-partitioned not found")
//
// 	metadata2 := table2.Metadata.(map[string]interface{})
//
// 	// Check partition info
// 	partitionSpec, ok := metadata2["partition_spec"].(string)
// 	require.True(t, ok, "partition_spec not found in metadata")
// 	assert.Contains(t, partitionSpec, "\"transform\":")
//
// 	// Clean up assets
// 	if table1 != nil {
// 		_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(table1.ID))
// 		assert.NoError(t, err, "failed to delete asset", table1.ID)
// 	}
//
// 	if table2 != nil {
// 		_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(table2.ID))
// 		assert.NoError(t, err, "failed to delete asset", table2.ID)
// 	}
// }
