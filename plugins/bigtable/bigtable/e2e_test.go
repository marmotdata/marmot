package bigtable_test

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
	"time"

	bt "cloud.google.com/go/bigtable"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses: plugintest.Build compiles the main
// package and every call spawns the process, runs one RPC and kills it
// again. They need a Bigtable emulator:
//
//	docker run -d --name marmot-test-bigtable -p 18086:8086 \
//	  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
//	  gcloud beta emulators bigtable start --host-port=0.0.0.0:8086
//	MARMOT_TEST_BIGTABLE_EMULATOR=localhost:18086 go test ./...
const (
	e2eProject  = "test-project"
	e2eInstance = "test-instance"
)

func emulatorHost(t *testing.T) string {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_BIGTABLE_EMULATOR")
	if host == "" {
		t.Skip("MARMOT_TEST_BIGTABLE_EMULATOR is not set, skipping the emulator end to end tests")
	}
	return host
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func emulatorOptions(host string) []option.ClientOption {
	return []option.ClientOption{
		option.WithEndpoint(host),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
}

func baseConfig(host string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"project_id":    e2eProject,
		"instances":     []string{e2eInstance},
		"emulator_host": host,
	}
}

// seed creates the fixture tables and rows. It drops the tables first so
// a second run against the same emulator starts from the same state.
func seed(t *testing.T, host string) {
	t.Helper()

	ctx := t.Context()

	admin, err := bt.NewAdminClient(ctx, e2eProject, e2eInstance, emulatorOptions(host)...)
	require.NoError(t, err)
	defer admin.Close()

	for _, table := range []string{"events", "users"} {
		if err := admin.DeleteTable(ctx, table); err != nil && status.Code(err) != codes.NotFound {
			require.NoError(t, err)
		}
	}

	require.NoError(t, admin.CreateTableFromConf(ctx, &bt.TableConf{
		TableID: "events",
		ColumnFamilies: map[string]bt.Family{
			"d": {GCPolicy: bt.MaxVersionsPolicy(3)},
			"m": {GCPolicy: bt.MaxAgePolicy(24 * time.Hour)},
		},
	}))

	// users stays empty so discovery has to cope with a table no sample
	// can say anything about.
	require.NoError(t, admin.CreateTableFromConf(ctx, &bt.TableConf{
		TableID:        "users",
		ColumnFamilies: map[string]bt.Family{"p": {GCPolicy: bt.MaxVersionsPolicy(1)}},
	}))

	client, err := bt.NewClientWithConfig(ctx, e2eProject, e2eInstance,
		bt.ClientConfig{MetricsProvider: bt.NoopMetricsProvider{}}, emulatorOptions(host)...)
	require.NoError(t, err)
	defer client.Close()

	events := client.Open("events")
	now := bt.Now()

	for _, row := range []struct {
		key   string
		name  string
		count uint64
	}{
		{"event#1", "checkout", 7},
		{"event#2", "signup", 12},
		{"event#3", "refund", 1},
	} {
		encoded := make([]byte, 8)
		binary.BigEndian.PutUint64(encoded, row.count)

		mutation := bt.NewMutation()
		mutation.Set("d", "name", now, []byte(row.name))
		mutation.Set("m", "count", now, encoded)
		require.NoError(t, events.Apply(ctx, row.key, mutation))
	}

	// One row also carries a binary blob, so the sample has a column that
	// is neither text nor a counter.
	blob := bt.NewMutation()
	blob.Set("d", "payload", now, []byte{0x00, 0x01, 0xff, 0xfe, 0x7f, 0x80, 0x03})
	require.NoError(t, events.Apply(ctx, "event#1", blob))
}

func discover(t *testing.T, bin plugintest.Binary, config pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func assetByMRN(t *testing.T, result *pluginsdk.DiscoveryResult, value string) pluginsdk.Asset {
	t.Helper()

	for _, a := range result.Assets {
		if a.MRN != nil && *a.MRN == value {
			return a
		}
	}
	t.Fatalf("no asset with MRN %s", value)
	return pluginsdk.Asset{}
}

func columnsOf(t *testing.T, asset pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	require.True(t, ok, "asset has no columns in its schema")

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]any, len(columns))
	for _, c := range columns {
		name, _ := c["column_name"].(string)
		byName[name] = c
	}
	return byName
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "bigtable", meta.ID)
	assert.Equal(t, "Bigtable", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "the plugin implements DataFetcher")
}

