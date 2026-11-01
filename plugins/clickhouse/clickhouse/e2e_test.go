package clickhouse_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real ClickHouse server:
//
//	docker run -d --name qtest-clickhouse -p 55550:9000 -p 55551:8123 \
//	  -e CLICKHOUSE_USER=marmot -e CLICKHOUSE_PASSWORD=marmot \
//	  clickhouse/clickhouse-server:24.8
//	MARMOT_TEST_CLICKHOUSE_HOST=localhost MARMOT_TEST_CLICKHOUSE_PORT=55550 \
//	  MARMOT_TEST_CLICKHOUSE_USER=marmot MARMOT_TEST_CLICKHOUSE_PASSWORD=marmot \
//	  go test ./...
//
// The fixture is created by the tests themselves and is idempotent, so the
// container can be reused across runs; remove it afterwards with
// `docker rm -f qtest-clickhouse`.

const (
	ordersMRN         = "mrn://table/clickhouse/shop.orders"
	customersMRN      = "mrn://table/clickhouse/shop.customers"
	customerTotalsMRN = "mrn://view/clickhouse/shop.customer_totals"
)

var seedStatements = []string{
	`CREATE DATABASE IF NOT EXISTS shop`,
	`CREATE TABLE IF NOT EXISTS shop.customers (
		id UInt64,
		name String
	) ENGINE = MergeTree ORDER BY id`,
	`CREATE TABLE IF NOT EXISTS shop.orders (
		id UInt64,
		customer_id UInt64,
		amount Decimal(10, 2)
	) ENGINE = MergeTree ORDER BY id`,
	`CREATE VIEW IF NOT EXISTS shop.customer_totals AS
		SELECT o.id AS id, c.name AS name, o.amount AS amount
		FROM shop.orders AS o
		INNER JOIN shop.customers AS c ON o.customer_id = c.id`,
	`TRUNCATE TABLE shop.customers`,
	`TRUNCATE TABLE shop.orders`,
	`INSERT INTO shop.customers (id, name) VALUES (1, 'alice'), (2, 'bob')`,
	`INSERT INTO shop.orders (id, customer_id, amount) VALUES (1, 1, 10.50), (2, 2, 20.75), (3, 1, 5.25)`,
}

type e2eParams struct {
	host, user, password, database string
	port                           int
}

func e2eEnv(t *testing.T) e2eParams {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_CLICKHOUSE_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_CLICKHOUSE_HOST is not set, skipping ClickHouse e2e tests")
	}

	port := 9000
	if raw := os.Getenv("MARMOT_TEST_CLICKHOUSE_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_CLICKHOUSE_PORT must be a number")
		port = parsed
	}

	return e2eParams{
		host:     host,
		port:     port,
		user:     envOr("MARMOT_TEST_CLICKHOUSE_USER", "default"),
		password: envOr("MARMOT_TEST_CLICKHOUSE_PASSWORD", ""),
		database: envOr("MARMOT_TEST_CLICKHOUSE_DATABASE", "shop"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (p e2eParams) config() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host":     p.host,
		"port":     p.port,
		"user":     p.user,
		"password": p.password,
		"database": p.database,
		"secure":   false,
	}
}

// seed connects directly with the driver, bypassing the plugin entirely, and leaves the fixture in a known state so every test run starts from the same data regardless of what an earlier run left behind.
func (p e2eParams) seed(t *testing.T) {
	t.Helper()

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", p.host, p.port)},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: p.user,
			Password: p.password,
		},
	})
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	for _, stmt := range seedStatements {
		require.NoErrorf(t, conn.Exec(ctx, stmt), "seeding: %s", stmt)
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// Discovery runs once and the result is shared, so each behaviour below
// can be its own small test without re-running discovery every time.
var (
	discoverOnce   sync.Once
	discoverResult *pluginsdk.DiscoveryResult
	discoverErr    error
)

func discover(t *testing.T, p e2eParams) *pluginsdk.DiscoveryResult {
	t.Helper()
	discoverOnce.Do(func() {
		discoverResult, discoverErr = buildBinary(t).Discover(t.Context(), p.config())
	})
	require.NoError(t, discoverErr)
	require.NotNil(t, discoverResult)
	return discoverResult
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		a := &result.Assets[i]
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	return nil
}

func requireAsset(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	t.Helper()
	a := findAsset(result, assetType, name)
	require.NotNilf(t, a, "expected %s asset %q", assetType, name)
	return a
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", meta.ID)
	assert.True(t, meta.SupportsQuery, "Serve sets SupportsQuery because Source implements Querier")
}

func TestE2E_DiscoveryMRNsMatchTheFixture(t *testing.T) {
	p := e2eEnv(t)
	p.seed(t)
	result := discover(t, p)

	assert.Equal(t, ordersMRN, *requireAsset(t, result, "Table", "orders").MRN)
	assert.Equal(t, customersMRN, *requireAsset(t, result, "Table", "customers").MRN)
	assert.Equal(t, customerTotalsMRN, *requireAsset(t, result, "View", "customer_totals").MRN)
}

func TestE2E_QueryPlanNamesTheTableARead(t *testing.T) {
	p := e2eEnv(t)
	p.seed(t)
	bin := buildBinary(t)

	plan, chunks, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{
		Statement: "SELECT id, amount FROM shop.orders ORDER BY id",
	})
	require.NoError(t, err)

	require.NotNil(t, plan)
	assert.Equal(t, []string{ordersMRN}, plan.ReferencedMRNs)
	assert.False(t, plan.Incomplete)

	require.Len(t, chunks, 1)
	require.Equal(t, []pluginsdk.QueryColumn{
		{Name: "id", Type: "UInt64"},
		{Name: "amount", Type: "Decimal(10, 2)"},
	}, chunks[0].Columns)
	require.Len(t, chunks[0].Rows, 3)
	assert.EqualValues(t, 1, chunks[0].Rows[0][0])
	assert.Equal(t, "10.5", chunks[0].Rows[0][1])
}

