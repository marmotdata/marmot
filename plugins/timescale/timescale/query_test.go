package timescale

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlanCollectsRelations(t *testing.T) {
	summary, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "Hash Join",
		"Plans": [
			{"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public"},
			{"Node Type": "Hash", "Plans": [
				{"Node Type": "Index Scan", "Relation Name": "customers", "Schema": "public"},
				{"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public"}
			]}
		]
	}}]`))
	require.NoError(t, err)
	assert.Equal(t, []relation{{schema: "public", name: "orders"}, {schema: "public", name: "customers"}}, summary.relations)
	assert.Empty(t, summary.opaqueCalls)
}

// A query over a hypertable plans as an Append of its chunks, each of which
// shows up as its own relation under the internal schema.
func TestParsePlanCollectsChunkRelations(t *testing.T) {
	summary, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "Append",
		"Plans": [
			{"Node Type": "Seq Scan", "Relation Name": "_hyper_1_1_chunk", "Schema": "_timescaledb_internal", "Alias": "readings"},
			{"Node Type": "Seq Scan", "Relation Name": "_hyper_1_2_chunk", "Schema": "_timescaledb_internal", "Alias": "readings_1"}
		]
	}}]`))
	require.NoError(t, err)
	assert.Equal(t, []relation{
		{schema: "_timescaledb_internal", name: "_hyper_1_1_chunk"},
		{schema: "_timescaledb_internal", name: "_hyper_1_2_chunk"},
	}, summary.relations)
}

