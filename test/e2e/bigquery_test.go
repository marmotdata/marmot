// This E2E test needs fixing

package e2e

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"os"
// 	"strings"
// 	"testing"
// 	"time"

// 	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
// 	"github.com/marmotdata/marmot/tests/e2e/internal/client/models"
// 	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// func TestBigQueryIngestion(t *testing.T) {
// 	ctx := context.Background()

// 	err := env.EnsureBigQueryStarted(ctx)
// 	require.NoError(t, err)

// 	emulatorHost := fmt.Sprintf("localhost:%s", env.BigQueryEmulatorPort)
// 	os.Setenv("BIGQUERY_EMULATOR_HOST", emulatorHost)
// 	defer os.Unsetenv("BIGQUERY_EMULATOR_HOST")

// 	err = setupBigQueryTestData(t, emulatorHost)
// 	require.NoError(t, err)

// 	configContent := `
// runs:
//   - bigquery:
//       project_id: "test-project"
//       include_views: true
//       include_tables: true
//       include_external_tables: true
//       include_columns: true
//       dataset_filter:
//         include:
//           - "^production_analytics$"
//           - "^staging_data$"
//         exclude:
//           - "^temp_.*"
//       table_filter:
//         include:
//           - ".*"
//         exclude:
//           - "^temp_.*"
//       tags:
//         - "bigquery"
//         - "test"
// `

// 	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
// 		[]string{fmt.Sprintf("BIGQUERY_EMULATOR_HOST=bigquery-emulator-test:%s marmot ingest -c /tmp/config.yaml -k %s -H http://marmot-test:8080", env.BigQueryEmulatorPort, env.APIKey)},
// 		configContent,
// 		[]string{"sh", "-c"},
// 	)
// 	require.NoError(t, err)

// 	params := assets.NewGetAssetsListParams()
// 	resp, err := env.APIClient.Assets.GetAssetsList(params)
// 	require.NoError(t, err)

// 	t.Logf("Total assets found: %d", len(resp.Payload.Assets))
// 	for i, asset := range resp.Payload.Assets {
// 		t.Logf("Asset %d: Name=%s, Type=%s, Providers=%v", i, asset.Name, asset.Type, asset.Providers)
// 	}

// 	productionDataset := utils.FindAssetByName(resp.Payload.Assets, "production_analytics")
// 	require.NotNil(t, productionDataset, "production_analytics dataset not found")
// 	assert.Equal(t, "Dataset", productionDataset.Type)
// 	assert.Contains(t, productionDataset.Providers, "BigQuery")

// 	stagingDataset := utils.FindAssetByName(resp.Payload.Assets, "staging_data")
// 	require.NotNil(t, stagingDataset, "staging_data dataset not found")

// 	userEventsTable := utils.FindAssetByName(resp.Payload.Assets, "user_events")
// 	require.NotNil(t, userEventsTable, "user_events table not found")
// 	assert.Equal(t, "Table", userEventsTable.Type)

// 	transactionsTable := utils.FindAssetByName(resp.Payload.Assets, "transactions")
// 	require.NotNil(t, transactionsTable, "transactions table not found")

// 	processedDataTable := utils.FindAssetByName(resp.Payload.Assets, "processed_data")
// 	require.NotNil(t, processedDataTable, "processed_data table not found")

// 	userSummaryView := utils.FindAssetByName(resp.Payload.Assets, "user_summary")
// 	require.NotNil(t, userSummaryView, "user_summary view not found")
// 	assert.Equal(t, "View", userSummaryView.Type)

// 	rawLogsExternal := utils.FindAssetByName(resp.Payload.Assets, "raw_logs")
// 	require.NotNil(t, rawLogsExternal, "raw_logs external table not found")