func TestE2E_QueryPlanOfAJoinNamesBothTables(t *testing.T) {
	p := e2eEnv(t)
	p.seed(t)
	bin := buildBinary(t)

	plan, _, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{
		Statement: "SELECT o.id, c.name FROM shop.orders o JOIN shop.customers c ON o.customer_id = c.id",
	})
	require.NoError(t, err)

	require.NotNil(t, plan)
	assert.ElementsMatch(t, []string{ordersMRN, customersMRN}, plan.ReferencedMRNs)
}

// A statement over the view is expanded by ClickHouse's planner into the
// tables behind it, so the plan names those tables rather than the view
// itself; that is an accepted limitation, not a bug, and the MRNs it
// produces are still exactly the ones discovery gave those tables.
func TestE2E_QueryPlanOfAViewNamesItsUnderlyingTables(t *testing.T) {
	p := e2eEnv(t)
	p.seed(t)
	bin := buildBinary(t)

	plan, chunks, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{
		Statement: "SELECT * FROM shop.customer_totals ORDER BY id",
	})
	require.NoError(t, err)

	require.NotNil(t, plan)
	assert.ElementsMatch(t, []string{ordersMRN, customersMRN}, plan.ReferencedMRNs)

	require.Len(t, chunks, 1)
	require.Len(t, chunks[0].Rows, 3)
}

func TestE2E_QueryHonoursMaxRows(t *testing.T) {
	p := e2eEnv(t)
	bin := buildBinary(t)

	_, chunks, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{
		Statement: "SELECT number FROM system.numbers LIMIT 1000000",
		MaxRows:   5,
	})
	require.NoError(t, err)

	var total int
	for _, c := range chunks {
		total += len(c.Rows)
	}
	assert.Equal(t, 5, total)
}

func TestE2E_QueryRefusesWritesAndDDL(t *testing.T) {
	p := e2eEnv(t)
	p.seed(t)
	bin := buildBinary(t)

	statements := map[string]string{
		"insert":   "INSERT INTO shop.orders (id, customer_id, amount) VALUES (99, 1, 1)",
		"alter":    "ALTER TABLE shop.orders DELETE WHERE id = 1",
		"create":   "CREATE TABLE shop.stolen (id UInt64) ENGINE = Memory",
		"drop":     "DROP TABLE shop.orders",
		"set":      "SELECT 1 SETTINGS readonly = 0",
		"optimize": "OPTIMIZE TABLE shop.orders",
	}
	for name, statement := range statements {
		t.Run(name, func(t *testing.T) {
			_, _, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{Statement: statement})
			require.Error(t, err)
		})
	}
}

func TestE2E_QueryRefusesExternalTableFunctions(t *testing.T) {
	p := e2eEnv(t)
	bin := buildBinary(t)

	statements := map[string]string{
		"url":    "SELECT * FROM url('http://169.254.169.254/latest/meta-data/', CSV, 'a String')",
		"file":   "SELECT * FROM file('/etc/passwd', CSV, 'a String')",
		"remote": "SELECT * FROM remote('127.0.0.1:9000', system.one)",
	}
	for name, statement := range statements {
		t.Run(name, func(t *testing.T) {
			_, _, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{Statement: statement})
			require.Error(t, err)
		})
	}
}

func TestE2E_QueryRefusesMultipleStatements(t *testing.T) {
	p := e2eEnv(t)
	bin := buildBinary(t)

	_, _, err := bin.Query(t.Context(), p.config(), pluginsdk.QueryRequest{
		Statement: "SELECT 1; SELECT 2",
	})
	require.Error(t, err)
}
