package mariadb_test

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real MariaDB server seeded with
// the "shop" database:
//
//   customers        table, column comments, an INVISIBLE, a JSON and a generated column
//   orders           table, FOREIGN KEY to customers, table comment
//   prices           table WITH SYSTEM VERSIONING
//   customer_totals  view joining orders and customers
//   vip_customers    view over customer_totals
//   order_seq        sequence START WITH 1000 INCREMENT BY 1
//
// Set MARMOT_TEST_MARIADB_HOST (and optionally _PORT, _USER, _PASSWORD,
// _DATABASE) to run them.

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_MARIADB_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_MARIADB_HOST is not set, skipping MariaDB e2e tests")
	}

	port := 3306
	if raw := os.Getenv("MARMOT_TEST_MARIADB_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_MARIADB_PORT must be a number")
		port = parsed
	}

	return pluginsdk.RawConfig{
		"host":     host,
		"port":     port,
		"user":     envOr("MARMOT_TEST_MARIADB_USER", "marmot"),
		"password": envOr("MARMOT_TEST_MARIADB_PASSWORD", "marmot"),
		"database": envOr("MARMOT_TEST_MARIADB_DATABASE", "shop"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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

func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	config := e2eConfig(t)

	discoverOnce.Do(func() {
		discoverResult, discoverErr = buildBinary(t).Discover(t.Context(), config)
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

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()
	raw, ok := a.Schema["columns"]
	require.Truef(t, ok, "expected a column schema on %s", *a.Name)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func hasEdge(result *pluginsdk.DiscoveryResult, edgeType, source, target string) bool {
	for _, e := range result.Lineage {
		if e.Type == edgeType && e.Source == source && e.Target == target {
			return true
		}
	}
	return false
}

func statisticsOf(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "mariadb", meta.ID)
	assert.Equal(t, "MariaDB", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Serve should advertise FetchSampleData")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	config := e2eConfig(t)
	delete(config, "host")
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheRealConfig(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), config)
	require.NoError(t, err)
}

func TestE2E_DiscoverEmitsTheDatabaseAsset(t *testing.T) {
	result := discover(t)

	db := requireAsset(t, result, "Database", "shop")
	assert.Equal(t, "mrn://database/mariadb/shop", *db.MRN)
	assert.Equal(t, []string{"MariaDB"}, db.Providers)
	assert.Equal(t, "shop", db.Metadata["database"])
	assert.Contains(t, db.Metadata["server_version"], "MariaDB")
	assert.Equal(t, "utf8mb4", db.Metadata["character_set"])
	assert.NotEmpty(t, db.Metadata["collation"])
}

func TestE2E_DiscoverEmitsEveryObjectWithItsType(t *testing.T) {
	result := discover(t)

	assert.NotNil(t, findAsset(result, "Table", "customers"))
	assert.NotNil(t, findAsset(result, "Table", "orders"))
	assert.NotNil(t, findAsset(result, "Table", "prices"))
	assert.NotNil(t, findAsset(result, "View", "customer_totals"))
	assert.NotNil(t, findAsset(result, "View", "vip_customers"))
	assert.NotNil(t, findAsset(result, "Sequence", "order_seq"))
}

func TestE2E_TableMetadataAndDescriptionComeFromInformationSchema(t *testing.T) {
	result := discover(t)

	orders := requireAsset(t, result, "Table", "orders")
	assert.Equal(t, "mrn://table/mariadb/orders", *orders.MRN)
	require.NotNil(t, orders.Description)
	assert.Equal(t, "Customer orders", *orders.Description)
	assert.Equal(t, "Customer orders", orders.Metadata["comment"])
	assert.Equal(t, "InnoDB", orders.Metadata["engine"])
	assert.Equal(t, "table", orders.Metadata["object_type"])
	assert.Equal(t, "shop", orders.Metadata["database"])
	assert.Equal(t, "orders", orders.Metadata["table_name"])
	assert.EqualValues(t, 3, orders.Metadata["row_count"])
	assert.EqualValues(t, 4, orders.Metadata["auto_increment"])
	assert.NotEmpty(t, orders.Metadata["collation"])
	assert.NotEmpty(t, orders.Metadata["created"])
	assert.Equal(t, false, orders.Metadata["system_versioned"])
}

func TestE2E_TemporaryFlagFollowsTheServerVersion(t *testing.T) {
	result := discover(t)

	// Older MariaDB servers have no TEMPORARY column in
	// information_schema.TABLES, and then no flag is better than a wrong
	// one. MariaDB 11.2 and later always expose it (recent 10.x
	// maintenance releases do too).
	version, _ := requireAsset(t, result, "Database", "shop").Metadata["server_version"].(string)
	orders := requireAsset(t, result, "Table", "orders")

	if serverAtLeast(t, version, 11, 2) {
		require.Contains(t, orders.Metadata, "temporary")
	}
	if flag, ok := orders.Metadata["temporary"]; ok {
		assert.Equal(t, false, flag)
	}
}

// serverAtLeast reads the major.minor of a VERSION() string such as
// "11.8.9-MariaDB-ubu2404".
func serverAtLeast(t *testing.T, version string, major, minor int) bool {
	t.Helper()
	parts := strings.SplitN(strings.SplitN(version, "-", 2)[0], ".", 3)
	require.GreaterOrEqual(t, len(parts), 2, "unexpected server version %q", version)

	gotMajor, err := strconv.Atoi(parts[0])
	require.NoError(t, err)
	gotMinor, err := strconv.Atoi(parts[1])
	require.NoError(t, err)

	return gotMajor > major || (gotMajor == major && gotMinor >= minor)
}

func TestE2E_SystemVersionedTableIsFlagged(t *testing.T) {
	result := discover(t)

	prices := requireAsset(t, result, "Table", "prices")
	assert.Equal(t, true, prices.Metadata["system_versioned"])
	assert.Equal(t, "table", prices.Metadata["object_type"])
}

func TestE2E_ColumnsCarryKeysDefaultsAndComments(t *testing.T) {
	result := discover(t)
	columns := columnsOf(t, requireAsset(t, result, "Table", "customers"))

	require.Contains(t, columns, "id")
	assert.Equal(t, true, columns["id"]["is_primary_key"])
	assert.Equal(t, true, columns["id"]["is_auto_increment"])
	assert.Equal(t, false, columns["id"]["is_nullable"])
	assert.Equal(t, "int(11)", columns["id"]["data_type"])
	assert.Equal(t, "Customer id", columns["id"]["description"])

	require.Contains(t, columns, "email")
	assert.Equal(t, "varchar(255)", columns["email"]["data_type"])
	assert.Equal(t, "Login email", columns["email"]["description"])
	assert.Equal(t, "utf8mb4", columns["email"]["character_set"])
	assert.NotEmpty(t, columns["email"]["collation"])

	require.Contains(t, columns, "created_at")
	assert.Equal(t, "current_timestamp()", columns["created_at"]["default_expression"])
}

func TestE2E_InvisibleAndGeneratedColumnsAreFlagged(t *testing.T) {
	result := discover(t)
	columns := columnsOf(t, requireAsset(t, result, "Table", "customers"))

	require.Contains(t, columns, "internal_note")
	assert.Equal(t, true, columns["internal_note"]["is_invisible"])
	assert.Equal(t, true, columns["internal_note"]["is_nullable"])

	require.Contains(t, columns, "email_domain")
	assert.Equal(t, true, columns["email_domain"]["is_generated"])

	// MariaDB stores JSON as LONGTEXT with a check constraint, so the
	// column type is reported as the server reports it.
	require.Contains(t, columns, "preferences")
	assert.Equal(t, "longtext", columns["preferences"]["data_type"])
}

func TestE2E_ViewCarriesItsDefinitionAndMetadata(t *testing.T) {
	result := discover(t)

	view := requireAsset(t, result, "View", "customer_totals")
	assert.Equal(t, "mrn://view/mariadb/customer_totals", *view.MRN)
	assert.Nil(t, view.Description, "the literal VIEW comment is not a description")
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "`shop`.`orders`")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)
	assert.Equal(t, "view", view.Metadata["object_type"])
	assert.Equal(t, "NONE", view.Metadata["check_option"])
	assert.Equal(t, false, view.Metadata["is_updatable"])
	assert.Equal(t, "DEFINER", view.Metadata["security_type"])
	assert.NotEmpty(t, view.Metadata["definer"])

	columns := columnsOf(t, view)
	assert.Len(t, columns, 3)
	assert.Contains(t, columns, "total_spent")
}

func TestE2E_ViewOfEdgesRunFromTheBaseTablesIntoTheView(t *testing.T) {
	result := discover(t)

	// Same direction as the MongoDB plugin: the data flows from the base
	// table into the view, so the base is the source.
	assert.True(t, hasEdge(result, "VIEW_OF", "mrn://table/mariadb/orders", "mrn://view/mariadb/customer_totals"))
	assert.True(t, hasEdge(result, "VIEW_OF", "mrn://table/mariadb/customers", "mrn://view/mariadb/customer_totals"))
	assert.True(t, hasEdge(result, "VIEW_OF", "mrn://view/mariadb/customer_totals", "mrn://view/mariadb/vip_customers"))
	assert.False(t, hasEdge(result, "VIEW_OF", "mrn://view/mariadb/customer_totals", "mrn://table/mariadb/orders"),
		"the reversed edge must not be emitted")
}

func TestE2E_SequenceCarriesItsSettings(t *testing.T) {
	result := discover(t)

	seq := requireAsset(t, result, "Sequence", "order_seq")
	assert.Equal(t, "mrn://sequence/mariadb/order_seq", *seq.MRN)
	assert.Equal(t, "sequence", seq.Metadata["object_type"])
	assert.EqualValues(t, 1000, seq.Metadata["start_value"])
	assert.EqualValues(t, 1, seq.Metadata["increment"])
	assert.EqualValues(t, 1, seq.Metadata["minimum_value"])
	// JSON carries the number as a float64 on this side of the wire, so the
	// default maximum (2^63 - 2) can only be compared as one.
	assert.EqualValues(t, float64(9223372036854775806), seq.Metadata["maximum_value"])
	assert.EqualValues(t, 1000, seq.Metadata["cache_size"])
	assert.Equal(t, false, seq.Metadata["cycle_option"])
	assert.NotContains(t, seq.Schema, "columns", "a sequence's bookkeeping columns are not a schema")
}

func TestE2E_DatabaseContainsEveryObject(t *testing.T) {
	result := discover(t)

	db := "mrn://database/mariadb/shop"
	assert.True(t, hasEdge(result, "CONTAINS", db, "mrn://table/mariadb/customers"))
	assert.True(t, hasEdge(result, "CONTAINS", db, "mrn://table/mariadb/orders"))
	assert.True(t, hasEdge(result, "CONTAINS", db, "mrn://table/mariadb/prices"))
	assert.True(t, hasEdge(result, "CONTAINS", db, "mrn://view/mariadb/customer_totals"))
	assert.True(t, hasEdge(result, "CONTAINS", db, "mrn://view/mariadb/vip_customers"))
	assert.True(t, hasEdge(result, "CONTAINS", db, "mrn://sequence/mariadb/order_seq"))
}

func TestE2E_ForeignKeyBecomesALineageEdge(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "FOREIGN_KEY", "mrn://table/mariadb/orders", "mrn://table/mariadb/customers"))
}