// 	userEventsMetadata := userEventsTable.Metadata.(map[string]interface{})
// 	assert.Equal(t, "test-project", userEventsMetadata["project_id"])
// 	assert.Equal(t, "production_analytics", userEventsMetadata["dataset_id"])
// 	assert.Equal(t, "user_events", userEventsMetadata["table_name"])

// 	columns, ok := userEventsMetadata["columns"]
// 	require.True(t, ok, "columns not found in metadata")
// 	columnList := columns.([]interface{})
// 	assert.GreaterOrEqual(t, len(columnList), 4, "expected at least 4 columns")

// 	tempDataset := utils.FindAssetByName(resp.Payload.Assets, "temp_test")
// 	assert.Nil(t, tempDataset, "temp_test dataset should be filtered out")

// 	for _, asset := range []*models.AssetAsset{productionDataset, stagingDataset, userEventsTable, transactionsTable, processedDataTable, userSummaryView, rawLogsExternal} {
// 		if asset != nil {
// 			_, err := env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(asset.ID))
// 			assert.NoError(t, err, "failed to delete asset", asset.Name)
// 		}
// 	}
// }

// func setupBigQueryTestData(t *testing.T, emulatorHost string) error {
// 	baseURL := fmt.Sprintf("http://%s/bigquery/v2/projects/test-project", emulatorHost)

// 	// Test emulator connectivity first
// 	testResp, err := http.Get(fmt.Sprintf("http://%s", emulatorHost))
// 	if err != nil {
// 		return fmt.Errorf("emulator not reachable: %w", err)
// 	}
// 	testResp.Body.Close()
// 	t.Logf("BigQuery emulator is reachable")

// 	datasets := []string{"production_analytics", "staging_data", "temp_test"}
// 	for _, dataset := range datasets {
// 		t.Logf("Creating dataset: %s", dataset)
// 		payload := map[string]interface{}{
// 			"datasetReference": map[string]string{
// 				"datasetId": dataset,
// 				"projectId": "test-project",
// 			},
// 			"location": "US",
// 		}
// 		err := makeRestAPICall("POST", fmt.Sprintf("%s/datasets", baseURL), payload)
// 		if err != nil {
// 			return fmt.Errorf("failed to create dataset %s: %w", dataset, err)
// 		}
// 		t.Logf("Created dataset: %s", dataset)
// 	}

// 	time.Sleep(2 * time.Second)

// 	tables := []struct {
// 		dataset string
// 		table   string
// 		fields  []map[string]string
// 	}{
// 		{
// 			dataset: "production_analytics",
// 			table:   "user_events",
// 			fields: []map[string]string{
// 				{"name": "event_id", "type": "STRING", "mode": "REQUIRED"},
// 				{"name": "user_id", "type": "STRING", "mode": "REQUIRED"},
// 				{"name": "event_type", "type": "STRING", "mode": "REQUIRED"},
// 				{"name": "timestamp", "type": "TIMESTAMP", "mode": "REQUIRED"},
// 			},
// 		},
// 		{
// 			dataset: "production_analytics",
// 			table:   "transactions",
// 			fields: []map[string]string{
// 				{"name": "transaction_id", "type": "STRING", "mode": "REQUIRED"},
// 				{"name": "user_id", "type": "STRING", "mode": "REQUIRED"},
// 				{"name": "amount", "type": "NUMERIC", "mode": "REQUIRED"},
// 				{"name": "currency", "type": "STRING", "mode": "REQUIRED"},
// 			},
// 		},
// 		{
// 			dataset: "staging_data",
// 			table:   "processed_data",
// 			fields: []map[string]string{
// 				{"name": "id", "type": "INTEGER", "mode": "REQUIRED"},
// 				{"name": "name", "type": "STRING", "mode": "NULLABLE"},
// 			},
// 		},
// 	}