func TestE2E_ValidateMissingProjectFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})

	require.Error(t, err)
}

func TestE2E_ValidateEmulatorWithoutInstancesFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{
		"project_id":    e2eProject,
		"emulator_host": "localhost:18086",
	})

	require.Error(t, err)
}

func TestE2E_DiscoversTheInstance(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	instance := assetByMRN(t, result, "mrn://instance/bigtable/test-instance")
	assert.Equal(t, "Instance", instance.Type)
	assert.Equal(t, []string{"Bigtable"}, instance.Providers)
	assert.Equal(t, e2eProject, instance.Metadata["project_id"])
	assert.Equal(t, true, instance.Metadata["emulator"])
	assert.Equal(t, float64(2), instance.Metadata["table_count"])
}

func TestE2E_DiscoversTheTables(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	events := assetByMRN(t, result, "mrn://table/bigtable/test-instance.events")
	require.NotNil(t, events.Name)
	assert.Equal(t, "test-instance.events", *events.Name)
	assert.Equal(t, "Table", events.Type)
	assert.Equal(t, "events", events.Metadata["table_name"])
	assert.Equal(t, e2eInstance, events.Metadata["instance_id"])

	users := assetByMRN(t, result, "mrn://table/bigtable/test-instance.users")
	assert.Equal(t, "users", users.Metadata["table_name"])
}

func TestE2E_EveryAssetsMRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from Type, Providers[0] and Name, so an
	// MRN that disagrees would create a second asset on ingest.
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestE2E_ReportsColumnFamiliesAndGCPolicies(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	events := assetByMRN(t, result, "mrn://table/bigtable/test-instance.events")
	assert.ElementsMatch(t, []any{"d", "m"}, events.Metadata["column_families"])

	policies, ok := events.Metadata["gc_policies"].(map[string]any)
	require.True(t, ok, "gc_policies should be a map, got %T", events.Metadata["gc_policies"])
	assert.Equal(t, "versions() > 3", policies["d"])
	assert.Equal(t, "age() > 1d", policies["m"])
}

func TestE2E_InfersColumnsFromSampledRows(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	events := assetByMRN(t, result, "mrn://table/bigtable/test-instance.events")
	columns := columnsOf(t, events)

	require.Contains(t, columns, "d:name")
	assert.Equal(t, "bytes", columns["d:name"]["data_type"])
	assert.Equal(t, "d", columns["d:name"]["column_family"])
	assert.Equal(t, "name", columns["d:name"]["qualifier"])
	assert.Equal(t, "text", columns["d:name"]["inferred_type"])
	assert.Equal(t, float64(3), columns["d:name"]["occurrence"], "all three rows carry a name")

	require.Contains(t, columns, "m:count")
	assert.Equal(t, "int64", columns["m:count"]["inferred_type"])

	require.Contains(t, columns, "d:payload")
	assert.Equal(t, "binary", columns["d:payload"]["inferred_type"])
	assert.Equal(t, float64(1), columns["d:payload"]["occurrence"], "only one row carries a payload")

	require.Contains(t, columns, "row_key")
	assert.Equal(t, true, columns["row_key"]["is_primary_key"])

	assert.Equal(t, float64(3), events.Metadata["sampled_rows"])
}

func TestE2E_LeavesAnEmptyTableWithOnlyTheRowKey(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	users := assetByMRN(t, result, "mrn://table/bigtable/test-instance.users")
	columns := columnsOf(t, users)

	assert.Len(t, columns, 1)
	assert.Contains(t, columns, "row_key")
	assert.Equal(t, float64(0), users.Metadata["sampled_rows"])
}

