package doris_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Doris frontend. They need
// MARMOT_TEST_DORIS_HOST (and optionally _PORT, _USER, _PASSWORD) to point
// at one, for example the apache/doris:doris-all-in-one-2.1.0 image:
//
//	docker run -d --name marmot-test-doris --memory 4g -p 19030:9030 apache/doris:doris-all-in-one-2.1.0
//	MARMOT_TEST_DORIS_HOST=127.0.0.1 MARMOT_TEST_DORIS_PORT=19030 go test ./...
//
// The fixture database is created by the tests themselves and left in
// place, so reruns are cheap.

const e2eDatabase = "shop"

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_DORIS_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_DORIS_HOST is not set; skipping Doris e2e tests")
	}

	config := pluginsdk.RawConfig{
		"host":      host,
		"user":      "root",
		"password":  os.Getenv("MARMOT_TEST_DORIS_PASSWORD"),
		"databases": []any{e2eDatabase},
	}
	if user := os.Getenv("MARMOT_TEST_DORIS_USER"); user != "" {
		config["user"] = user
	}
	if port := os.Getenv("MARMOT_TEST_DORIS_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		require.NoError(t, err, "MARMOT_TEST_DORIS_PORT")
		config["port"] = p
	}
	return config
}

func openE2EDB(t *testing.T, config pluginsdk.RawConfig) *sql.DB {
	t.Helper()

	cfg := mysql.NewConfig()
	cfg.User = config["user"].(string)
	cfg.Passwd = config["password"].(string)
	cfg.Net = "tcp"
	port := 9030
	if p, ok := config["port"].(int); ok {
		port = p
	}
	cfg.Addr = fmt.Sprintf("%s:%d", config["host"], port)
	cfg.Timeout = 15 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.PingContext(t.Context()))
	return db
}

// seedStatements builds the fixture: one table per key model, range
// partitioning, hash and random distribution, aggregation types, complex
// column types, a view that joins two tables and an async materialized
// view over the same join. Every statement is safe to rerun.
var seedStatements = []string{
	`CREATE DATABASE IF NOT EXISTS shop`,
	`CREATE TABLE IF NOT EXISTS shop.events (
		event_date DATE NOT NULL COMMENT "Day the event happened",
		event_id BIGINT NOT NULL COMMENT "Event identifier",
		customer_id INT NOT NULL COMMENT "Customer who triggered the event",
		event_type VARCHAR(32) NOT NULL DEFAULT "click" COMMENT "click, view or purchase",
		amount DECIMALV3(9, 2) NULL COMMENT "Purchase amount",
		tags ARRAY<INT> NULL COMMENT "Tag ids attached to the event",
		props STRUCT<source:STRING, campaign:STRING> NULL COMMENT "Attribution",
		created_at DATETIMEV2 NULL
	) ENGINE=OLAP
	DUPLICATE KEY(event_date, event_id)
	PARTITION BY RANGE(event_date) (
		PARTITION p202601 VALUES [("2026-01-01"), ("2026-02-01")),
		PARTITION p202602 VALUES [("2026-02-01"), ("2026-03-01"))
	)
	DISTRIBUTED BY HASH(event_id) BUCKETS 4
	PROPERTIES ("replication_allocation" = "tag.location.default: 1", "storage_medium" = "HDD")`,
	// Doris 2.1 drops a COMMENT clause given inside CREATE TABLE, so the
	// table comment is set afterwards.
	`ALTER TABLE shop.events MODIFY COMMENT "Raw click stream events"`,
	`CREATE TABLE IF NOT EXISTS shop.customers (
		customer_id INT NOT NULL COMMENT "Customer identifier",
		email VARCHAR(255) NOT NULL,
		country CHAR(2) NULL DEFAULT "NL",
		signed_up DATEV2 NULL,
		lifetime_value DECIMALV3(12, 2) NULL DEFAULT "0"
	) ENGINE=OLAP
	UNIQUE KEY(customer_id)
	DISTRIBUTED BY HASH(customer_id) BUCKETS 2
	PROPERTIES ("replication_allocation" = "tag.location.default: 1")`,
	`CREATE TABLE IF NOT EXISTS shop.daily_stats (
		stat_date DATE NOT NULL,
		country CHAR(2) NOT NULL,
		purchases BIGINT SUM NULL DEFAULT "0" COMMENT "Purchases that day",
		max_amount DECIMALV3(9, 2) MAX NULL,
		last_event_type VARCHAR(32) REPLACE NULL,
		visitors HLL HLL_UNION NOT NULL COMMENT "Approximate distinct visitors",
		buyers BITMAP BITMAP_UNION NOT NULL
	) ENGINE=OLAP
	AGGREGATE KEY(stat_date, country)
	DISTRIBUTED BY HASH(stat_date) BUCKETS 1
	PROPERTIES ("replication_allocation" = "tag.location.default: 1")`,
	`CREATE TABLE IF NOT EXISTS shop.orders (
		order_id BIGINT NOT NULL,
		customer_id INT NOT NULL,
		order_total DECIMALV3(9, 2) NULL
	) ENGINE=OLAP
	DUPLICATE KEY(order_id)
	DISTRIBUTED BY RANDOM BUCKETS AUTO
	PROPERTIES ("replication_allocation" = "tag.location.default: 1")`,
	`CREATE VIEW IF NOT EXISTS shop.daily_sales COMMENT "Purchases per day and country" AS
	SELECT e.event_date, c.country, SUM(e.amount) AS revenue, COUNT(*) AS purchases
	FROM shop.events e
	JOIN shop.customers c ON e.customer_id = c.customer_id
	WHERE e.event_type = 'purchase'
	GROUP BY e.event_date, c.country`,
}

