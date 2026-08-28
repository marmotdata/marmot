package postgresql

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestViewLineage_Integration runs the view column-lineage path end-to-end
// against a real PostgreSQL. Point MARMOT_TEST_PG_DSN at any reachable
// instance; the default is the testenv service (docker compose service
// "postgres" exposed on localhost:5433 with airflow/airflow credentials).
// The test creates and drops its own schema so it is safe to run against
// a persistent instance.
func TestViewLineage_Integration(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_PG_DSN")
	if dsn == "" {
		dsn = "postgres://airflow:airflow@localhost:5433/airflow?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("skipping: postgres not reachable at %s (%v). Bring up testenv: `docker compose -f testenv/docker-compose.yml up -d postgres`", dsn, err)
	}
	defer conn.Close(context.Background())

	const schemaName = "marmot_test_cll"
	// Fresh schema every run.
	_, err = conn.Exec(ctx, `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE SCHEMA `+schemaName)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`)
	})

	_, err = conn.Exec(ctx, `
        CREATE TABLE `+schemaName+`.orders (
            id INT PRIMARY KEY,
            customer_id INT,
            total NUMERIC
        );
        CREATE VIEW `+schemaName+`.orders_by_customer AS
            SELECT customer_id,
                   SUM(total) AS total_spend,
                   COUNT(*)  AS n
            FROM `+schemaName+`.orders
            GROUP BY customer_id;
    `)
	require.NoError(t, err)

	// Talk to the same instance via the plugin's own connection path so
	// the code under test uses its production wiring.
	src := &Source{
		config: &Config{
			Host:                 "localhost",
			Port:                 5433,
			User:                 "airflow",
			Password:             "airflow",
			SSLMode:              "disable",
			IncludeColumns:       true,
			ExcludeSystemSchemas: true,
		},
	}
	require.NoError(t, src.initConnection(ctx, "airflow"))
	t.Cleanup(src.closeConnection)

	assets, err := src.discoverTablesAndViews(ctx, "airflow")
	require.NoError(t, err)

	edges, err := src.discoverViewLineage(ctx, "airflow", assets)
	require.NoError(t, err)

	viewEdge := findEdgeByTargetSuffix(edges, "orders_by_customer", "orders")
	require.NotNil(t, viewEdge, "expected view→orders lineage edge; got %d edges", len(edges))
	require.Equal(t, "DEPENDS_ON", viewEdge.Type)
	require.NotEmpty(t, viewEdge.ColumnLineage, "expected column-level lineage on view edge")

	byTarget := map[string]pluginsdk.ColumnEdge{}
	for _, ce := range viewEdge.ColumnLineage {
		byTarget[ce.ToColumn] = ce
	}

	if got, ok := byTarget["customer_id"]; assert.True(t, ok, "missing customer_id edge; got %v", byTarget) {
		assert.Equal(t, []string{"customer_id"}, got.FromColumns)
		assert.Empty(t, got.Transform, "customer_id is a plain projection, no transform")
	}
	if got, ok := byTarget["total_spend"]; assert.True(t, ok, "missing total_spend edge; got %v", byTarget) {
		assert.Equal(t, []string{"total"}, got.FromColumns)
		assert.Equal(t, "expression", got.Transform)
	}
}

func findEdgeByTargetSuffix(edges []pluginsdk.LineageEdge, targetContains, sourceContains string) *pluginsdk.LineageEdge {
	for i := range edges {
		e := &edges[i]
		if strings.Contains(e.Target, targetContains) && strings.Contains(e.Source, sourceContains) {
			return e
		}
	}
	return nil
}