func TestE2E_EveryEdgeEndpointIsADiscoveredAsset(t *testing.T) {
	result := discover(t)

	known := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		known[*a.MRN] = struct{}{}
	}
	for _, e := range result.Lineage {
		assert.Containsf(t, known, e.Source, "edge %s source %s was never created", e.Type, e.Source)
		assert.Containsf(t, known, e.Target, "edge %s target %s was never created", e.Type, e.Target)
	}
}

func TestE2E_EveryAssetMRNMatchesItsOwnFields(t *testing.T) {
	result := discover(t)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, "MariaDB", a.Providers[0])
	}
}

func TestE2E_TableStatisticsComeFromInformationSchema(t *testing.T) {
	result := discover(t)

	stats := statisticsOf(result, "mrn://table/mariadb/customers")
	assert.Equal(t, float64(3), stats["asset.row_count"])
	assert.Equal(t, float64(7), stats["asset.column_count"])
	assert.Greater(t, stats["asset.size_bytes"], float64(0))
}

func TestE2E_ViewsOnlyGetAColumnCount(t *testing.T) {
	result := discover(t)

	stats := statisticsOf(result, "mrn://view/mariadb/customer_totals")
	assert.Equal(t, map[string]float64{"asset.column_count": 3}, stats)
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	name := "customers"
	columns, rows, err := bin.FetchSampleData(t.Context(), config, &pluginsdk.Asset{
		Name:     &name,
		Type:     "Table",
		Metadata: map[string]interface{}{"database": config["database"], "table_name": name},
	})
	require.NoError(t, err)

	assert.Contains(t, columns, "email")
	assert.Contains(t, columns, "email_domain")
	assert.NotContains(t, columns, "internal_note", "SELECT * leaves invisible columns out")
	require.Len(t, rows, 3)
	assert.Contains(t, rows[0], "alice@example.com")
}

func TestE2E_FetchSampleDataReadsAView(t *testing.T) {
	config := e2eConfig(t)
	bin := buildBinary(t)

	name := "customer_totals"
	columns, rows, err := bin.FetchSampleData(t.Context(), config, &pluginsdk.Asset{
		Name:     &name,
		Type:     "View",
		Metadata: map[string]interface{}{"database": config["database"], "table_name": name},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"customer_id", "email", "total_spent"}, columns)
	assert.Len(t, rows, 2, "only two customers placed orders")
}