// seedRows is loaded once per table, only while the table is empty, so
// row counts stay what the statistics test expects.
var seedRows = map[string]string{
	"customers": `INSERT INTO shop.customers VALUES
		(1, 'ann@example.com', 'NL', '2025-12-01', 120.50),
		(2, 'bob@example.com', 'DE', '2026-01-15', 0),
		(3, 'cyd@example.com', 'NL', '2026-02-02', 42.00)`,
	"events": `INSERT INTO shop.events VALUES
		('2026-01-05', 1, 1, 'purchase', 20.50, [1, 2], {'web', 'jan'}, '2026-01-05 10:00:00'),
		('2026-01-06', 2, 2, 'click', NULL, [3], {'app', NULL}, '2026-01-06 11:00:00'),
		('2026-02-10', 3, 1, 'purchase', 100.00, NULL, NULL, '2026-02-10 12:00:00'),
		('2026-02-11', 4, 3, 'purchase', 42.00, [], {'web', 'feb'}, '2026-02-11 13:00:00')`,
	"daily_stats": `INSERT INTO shop.daily_stats VALUES
		('2026-01-05', 'NL', 1, 20.50, 'purchase', hll_hash(1), to_bitmap(1)),
		('2026-01-05', 'NL', 1, 5.00, 'click', hll_hash(2), to_bitmap(2))`,
	"orders": `INSERT INTO shop.orders VALUES (100, 1, 20.50), (101, 3, 42.00)`,
}

// The materialized view is created after the rows so BUILD IMMEDIATE has
// something to materialize.
const seedMaterializedView = `CREATE MATERIALIZED VIEW IF NOT EXISTS shop.mv_sales
	BUILD IMMEDIATE REFRESH COMPLETE ON MANUAL
	DISTRIBUTED BY HASH(event_date) BUCKETS 2
	PROPERTIES ("replication_allocation" = "tag.location.default: 1")
	AS SELECT e.event_date, c.country, SUM(e.amount) AS revenue
	FROM shop.events e JOIN shop.customers c ON e.customer_id = c.customer_id
	GROUP BY e.event_date, c.country`

// Doris records constraints for the planner only, and adding one that
// already exists is an error, so each is added once.
var seedConstraints = map[string]string{
	"customers": `ALTER TABLE shop.customers ADD CONSTRAINT pk_customer PRIMARY KEY (customer_id)`,
	"orders":    `ALTER TABLE shop.orders ADD CONSTRAINT fk_customer FOREIGN KEY (customer_id) REFERENCES shop.customers (customer_id)`,
}

func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := t.Context()

	for _, stmt := range seedStatements {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err, stmt)
	}

	for table, insert := range seedRows {
		var count int64
		require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM shop."+table).Scan(&count))
		if count > 0 {
			continue
		}
		_, err := db.ExecContext(ctx, insert)
		require.NoError(t, err, insert)
	}

	_, err := db.ExecContext(ctx, seedMaterializedView)
	require.NoError(t, err)

	for table, alter := range seedConstraints {
		rows, err := db.QueryContext(ctx, "SHOW CONSTRAINTS FROM shop."+table)
		require.NoError(t, err)
		has := rows.Next()
		require.NoError(t, rows.Close())
		if has {
			continue
		}
		_, err = db.ExecContext(ctx, alter)
		require.NoError(t, err, alter)
	}
}

