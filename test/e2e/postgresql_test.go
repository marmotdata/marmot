package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/lineage"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresIngestion(t *testing.T) {
	ctx := context.Background()

	err := env.EnsurePostgresStarted(ctx)
	require.NoError(t, err)

	err = setupTestDatabase(t)
	require.NoError(t, err)

	configContent := `
runs:
  - postgresql:
      host: "postgres-test-plugin"
      user: "postgres"
      password: "postgres"
      ssl_mode: "disable"
      discover_foreign_keys: true
      tags:
        - "postgres"
        - "test"
`

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
	)
	require.NoError(t, err)

	time.Sleep(10 * time.Second)

	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	t.Logf("Total assets found: %d", len(resp.Payload.Assets))

	var usersTable, ordersTable *models.AssetAsset
	for _, a := range resp.Payload.Assets {
		if meta, ok := a.Metadata.(map[string]interface{}); ok {
			if tableName, ok := meta["table_name"].(string); ok {
				if tableName == "users" {
					usersTable = a
				} else if tableName == "orders" {
					ordersTable = a
				}
			}
		}
	}

	require.NotNil(t, usersTable, "users table not found")
	require.NotNil(t, ordersTable, "orders table not found")

	orderUUID := strfmt.UUID(ordersTable.ID)
	lineageParams := lineage.NewGetLineageAssetsIDParams().WithID(orderUUID)
	lineageResp, err := env.APIClient.Lineage.GetLineageAssetsID(lineageParams)
	require.NoError(t, err)

	var lineageFound bool
	for _, edge := range lineageResp.Payload.Edges {
		if edge.Type == "DEFAULT" &&
			((strings.Contains(edge.Source, "postgresql/orders") && strings.Contains(edge.Target, "postgresql/users")) ||
				(strings.Contains(edge.Source, "postgresql/users") && strings.Contains(edge.Target, "postgresql/orders"))) {
			lineageFound = true
			break
		}
	}

	assert.True(t, lineageFound, "relationship between orders and users not found")
}

func setupTestDatabase(t *testing.T) error {
	createCmd := "CREATE DATABASE test_db;"
	_, err := executePostgresCommand(createCmd)
	if err != nil {
		return err
	}

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
			return err
		}
	}

	data := []string{
		"INSERT INTO public.users (username, email) VALUES ('user1', 'user1@example.com'), ('user2', 'user2@example.com');",
		"INSERT INTO public.orders (user_id, amount, status) VALUES (1, 99.99, 'completed'), (1, 149.99, 'processing'), (2, 25.50, 'pending');",
		"COMMENT ON TABLE public.users IS 'User account information';",
		"ANALYZE;",
	}

	for _, cmd := range data {
		_, err := executePostgresCommand(cmd, "test_db")
		if err != nil {
			return err
		}
	}

	return nil
}

// executePostgresCommand executes a SQL command on a specified database
func executePostgresCommand(cmd string, dbName ...string) (string, error) {
	args := []string{"psql", "-U", "postgres"}

	if len(dbName) > 0 && dbName[0] != "" {
		args = append(args, "-d", dbName[0], "-v", "ON_ERROR_STOP=1")
	}

	args = append(args, "-c", cmd)

	return env.ContainerManager.ExecCommand("postgres-test-plugin", args)
}
