package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/lineage"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresIngestion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	containerStatus, err := env.ContainerManager.ExecCommand("postgres-test-plugin", []string{
		"pg_isready", "-U", "postgres",
	})
	if err != nil || !strings.Contains(containerStatus, "accepting connections") {
		err = env.EnsurePostgresStarted(ctx)
		require.NoError(t, err, "Failed to start PostgreSQL container")

		for i := 0; i < 30; i++ {
			readyStatus, readyErr := env.ContainerManager.ExecCommand("postgres-test-plugin", []string{
				"pg_isready", "-U", "postgres",
			})
			if readyErr == nil && strings.Contains(readyStatus, "accepting connections") {
				break
			}
			if i == 29 {
				t.Fatalf("PostgreSQL failed to become ready: %v", readyErr)
			}
			time.Sleep(1 * time.Second)
		}
	}

	_, err = executePostgresCommand("DROP DATABASE IF EXISTS test_db WITH (FORCE);")
	require.NoError(t, err, "Failed to drop database")

	_, err = executePostgresCommand("CREATE DATABASE test_db;")
	require.NoError(t, err, "Failed to create database")

	dbExists, err := executePostgresCommand("SELECT 1 FROM pg_database WHERE datname='test_db';")
	require.NoError(t, err, "Failed to check if database exists")
	require.Contains(t, dbExists, "1", "Database test_db was not created properly")

	setupErr := setupTestDatabase(t)
	require.NoError(t, setupErr, "Failed to set up test database")

	configContent := `
runs:
- postgresql:
  host: "postgres-test-plugin"
  port: 5432
  database: "postgres"
  user: "postgres"
  password: "postgres"
  ssl_mode: "disable"
  discover_foreign_keys: true
  include_row_counts: false
  include_columns: true
  exclude_system_schemas: true
  database_filter:
    include:
      - "test_db"
    exclude:
      - "template.*"
      - "postgres"
  schema_filter:
    include:
      - "^public$"
      - "^app_.*"
    exclude:
      - "^pg_.*"
  tags:
    - "postgres"
    - "test"
    - "database"
`

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080", "--timeout", "60s", "-v"},
		configContent,
	)
	require.NoError(t, err, "Ingest command failed")

	waitStart := time.Now()
	maxWait := 30 * time.Second
	var resp *assets.GetAssetsListOK

	for {
		if time.Since(waitStart) > maxWait {
			t.Fatalf("Timed out waiting for assets after %v", maxWait)
		}

		params := assets.NewGetAssetsListParams().WithTimeout(10 * time.Second)
		tmpResp, err := env.APIClient.Assets.GetAssetsList(params)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		found := false
		for _, a := range tmpResp.Payload.Assets {
			if a.Name == "test_db" {
				found = true
				break
			}
		}

		if found {
			resp = tmpResp
			break
		}

		time.Sleep(2 * time.Second)
	}

	require.NotNil(t, resp, "Failed to get assets list")

	t.Logf("Total assets found: %d", len(resp.Payload.Assets))
	for i, a := range resp.Payload.Assets {
		t.Logf("Asset %d: ID=%s, Name=%s, Type=%s", i, a.ID, a.Name, a.Type)
	}

	database := utils.FindAssetByName(resp.Payload.Assets, "test_db")
	require.NotNil(t, database, "asset test_db not found")

	assert.Equal(t, "Database", database.Type)
	assert.Contains(t, database.Providers, "PostgreSQL")
	assert.Contains(t, database.Tags, "postgres")
	assert.Contains(t, database.Tags, "test")
	assert.Contains(t, database.Tags, "database")

	usersTable := utils.FindAssetByName(resp.Payload.Assets, "users")
	if usersTable == nil {
		for _, a := range resp.Payload.Assets {
			if a.Type == "Table" {
				meta, ok := a.Metadata.(map[string]interface{})
				if ok && meta["table_name"] == "users" {
					usersTable = a
					break
				}
			}
		}
	}

	require.NotNil(t, usersTable, "asset users not found")
	assert.Equal(t, "Table", usersTable.Type)
	assert.Contains(t, usersTable.Providers, "PostgreSQL")
	assert.Contains(t, usersTable.Tags, "postgres")
	assert.Contains(t, usersTable.Tags, "test")

	metadata := usersTable.Metadata.(map[string]interface{})
	schema, ok := metadata["schema"].(string)
	assert.True(t, ok, "schema not found or not a string in metadata")
	assert.Equal(t, "public", schema, "expected 'public' schema")

	assert.Equal(t, "users", metadata["table_name"])
	assert.Equal(t, "postgres", metadata["owner"])

	columns, ok := metadata["columns"].([]interface{})
	require.True(t, ok, "columns not found in metadata")
	assert.GreaterOrEqual(t, len(columns), 3, "expected at least 3 columns")

	var idColumn map[string]interface{}
	for _, col := range columns {
		colMap, ok := col.(map[string]interface{})
		require.True(t, ok, "column is not a map")

		colName, ok := colMap["column_name"].(string)
		require.True(t, ok, "column_name not found or not a string")

		if colName == "id" {
			idColumn = colMap
			break
		}
	}

	require.NotNil(t, idColumn, "id column not found")
	assert.Equal(t, "integer", idColumn["data_type"])
	assert.Equal(t, true, idColumn["is_primary_key"])

	ordersTable := utils.FindAssetByName(resp.Payload.Assets, "orders")
	if ordersTable == nil {
		for _, a := range resp.Payload.Assets {
			if a.Type == "Table" {
				meta, ok := a.Metadata.(map[string]interface{})
				if ok && meta["table_name"] == "orders" {
					ordersTable = a
					break
				}
			}
		}
	}
	require.NotNil(t, ordersTable, "asset orders not found")

	orderUUID := strfmt.UUID(ordersTable.ID)
	lineageParams := lineage.NewGetLineageAssetsIDParams().WithID(orderUUID)
	lineageParams.WithTimeout(30 * time.Second)
	lineageResp, err := env.APIClient.Lineage.GetLineageAssetsID(lineageParams)
	require.NoError(t, err, "failed to get lineage for orders table")

	t.Logf("Lineage edges for orders table: %d", len(lineageResp.Payload.Edges))

	var fkFound bool
	for _, edge := range lineageResp.Payload.Edges {
		t.Logf("Edge: Source=%s, Target=%s, Type=%s", edge.Source, edge.Target, edge.Type)

		if edge.Type == "FOREIGN_KEY" && strings.Contains(edge.Target, usersTable.ID) {
			fkFound = true
			break
		}
	}

	assert.True(t, fkFound, "foreign key relationship from orders to users not found")

	deleteParams := assets.NewDeleteAssetsIDParams().WithID(database.ID)
	deleteParams.WithTimeout(10 * time.Second)
	_, err = env.APIClient.Assets.DeleteAssetsID(deleteParams)
	if err != nil {
		t.Logf("Warning: failed to delete asset %s: %v", database.ID, err)
	}

	deleteParams = assets.NewDeleteAssetsIDParams().WithID(usersTable.ID)
	deleteParams.WithTimeout(10 * time.Second)
	_, err = env.APIClient.Assets.DeleteAssetsID(deleteParams)
	if err != nil {
		t.Logf("Warning: failed to delete asset %s: %v", usersTable.ID, err)
	}

	deleteParams = assets.NewDeleteAssetsIDParams().WithID(ordersTable.ID)
	deleteParams.WithTimeout(10 * time.Second)
	_, err = env.APIClient.Assets.DeleteAssetsID(deleteParams)
	if err != nil {
		t.Logf("Warning: failed to delete asset %s: %v", ordersTable.ID, err)
	}
}