// discoverE2E seeds the fixture and runs Discover through the built
// binary. Discovery is fast, so each test gets its own run.
func discoverE2E(t *testing.T) (pluginsdk.RawConfig, *pluginsdk.DiscoveryResult) {
	t.Helper()

	config := e2eConfig(t)
	seed(t, openE2EDB(t, config))

	bin := plugintest.Build(t, "..")
	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return config, result
}

func findAsset(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].Name != nil && *result.Assets[i].Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func requireAsset(t *testing.T, result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	t.Helper()
	a := findAsset(result, name)
	require.NotNil(t, a, "expected asset %s", name)
	return a
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected columns on %s", *a.Name)

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]any, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

// Lists over the wire arrive as []any.
func stringList(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)
	bin := plugintest.Build(t, "..")

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "doris", meta.ID)
	assert.Equal(t, "Doris", meta.Name)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	e2eConfig(t)
	bin := plugintest.Build(t, "..")

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"user": "root"})
	require.Error(t, err)
}

func TestE2E_DiscoverUnreachableHostFails(t *testing.T) {
	config := e2eConfig(t)
	config["port"] = 1
	bin := plugintest.Build(t, "..")

	_, err := bin.Discover(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_DiscoverEmitsTheDatabaseAsset(t *testing.T) {
	_, result := discoverE2E(t)

	db := requireAsset(t, result, "shop")
	assert.Equal(t, "Database", db.Type)
	assert.Equal(t, []string{"Doris"}, db.Providers)
	assert.Equal(t, "mrn://database/doris/shop", *db.MRN)
	assert.Equal(t, "internal", db.Metadata["catalog"])
	assert.Contains(t, db.Metadata["doris_version"], "doris-2.1")
	assert.Equal(t, float64(4), db.Metadata["table_count"])
	assert.Equal(t, float64(2), db.Metadata["view_count"])
}

func TestE2E_DiscoverNamesObjectsByDatabaseAndTable(t *testing.T) {
	_, result := discoverE2E(t)

	events := requireAsset(t, result, "shop.events")
	assert.Equal(t, "Table", events.Type)
	assert.Equal(t, "mrn://table/doris/shop.events", *events.MRN)
	assert.Equal(t, "shop", events.Metadata["database"])
	assert.Equal(t, "events", events.Metadata["table_name"])
	assert.Equal(t, "table", events.Metadata["object_type"])
	assert.Equal(t, "Doris", events.Metadata["engine"])
}

func TestE2E_DiscoverReportsTheKeyModels(t *testing.T) {
	_, result := discoverE2E(t)

	events := requireAsset(t, result, "shop.events")
	assert.Equal(t, "DUPLICATE", events.Metadata["key_model"])
	assert.Equal(t, []string{"event_date", "event_id"}, stringList(events.Metadata["key_columns"]))

	customers := requireAsset(t, result, "shop.customers")
	assert.Equal(t, "UNIQUE", customers.Metadata["key_model"])
	assert.Equal(t, []string{"customer_id"}, stringList(customers.Metadata["key_columns"]))

	stats := requireAsset(t, result, "shop.daily_stats")
	assert.Equal(t, "AGGREGATE", stats.Metadata["key_model"])
	assert.Equal(t, []string{"stat_date", "country"}, stringList(stats.Metadata["key_columns"]))
}

func TestE2E_DiscoverReportsPartitioningAndDistribution(t *testing.T) {
	_, result := discoverE2E(t)

	events := requireAsset(t, result, "shop.events")
	assert.Equal(t, "RANGE", events.Metadata["partition_type"])
	assert.Equal(t, []string{"event_date"}, stringList(events.Metadata["partition_columns"]))
	assert.Equal(t, float64(2), events.Metadata["partition_count"])
	assert.Equal(t, "HASH", events.Metadata["distribution_type"])
	assert.Equal(t, []string{"event_id"}, stringList(events.Metadata["distribution_columns"]))
	assert.Equal(t, float64(4), events.Metadata["buckets"])
	assert.Equal(t, "tag.location.default: 1", events.Metadata["replication"])
	assert.Equal(t, "hdd", events.Metadata["storage_medium"])

	orders := requireAsset(t, result, "shop.orders")
	assert.Equal(t, "none", orders.Metadata["partition_type"])
	assert.Equal(t, float64(1), orders.Metadata["partition_count"])
	assert.Equal(t, "RANDOM", orders.Metadata["distribution_type"])
	assert.Equal(t, true, orders.Metadata["auto_bucket"])
}

func TestE2E_DiscoverKeepsTheDDLAndTableComment(t *testing.T) {
	_, result := discoverE2E(t)

	events := requireAsset(t, result, "shop.events")
	require.NotNil(t, events.Description)
	assert.Equal(t, "Raw click stream events", *events.Description)
	assert.Equal(t, "Raw click stream events", events.Metadata["comment"])
	assert.Contains(t, events.Metadata["ddl"], "DUPLICATE KEY(`event_date`, `event_id`)")

	// The OLAP placeholder Doris writes for uncommented tables is not a
	// description.
	orders := requireAsset(t, result, "shop.orders")
	assert.Nil(t, orders.Description)
	assert.NotContains(t, orders.Metadata, "comment")
}

func TestE2E_DiscoverKeepsDeclaredColumnTypesAndComments(t *testing.T) {
	_, result := discoverE2E(t)

	columns := columnsOf(t, requireAsset(t, result, "shop.events"))
	require.Len(t, columns, 8)

	assert.Equal(t, "decimalv3(9, 2)", columns["amount"]["data_type"])
	assert.Equal(t, "array<int>", columns["tags"]["data_type"])
	assert.Equal(t, "struct<source:text,campaign:text>", columns["props"]["data_type"])
	assert.Equal(t, "datev2", columns["event_date"]["data_type"])
	assert.Equal(t, "Day the event happened", columns["event_date"]["description"])
	assert.Equal(t, "click", columns["event_type"]["default_expression"])
	assert.Equal(t, false, columns["event_type"]["is_nullable"])
	assert.Equal(t, true, columns["amount"]["is_nullable"])
}

func TestE2E_DiscoverFlagsKeyColumns(t *testing.T) {
	_, result := discoverE2E(t)

	// UNIQUE KEY columns identify a row, so they are the primary key.
	customers := columnsOf(t, requireAsset(t, result, "shop.customers"))
	assert.Equal(t, true, customers["customer_id"]["is_primary_key"])
	assert.Equal(t, true, customers["customer_id"]["is_key"])
	assert.NotContains(t, customers["email"], "is_key")

	// DUPLICATE KEY columns only order the data.
	events := columnsOf(t, requireAsset(t, result, "shop.events"))
	assert.Equal(t, true, events["event_date"]["is_sorting_key"])
	assert.Equal(t, true, events["event_date"]["is_key"])
	assert.NotContains(t, events["event_date"], "is_primary_key")
}

func TestE2E_DiscoverReportsAggregationTypes(t *testing.T) {
	_, result := discoverE2E(t)

	columns := columnsOf(t, requireAsset(t, result, "shop.daily_stats"))
	require.Len(t, columns, 7)

	assert.Equal(t, "SUM", columns["purchases"]["aggregation_type"])
	assert.Equal(t, "MAX", columns["max_amount"]["aggregation_type"])
	assert.Equal(t, "REPLACE", columns["last_event_type"]["aggregation_type"])
	assert.Equal(t, "HLL_UNION", columns["visitors"]["aggregation_type"])
	assert.Equal(t, "BITMAP_UNION", columns["buyers"]["aggregation_type"])
	assert.NotContains(t, columns["stat_date"], "aggregation_type")
	assert.Equal(t, true, columns["stat_date"]["is_primary_key"])
	// HLL and BITMAP columns carry an internal NUL default that is not a
	// value anyone declared.
	assert.NotContains(t, columns["visitors"], "default_expression")
}

func TestE2E_DiscoverCapturesTheViewQuery(t *testing.T) {
	_, result := discoverE2E(t)

	view := requireAsset(t, result, "shop.daily_sales")
	assert.Equal(t, "View", view.Type)
	assert.Equal(t, "mrn://view/doris/shop.daily_sales", *view.MRN)
	assert.Equal(t, "view", view.Metadata["object_type"])
	assert.Equal(t, "View", view.Metadata["engine"])
	require.NotNil(t, view.Description)
	assert.Equal(t, "Purchases per day and country", *view.Description)
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "FROM `shop`.`events`")
	assert.NotContains(t, *view.Query, "CREATE VIEW")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
	assert.Len(t, columnsOf(t, view), 4)
}

func TestE2E_DiscoverCapturesTheMaterializedView(t *testing.T) {
	_, result := discoverE2E(t)

	mv := requireAsset(t, result, "shop.mv_sales")
	assert.Equal(t, "View", mv.Type)
	assert.Equal(t, "materialized_view", mv.Metadata["object_type"])
	assert.Equal(t, true, mv.Metadata["materialized"])
	assert.Equal(t, "MATERIALIZED_VIEW", mv.Metadata["engine"])
	assert.Equal(t, "BUILD IMMEDIATE REFRESH COMPLETE ON MANUAL", mv.Metadata["refresh_info"])
	assert.NotEmpty(t, mv.Metadata["refresh_state"])
	assert.NotEmpty(t, mv.Metadata["job_name"])
	assert.Equal(t, "HASH", mv.Metadata["distribution_type"])
	require.NotNil(t, mv.Query)
	assert.Contains(t, *mv.Query, "FROM shop.events e JOIN shop.customers c")
	assert.Len(t, columnsOf(t, mv), 3)
}

func TestE2E_DiscoverEmitsContainsEdges(t *testing.T) {
	_, result := discoverE2E(t)

	for _, name := range []string{"shop.events", "shop.customers", "shop.daily_stats", "shop.orders"} {
		assert.True(t, hasEdge(result, "mrn://database/doris/shop", "mrn://table/doris/"+name, "CONTAINS"), name)
	}
	assert.True(t, hasEdge(result, "mrn://database/doris/shop", "mrn://view/doris/shop.daily_sales", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://database/doris/shop", "mrn://view/doris/shop.mv_sales", "CONTAINS"))
}

func TestE2E_DiscoverEmitsViewOfEdgesFromBaseTables(t *testing.T) {
	_, result := discoverE2E(t)

	assert.True(t, hasEdge(result, "mrn://table/doris/shop.events", "mrn://view/doris/shop.daily_sales", "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/doris/shop.customers", "mrn://view/doris/shop.daily_sales", "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/doris/shop.events", "mrn://view/doris/shop.mv_sales", "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/doris/shop.customers", "mrn://view/doris/shop.mv_sales", "VIEW_OF"))
}

func TestE2E_DiscoverEmitsForeignKeyEdges(t *testing.T) {
	_, result := discoverE2E(t)

	assert.True(t, hasEdge(result, "mrn://table/doris/shop.orders", "mrn://table/doris/shop.customers", "FOREIGN_KEY"))
}

func TestE2E_EveryEdgePointsAtADiscoveredAsset(t *testing.T) {
	_, result := discoverE2E(t)

	known := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		known[*a.MRN] = struct{}{}
	}
	for _, e := range result.Lineage {
		assert.Contains(t, known, e.Source, e)
		assert.Contains(t, known, e.Target, e)
	}
}

func TestE2E_DiscoverEmitsStatistics(t *testing.T) {
	config := e2eConfig(t)
	db := openE2EDB(t, config)
	seed(t, db)

	// Doris fills TABLE_ROWS from the backends' periodic tablet report,
	// so a freshly loaded table can show zero rows for a minute or two.
	require.Eventually(t, func() bool {
		var rows int64
		err := db.QueryRowContext(t.Context(),
			"SELECT TABLE_ROWS FROM information_schema.TABLES WHERE TABLE_SCHEMA = 'shop' AND TABLE_NAME = 'events'").Scan(&rows)
		return err == nil && rows == 4
	}, 3*time.Minute, 5*time.Second, "waiting for Doris to report the row count of shop.events")

	bin := plugintest.Build(t, "..")
	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	stats := make(map[string]map[string]float64)
	for _, st := range result.Statistics {
		if stats[st.AssetMRN] == nil {
			stats[st.AssetMRN] = make(map[string]float64)
		}
		stats[st.AssetMRN][st.MetricName] = st.Value
	}

	events := stats["mrn://table/doris/shop.events"]
	assert.Equal(t, float64(4), events["asset.row_count"])
	assert.Equal(t, float64(8), events["asset.column_count"])
	assert.Greater(t, events["asset.size_bytes"], float64(0))

	view := stats["mrn://view/doris/shop.daily_sales"]
	assert.Equal(t, float64(4), view["asset.column_count"])
	assert.NotContains(t, view, "asset.row_count")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	config, result := discoverE2E(t)
	customers := requireAsset(t, result, "shop.customers")

	bin := plugintest.Build(t, "..")
	columns, rows, err := bin.FetchSampleData(t.Context(), config, customers)
	require.NoError(t, err)

	assert.Equal(t, []string{"customer_id", "email", "country", "signed_up", "lifetime_value"}, columns)
	require.Len(t, rows, 3)

	var emails, dates []any
	for _, row := range rows {
		require.Len(t, row, 5)
		emails = append(emails, row[1])
		dates = append(dates, row[3])
	}
	assert.Contains(t, emails, "ann@example.com")
	// Dates come back as the text Doris prints, not a timestamp.
	assert.Contains(t, dates, "2025-12-01")
}

func TestE2E_FetchSampleDataRejectsAnAssetWithoutMetadata(t *testing.T) {
	config := e2eConfig(t)
	bin := plugintest.Build(t, "..")

	name := "shop.events"
	_, _, err := bin.FetchSampleData(context.Background(), config, &pluginsdk.Asset{Name: &name, Type: "Table"})
	require.Error(t, err)
}
