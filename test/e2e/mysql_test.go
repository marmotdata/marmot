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

func TestMySQLIngestion(t *testing.T) {
	ctx := context.Background()

	err := env.EnsureMySQLStarted(ctx)
	require.NoError(t, err)

	err = setupTestMySQLDatabase(t)
	require.NoError(t, err)

	configContent := `
runs:
  - mysql:
      host: "mysql-test-plugin"
      user: "root"
      password: "mysql"
      database: "test_db"
      tls: "false"
      discover_foreign_keys: true
      tags:
        - "mysql"
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
	for _, a := range resp.Payload.Assets {
		if meta, ok := a.Metadata.(map[string]interface{}); ok {
			t.Logf("Asset: %s, Type: %s, Database: %v, Schema: %v, Table: %v",
				a.Name, a.Type, meta["database"], meta["schema"], meta["table_name"])
		}
	}

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
			((strings.Contains(edge.Source, "mysql/orders") && strings.Contains(edge.Target, "mysql/users")) ||
				(strings.Contains(edge.Source, "mysql/users") && strings.Contains(edge.Target, "mysql/orders"))) {
			lineageFound = true
			break
		}
	}

	assert.True(t, lineageFound, "relationship between orders and users not found")
}

func setupTestMySQLDatabase(t *testing.T) error {
	createCmd := "CREATE DATABASE IF NOT EXISTS test_db;"
	_, err := executeMySQLCommand(createCmd)
	if err != nil {
		return err
	}

	tables := []string{
		`CREATE TABLE test_db.users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(100) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE test_db.orders (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			amount DECIMAL(10, 2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES test_db.users(id)
		);`,
	}

	for _, cmd := range tables {
		_, err := executeMySQLCommand(cmd)
		if err != nil {
			return err
		}
	}

	data := []string{
		"INSERT INTO test_db.users (username, email) VALUES ('user1', 'user1@example.com'), ('user2', 'user2@example.com');",
		"INSERT INTO test_db.orders (user_id, amount, status) VALUES (1, 99.99, 'completed'), (1, 149.99, 'processing'), (2, 25.50, 'pending');",
		"ALTER TABLE test_db.users COMMENT = 'User account information';",
	}

	for _, cmd := range data {
		_, err := executeMySQLCommand(cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func executeMySQLCommand(cmd string) (string, error) {
	args := []string{"mysql", "-u", "root", "-pmysql", "-e", cmd}
	return env.ContainerManager.ExecCommand("mysql-test-plugin", args)
}