func TestParsePlanRefusesWrites(t *testing.T) {
	// A data-modifying CTE inside a SELECT still plans a ModifyTable node.
	_, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "CTE Scan",
		"Plans": [{"Node Type": "ModifyTable", "Operation": "Delete", "Relation Name": "orders", "Schema": "public"}]
	}}]`))
	require.ErrorIs(t, err, errReadOnly)

	_, err = parsePlan([]byte(`[{"Plan": {"Node Type": "LockRows", "Plans": [{"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public"}]}}]`))
	require.ErrorIs(t, err, errReadOnly)
}

func TestParsePlanWithNoRelations(t *testing.T) {
	summary, err := parsePlan([]byte(`[{"Plan": {"Node Type": "Result"}}]`))
	require.NoError(t, err)
	assert.Empty(t, summary.relations, "the host then falls back to a whole-source grant")
}

func TestParsePlanFindsOpaqueCalls(t *testing.T) {
	summary, err := parsePlan([]byte(`[{"Plan": {
		"Node Type": "Seq Scan", "Relation Name": "orders", "Schema": "public",
		"Filter": "(query_to_xml('select * from payroll'::text, true, false, ''::text) IS NOT NULL)"
	}}]`))
	require.NoError(t, err)
	assert.Equal(t, []string{"query_to_xml"}, summary.opaqueCalls)
}

func TestInternalRelations_KeepsOnlyTimescaleAndPostgresSchemas(t *testing.T) {
	relations := []relation{
		{schema: "public", name: "orders"},
		{schema: "_timescaledb_internal", name: "_hyper_1_1_chunk"},
		{schema: "timescaledb_information", name: "chunks"},
		{schema: "pg_catalog", name: "pg_class"},
	}
	assert.Equal(t, []relation{
		{schema: "_timescaledb_internal", name: "_hyper_1_1_chunk"},
		{schema: "timescaledb_information", name: "chunks"},
		{schema: "pg_catalog", name: "pg_class"},
	}, internalRelations(relations))
}

func TestParseConfig_DefaultsQueryDatabaseToPostgresWhenNoDatabaseIsConfigured(t *testing.T) {
	config, err := parseConfig(pluginsdk.RawConfig{"host": "db", "user": "marmot"})
	require.NoError(t, err)
	assert.Equal(t, "postgres", config.QueryDatabase)
}

func TestParseConfig_DefaultsQueryDatabaseToTheConfiguredDiscoveryDatabase(t *testing.T) {
	config, err := parseConfig(pluginsdk.RawConfig{"host": "db", "user": "marmot", "database": "metrics"})
	require.NoError(t, err)
	assert.Equal(t, "metrics", config.QueryDatabase)
}

func TestParseConfig_RespectsAnExplicitQueryDatabase(t *testing.T) {
	config, err := parseConfig(pluginsdk.RawConfig{"host": "db", "user": "marmot", "database": "metrics", "query_database": "reporting"})
	require.NoError(t, err)
	assert.Equal(t, "reporting", config.QueryDatabase)
}

// Two sources served by one process must each keep their own pool; asking for one never closes the other.
func TestQueryPoolsKeepOnePoolPerConfig(t *testing.T) {
	// pgxpool connects lazily, so these pools never touch the network.
	open := func() *pgxpool.Pool {
		pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/db")
		require.NoError(t, err)
		return pool
	}
	var q queryPools
	a, b := q.put("a", open()), q.put("b", open())

	assert.Same(t, a, q.get("a"))
	assert.Same(t, b, q.get("b"))
	assert.NotSame(t, a, b)

	duplicate := open()
	assert.Same(t, a, q.put("a", duplicate), "a pool opened concurrently for the same config is discarded")

	q.pools["a"].lastUsed = time.Now().Add(-poolIdleTTL - time.Second)
	assert.Nil(t, q.get("a"), "an idle pool is closed and forgotten")
	assert.Same(t, b, q.get("b"))
}

// TestQuery_Integration exercises PlanQuery and ExecuteQuery against a real
// TimescaleDB. Point MARMOT_TEST_TIMESCALE_DSN at a throwaway instance, for
// example:
//
//	docker run -d --name qtest-timescale -p 55510:5432 \
//	  -e POSTGRES_PASSWORD=postgres timescale/timescaledb:latest-pg16
//	MARMOT_TEST_TIMESCALE_DSN=postgres://postgres:postgres@localhost:55510/postgres?sslmode=disable go test ./...
//	docker rm -f qtest-timescale
//
// The test creates and drops its own schema, so it is safe to run against a
// persistent instance too.
func TestQuery_Integration(t *testing.T) {
	dsn := os.Getenv("MARMOT_TEST_TIMESCALE_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_TIMESCALE_DSN to run the data query tests against a real TimescaleDB")
	}

	ctx := context.Background()
	connConfig, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)

	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	const schemaName = "marmot_query_test"
	_, err = conn.Exec(ctx, `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schemaName+` CASCADE`)
	})

	_, err = conn.Exec(ctx, `
		CREATE SCHEMA `+schemaName+`;

		CREATE TABLE `+schemaName+`.readings (
			time TIMESTAMPTZ NOT NULL,
			device_id INT,
			value DOUBLE PRECISION
		);
		SELECT create_hypertable('`+schemaName+`.readings', 'time');
		INSERT INTO `+schemaName+`.readings
			SELECT now() - (i || ' minutes')::interval, i % 3, i
			FROM generate_series(1, 20) i;

		CREATE TABLE `+schemaName+`.devices (id INT PRIMARY KEY, name TEXT);
		INSERT INTO `+schemaName+`.devices VALUES (0, 'a'), (1, 'b'), (2, 'c');

		CREATE VIEW `+schemaName+`.readings_with_device AS
			SELECT r.time, d.name, r.value
			FROM `+schemaName+`.readings r
			JOIN `+schemaName+`.devices d ON d.id = r.device_id;
	`)
	require.NoError(t, err)

	raw := pluginsdk.RawConfig{
		"host": connConfig.Host, "port": int(connConfig.Port),
		"user": connConfig.User, "password": connConfig.Password,
		"query_database": connConfig.Database, "ssl_mode": "disable",
	}

	s := &Source{}

	t.Run("hypertable chunks resolve to the hypertable's own MRN", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT * FROM ` + schemaName + `.readings`})
		require.NoError(t, err)
		assert.Equal(t, []string{assetMRN(&Config{}, "Table", connConfig.Database, schemaName, "readings")}, plan.ReferencedMRNs)
		assert.False(t, plan.Incomplete)
	})

	t.Run("plain table keeps its own MRN", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT * FROM ` + schemaName + `.devices`})
		require.NoError(t, err)
		assert.Equal(t, []string{assetMRN(&Config{}, "Table", connConfig.Database, schemaName, "devices")}, plan.ReferencedMRNs)
	})

	t.Run("a view expands to the hypertable and the table it joins", func(t *testing.T) {
		plan, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT * FROM ` + schemaName + `.readings_with_device`})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{assetMRN(&Config{}, "Table", connConfig.Database, schemaName, "readings"), assetMRN(&Config{}, "Table", connConfig.Database, schemaName, "devices")}, plan.ReferencedMRNs)
	})

	refused := []string{
		`INSERT INTO ` + schemaName + `.devices VALUES (99, 'z')`,
		`UPDATE ` + schemaName + `.devices SET name = 'z' WHERE id = 0`,
		`DELETE FROM ` + schemaName + `.devices WHERE id = 0`,
		`DROP TABLE ` + schemaName + `.devices`,
		`SELECT 1; SELECT 2`,
		`WITH removed AS (DELETE FROM ` + schemaName + `.devices RETURNING id) SELECT * FROM removed`,
	}
	for _, statement := range refused {
		t.Run("PlanQuery refuses: "+statement, func(t *testing.T) {
			_, err := s.PlanQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement})
			require.Error(t, err)
		})
		t.Run("ExecuteQuery refuses: "+statement, func(t *testing.T) {
			err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: statement}, func(pluginsdk.QueryResultChunk) error { return nil })
			require.Error(t, err)
		})
	}

	t.Run("execute streams rows with column types", func(t *testing.T) {
		var columns []pluginsdk.QueryColumn
		var rows [][]any
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT id, name FROM ` + schemaName + `.devices ORDER BY id`}, func(chunk pluginsdk.QueryResultChunk) error {
			if chunk.Columns != nil {
				columns = chunk.Columns
			}
			rows = append(rows, chunk.Rows...)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, columns, 2)
		assert.Equal(t, "id", columns[0].Name)
		assert.NotEmpty(t, columns[0].Type)
		require.Len(t, rows, 3)
		assert.Equal(t, "a", rows[0][1])
	})

	t.Run("a statement with no rows still carries the schema", func(t *testing.T) {
		var columns []pluginsdk.QueryColumn
		var chunks int
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT id FROM ` + schemaName + `.devices WHERE id < 0`}, func(chunk pluginsdk.QueryResultChunk) error {
			chunks++
			columns = chunk.Columns
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 1, chunks)
		require.Len(t, columns, 1)
	})

	t.Run("max rows is honoured", func(t *testing.T) {
		var rows [][]any
		err := s.ExecuteQuery(ctx, raw, pluginsdk.QueryRequest{Statement: `SELECT * FROM ` + schemaName + `.readings`, MaxRows: 5}, func(chunk pluginsdk.QueryResultChunk) error {
			rows = append(rows, chunk.Rows...)
			return nil
		})
		require.NoError(t, err)
		assert.Len(t, rows, 5)
	})
}