func setupTestDatabase(t *testing.T) error {
	tables := []string{
		`CREATE TABLE public.users (
        id SERIAL PRIMARY KEY,
        username VARCHAR(100) NOT NULL,
        email VARCHAR(255) NOT NULL UNIQUE,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`,
		`CREATE TABLE public.orders (
        id SERIAL PRIMARY KEY,
        user_id INTEGER NOT NULL,
        amount DECIMAL(10, 2) NOT NULL,
        status VARCHAR(50) DEFAULT 'pending',
        order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES public.users(id)
    );`,
		`CREATE SCHEMA IF NOT EXISTS app_schema;`,
	}

	for _, cmd := range tables {
		_, err := executePostgresCommand(cmd, "test_db")
		if err != nil {
			return fmt.Errorf("failed to execute SQL: %s, error: %w", cmd, err)
		}
	}

	data := []string{
		"INSERT INTO public.users (username, email) VALUES ('user1', 'user1@example.com'), ('user2', 'user2@example.com');",
		"INSERT INTO public.orders (user_id, amount, status) VALUES (1, 99.99, 'completed'), (1, 149.99, 'processing'), (2, 25.50, 'pending');",
		"COMMENT ON TABLE public.users IS 'User account information';",
	}

	for _, cmd := range data {
		_, err := executePostgresCommand(cmd, "test_db")
		if err != nil {
			return fmt.Errorf("failed to insert data: %s, error: %w", cmd, err)
		}
	}

	tableCheck, err := executePostgresCommand("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('users', 'orders');", "test_db")
	if err != nil {
		return fmt.Errorf("failed to verify tables: %w", err)
	}

	if !strings.Contains(tableCheck, "2") {
		return fmt.Errorf("tables not created properly, found: %s", tableCheck)
	}

	return nil
}

// executePostgresCommand executes a SQL command on a specified database
func executePostgresCommand(cmd string, dbName ...string) (string, error) {
	_, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := []string{"psql", "-U", "postgres"}

	// if dbName is empty, executes on the default postgres database
	if len(dbName) > 0 && dbName[0] != "" {
		args = append(args, "-d", dbName[0], "-v", "ON_ERROR_STOP=1")
	}

	args = append(args, "-c", cmd)

	output, err := env.ContainerManager.ExecCommand("postgres-test-plugin", args)

	if err != nil {
		return output, fmt.Errorf("command error: %w, output: %s", err, output)
	}

	return output, nil
}