// 	for _, table := range tables {
// 		t.Logf("Creating table: %s.%s", table.dataset, table.table)
// 		payload := map[string]interface{}{
// 			"tableReference": map[string]string{
// 				"tableId":   table.table,
// 				"datasetId": table.dataset,
// 				"projectId": "test-project",
// 			},
// 			"schema": map[string]interface{}{
// 				"fields": table.fields,
// 			},
// 		}
// 		err := makeRestAPICall("POST", fmt.Sprintf("%s/datasets/%s/tables", baseURL, table.dataset), payload)
// 		if err != nil {
// 			return fmt.Errorf("failed to create table %s.%s: %w", table.dataset, table.table, err)
// 		}
// 		t.Logf("Created table: %s.%s", table.dataset, table.table)
// 	}

// 	// Create view
// 	t.Logf("Creating view: user_summary")
// 	viewPayload := map[string]interface{}{
// 		"tableReference": map[string]string{
// 			"tableId":   "user_summary",
// 			"datasetId": "production_analytics",
// 			"projectId": "test-project",
// 		},
// 		"view": map[string]interface{}{
// 			"query": "SELECT user_id, COUNT(*) as event_count FROM `test-project.production_analytics.user_events` GROUP BY user_id",
// 		},
// 	}
// 	err = makeRestAPICall("POST", fmt.Sprintf("%s/datasets/production_analytics/tables", baseURL), viewPayload)
// 	if err != nil {
// 		return fmt.Errorf("failed to create view: %w", err)
// 	}
// 	t.Logf("Created view: user_summary")

// 	// Create external table
// 	t.Logf("Creating external table: raw_logs")
// 	externalTablePayload := map[string]interface{}{
// 		"tableReference": map[string]string{
// 			"tableId":   "raw_logs",
// 			"datasetId": "staging_data",
// 			"projectId": "test-project",
// 		},
// 		"externalDataConfiguration": map[string]interface{}{
// 			"sourceFormat": "CSV",
// 			"sourceUris":   []string{"gs://example-bucket/logs/*.csv"},
// 			"schema": map[string]interface{}{
// 				"fields": []map[string]string{
// 					{"name": "timestamp", "type": "TIMESTAMP"},
// 					{"name": "message", "type": "STRING"},
// 					{"name": "level", "type": "STRING"},
// 				},
// 			},
// 		},
// 	}
// 	err = makeRestAPICall("POST", fmt.Sprintf("%s/datasets/staging_data/tables", baseURL), externalTablePayload)
// 	if err != nil {
// 		return fmt.Errorf("failed to create external table: %w", err)
// 	}
// 	t.Logf("Created external table: raw_logs")

// 	// Verify created resources
// 	listResp, err := http.Get(fmt.Sprintf("%s/datasets", baseURL))
// 	if err != nil {
// 		return fmt.Errorf("failed to list datasets: %w", err)
// 	}
// 	defer listResp.Body.Close()
// 	listBody, _ := io.ReadAll(listResp.Body)
// 	t.Logf("Datasets response: %s", string(listBody))

// 	return nil
// }

// func makeRestAPICall(method, url string, payload interface{}) error {
// 	var body io.Reader
// 	if payload != nil {
// 		jsonBytes, err := json.Marshal(payload)
// 		if err != nil {
// 			return fmt.Errorf("marshaling payload: %w", err)
// 		}
// 		body = bytes.NewBuffer(jsonBytes)
// 	}

// 	req, err := http.NewRequest(method, url, body)
// 	if err != nil {
// 		return fmt.Errorf("creating request: %w", err)
// 	}

// 	if payload != nil {
// 		req.Header.Set("Content-Type", "application/json")
// 	}

// 	client := &http.Client{Timeout: 10 * time.Second}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return fmt.Errorf("making request: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
// 		bodyBytes, _ := io.ReadAll(resp.Body)
// 		if resp.StatusCode == 409 || strings.Contains(string(bodyBytes), "already created") || strings.Contains(string(bodyBytes), "already exists") {
// 			return nil
// 		}
// 		return fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
// 	}

// 	return nil
// }