func TestE2E_LinksTheInstanceToItsTables(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://instance/bigtable/test-instance",
		Target: "mrn://table/bigtable/test-instance.events",
		Type:   "CONTAINS",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://instance/bigtable/test-instance",
		Target: "mrn://table/bigtable/test-instance.users",
		Type:   "CONTAINS",
	})
}

func TestE2E_ReportsColumnCounts(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	assert.Contains(t, result.Statistics, pluginsdk.Statistic{
		AssetMRN:   "mrn://table/bigtable/test-instance.events",
		MetricName: "asset.column_count",
		Value:      4,
	})
	assert.Contains(t, result.Statistics, pluginsdk.Statistic{
		AssetMRN:   "mrn://table/bigtable/test-instance.users",
		MetricName: "asset.column_count",
		Value:      1,
	})
}

func TestE2E_CountsRowsWhenStatisticsAreAskedFor(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	config := baseConfig(host)
	config["include_statistics"] = true

	result := discover(t, bin, config)

	assert.Contains(t, result.Statistics, pluginsdk.Statistic{
		AssetMRN:   "mrn://table/bigtable/test-instance.events",
		MetricName: "asset.row_count",
		Value:      3,
	})
}

func TestE2E_SkipsARowCountThatHitsTheLimit(t *testing.T) {
	// A count that stopped at the limit is a floor, not a row count, so
	// nothing is reported rather than something misleading.
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	config := baseConfig(host)
	config["include_statistics"] = true
	config["max_count_rows"] = 1

	result := discover(t, bin, config)

	for _, s := range result.Statistics {
		if s.AssetMRN == "mrn://table/bigtable/test-instance.events" {
			assert.NotEqual(t, "asset.row_count", s.MetricName)
		}
	}
}

func TestE2E_SkipsSamplingWhenColumnsAreTurnedOff(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	config := baseConfig(host)
	config["include_columns"] = false

	result := discover(t, bin, config)

	events := assetByMRN(t, result, "mrn://table/bigtable/test-instance.events")
	assert.Len(t, columnsOf(t, events), 1, "only the row key, which needs no sample")
	assert.NotContains(t, events.Metadata, "sampled_rows")
}

func TestE2E_LeavesOutTheConsoleLinkForAnEmulator(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))

	events := assetByMRN(t, result, "mrn://table/bigtable/test-instance.events")
	assert.Empty(t, events.ExternalLinks)
	assert.NotContains(t, events.Metadata, "url")
}

func TestE2E_FetchSampleDataOverTheWire(t *testing.T) {
	host := emulatorHost(t)
	seed(t, host)
	bin := buildBinary(t)

	result := discover(t, bin, baseConfig(host))
	events := assetByMRN(t, result, "mrn://table/bigtable/test-instance.events")

	columns, rows, err := bin.FetchSampleData(t.Context(), baseConfig(host), &events)
	require.NoError(t, err)

	assert.Equal(t, []string{"row_key", "d:name", "d:payload", "m:count"}, columns)
	require.Len(t, rows, 3)

	// The first row is event#1, the one carrying the binary payload, which
	// is shown base64 encoded because it is not readable text.
	require.Len(t, rows[0], 4)
	assert.Equal(t, "event#1", rows[0][0])
	assert.Equal(t, "checkout", rows[0][1])
	assert.Equal(t, "AAH//n+AAw==", rows[0][2])
}

func TestE2E_DiscoverFailsWhenTheEmulatorIsUnreachable(t *testing.T) {
	// An unreachable system is a real failure, not something to log and
	// carry on from, because the alternative is a silently empty run.
	emulatorHost(t)
	bin := buildBinary(t)

	config := baseConfig("localhost:1")
	_, err := bin.Discover(t.Context(), config)

	require.Error(t, err)
}
