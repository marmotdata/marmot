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
	ctx := context.Background()

	// Ensure PostgreSQL is started
	err := env.EnsurePostgresStarted(ctx)
	require.NoError(t, err)

	// Create test database and tables using the shared PostgreSQL instance
	setupTestDatabase(t)

	// Create test config file
	configContent := `
runs:
  - postgresql:
      host: "postgres-test-plugin"
      database: "test_db"
      user: "postgres"
      password: "postgres"
      ssl_mode: "disable"
      discover_foreign_keys: true
      tags:
        - "postgres"
        - "test"
        - "database"
      schema_filter:
        include:
          - "^public$"
          - "^app_.*"
        exclude:
          - "^pg_.*"
`

	// Run ingest command
	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
	)
	require.NoError(t, err)

	// Allow time for ingestion
	time.Sleep(5 * time.Second)

	// Fetch all assets
	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	t.Logf("Total assets found: %d", len(resp.Payload.Assets))
	for i, a := range resp.Payload.Assets {
		t.Logf("Asset %d: ID=%s, Name=%s, Type=%s", i, a.ID, a.Name, a.Type)
	}

	// Verify database
	database := utils.FindAssetByName(resp.Payload.Assets, "test_db")
	require.NotNil(t, database, "asset test_db not found")

	assert.Equal(t, "Database", database.Type)
	assert.Contains(t, database.Providers, "PostgreSQL")
	assert.Contains(t, database.Tags, "postgres")
	assert.Contains(t, database.Tags, "test")
	assert.Contains(t, database.Tags, "database")

	// Verify users table
	usersTable := utils.FindAssetByName(resp.Payload.Assets, "users")
	require.NotNil(t, usersTable, "asset users not found")

	assert.Equal(t, "Table", usersTable.Type)
	assert.Contains(t, usersTable.Providers, "PostgreSQL")
	assert.Contains(t, usersTable.Tags, "postgres")
	assert.Contains(t, usersTable.Tags, "test")

	// Verify table metadata
	metadata := usersTable.Metadata.(map[string]interface{})
	t.Logf("Table metadata: %+v", metadata)

	schema, ok := metadata["schema"].(string)
	assert.True(t, ok, "schema not found or not a string in metadata")
	assert.Equal(t, "public", schema, "expected 'public' schema")

	assert.Equal(t, "users", metadata["table_name"])
	assert.Equal(t, "postgres", metadata["owner"])

	// Verify columns
	columns, ok := metadata["columns"].([]interface{})
	require.True(t, ok, "columns not found in metadata")
	assert.GreaterOrEqual(t, len(columns), 3, "expected at least 3 columns")

	// Find id column
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

	// Verify orders table
	ordersTable := utils.FindAssetByName(resp.Payload.Assets, "orders")
	require.NotNil(t, ordersTable, "asset orders not found")

	// Check lineage between orders and users table
	orderUUID := strfmt.UUID(ordersTable.ID)
	lineageParams := lineage.NewGetLineageAssetsIDParams().WithID(orderUUID)
	lineageResp, err := env.APIClient.Lineage.GetLineageAssetsID(lineageParams)
	require.NoError(t, err, "failed to get lineage for orders table")

	t.Logf("Lineage edges for orders table: %d", len(lineageResp.Payload.Edges))

	// Find the foreign key relationship to users table
	var fkFound bool
	for _, edge := range lineageResp.Payload.Edges {
		t.Logf("Edge: Source=%s, Target=%s, Type=%s", edge.Source, edge.Target, edge.Type)

		// Check for FOREIGN_KEY relation to users table
		if edge.Type == "FOREIGN_KEY" && strings.Contains(edge.Target, usersTable.ID) {
			fkFound = true
			break
		}
	}

	assert.True(t, fkFound, "foreign key relationship from orders to users not found")

	// Clean up assets
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(database.ID))
	assert.NoError(t, err, "failed to delete asset", database.ID)
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(usersTable.ID))
	assert.NoError(t, err, "failed to delete asset", usersTable.ID)
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(ordersTable.ID))
	assert.NoError(t, err, "failed to delete asset", ordersTable.ID)
}

// setupTestDatabase creates test databases and tables for PostgreSQL testing
func setupTestDatabase(t *testing.T) {
	// Create test_db database
	err := executePostgresCommand("DROP DATABASE IF EXISTS test_db;")
	require.NoError(t, err, "Failed to drop database")

	err = executePostgresCommand("CREATE DATABASE test_db;")
	require.NoError(t, err, "Failed to create database")

	// Create users table first
	userTableSQL := `CREATE TABLE public.users (
        id SERIAL PRIMARY KEY,
        username VARCHAR(100) NOT NULL,
        email VARCHAR(255) NOT NULL UNIQUE,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`

	err = executePostgresCommandOnDB("test_db", userTableSQL)
	require.NoError(t, err, "Failed to create users table")
	// Create orders table (with foreign key to users)
	ordersTableSQL := `CREATE TABLE public.orders (
        id SERIAL PRIMARY KEY,
        user_id INTEGER NOT NULL,
        amount DECIMAL(10, 2) NOT NULL,
        status VARCHAR(50) DEFAULT 'pending',
        order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES public.users(id)
    );`

	err = executePostgresCommandOnDB("test_db", ordersTableSQL)
	require.NoError(t, err, "Failed to create orders table")

	// Create app_schema
	err = executePostgresCommandOnDB("test_db", "CREATE SCHEMA IF NOT EXISTS app_schema;")
	require.NoError(t, err, "Failed to create app_schema")

	// Insert test data (only proceed if earlier steps succeeded)
	testData := []string{
		"INSERT INTO public.users (username, email) VALUES ('user1', 'user1@example.com'), ('user2', 'user2@example.com');",
		"INSERT INTO public.orders (user_id, amount, status) VALUES (1, 99.99, 'completed'), (1, 149.99, 'processing'), (2, 25.50, 'pending');",
		"COMMENT ON TABLE public.users IS 'User account information';",
		"ANALYZE;",
	}

	for _, cmd := range testData {
		err = executePostgresCommandOnDB("test_db", cmd)
		require.NoError(t, err, "Failed to execute: %s", cmd)
	}
}

func executePostgresCommandOnDB(dbName string, cmd string) error {
	// Execute the command directly specifying the database
	output, err := env.ContainerManager.ExecCommand("postgres-test-plugin", []string{
		"psql",
		"-U", "postgres",
		"-d", dbName,
		"-v", "ON_ERROR_STOP=1",
		"-c", cmd,
	})

	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}

	return nil
}

// executePostgresCommand executes a SQL command in the PostgreSQL container
func executePostgresCommand(cmd string) error {
	// Escape single quotes for psql if needed
	escapedCmd := strings.ReplaceAll(cmd, "'", "\\'")

	// Execute the command directly using the ExecCommand method
	_, err := env.ContainerManager.ExecCommand("postgres-test-plugin", []string{
		"psql", "-U", "postgres", "-c", escapedCmd,
	})

	return err
}
