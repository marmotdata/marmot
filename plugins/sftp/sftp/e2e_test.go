package sftp_test

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real SFTP server. Numbers come
// back as float64 because the result crosses the wire as JSON.
//
// Start the server and seed it with:
//
//	docker run -d --name marmot-test-sftp -p 12222:2222 \
//	  -e PUID=1000 -e PGID=1000 -e USER_NAME=marmot \
//	  -e USER_PASSWORD=marmotpass -e PASSWORD_ACCESS=true -e SUDO_ACCESS=false \
//	  linuxserver/openssh-server:latest
//
// then run with MARMOT_TEST_SFTP_HOST=localhost MARMOT_TEST_SFTP_PORT=12222
// MARMOT_TEST_SFTP_USER=marmot MARMOT_TEST_SFTP_PASSWORD=marmotpass.

// testRoot is the directory seeded on the server. Asset names are
// relative to it.
const testRoot = "/config/data"

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// requireServer skips unless a live SFTP server was pointed at.
func requireServer(t *testing.T) {
	t.Helper()

	if os.Getenv("MARMOT_TEST_SFTP_HOST") == "" {
		t.Skip("set MARMOT_TEST_SFTP_HOST, MARMOT_TEST_SFTP_PORT, MARMOT_TEST_SFTP_USER and MARMOT_TEST_SFTP_PASSWORD to run the SFTP end to end tests")
	}
}

func e2eConfig(t *testing.T, overrides pluginsdk.RawConfig) pluginsdk.RawConfig {
	t.Helper()

	port := 22
	if raw := os.Getenv("MARMOT_TEST_SFTP_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err)
		port = parsed
	}

	config := pluginsdk.RawConfig{
		"host":             os.Getenv("MARMOT_TEST_SFTP_HOST"),
		"port":             port,
		"username":         os.Getenv("MARMOT_TEST_SFTP_USER"),
		"password":         os.Getenv("MARMOT_TEST_SFTP_PASSWORD"),
		"root_directories": []string{testRoot},
	}
	for key, value := range overrides {
		config[key] = value
	}
	return config
}

func discover(t *testing.T, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := buildBinary(t).Discover(t.Context(), e2eConfig(t, overrides))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func e2eAsset(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	for _, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	t.Fatalf("no %s asset named %q", assetType, name)
	return pluginsdk.Asset{}
}

func e2eAssetNames(result *pluginsdk.DiscoveryResult, assetType string) []string {
	var names []string
	for _, a := range result.Assets {
		if a.Type == assetType && a.Name != nil {
			names = append(names, *a.Name)
		}
	}
	return names
}

func e2eColumn(t *testing.T, asset pluginsdk.Asset, name string) map[string]any {
	t.Helper()

	encoded, ok := asset.Schema["columns"]
	require.True(t, ok, "asset %q has no columns", *asset.Name)

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(encoded), &columns))

	for _, column := range columns {
		if column["column_name"] == name {
			return column
		}
	}
	t.Fatalf("asset %q has no column %q", *asset.Name, name)
	return nil
}

func e2eStatistic(t *testing.T, result *pluginsdk.DiscoveryResult, assetMRN, metric string) float64 {
	t.Helper()

	for _, s := range result.Statistics {
		if s.AssetMRN == assetMRN && s.MetricName == metric {
			return s.Value
		}
	}
	t.Fatalf("no %s statistic for %s", metric, assetMRN)
	return 0
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "sftp", meta.ID)
	assert.Equal(t, "SFTP", meta.Name)
	assert.Equal(t, "storage", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "the plugin implements DataFetcher")
}

func TestE2E_ValidateWithoutAHostFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"username": "marmot", "password": "x"})

	require.Error(t, err)
}

func TestE2E_ValidateWithoutAnyAuthFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"host": "localhost", "username": "marmot"})

	require.Error(t, err)
}

func TestE2E_ValidateAgainstTheRealServer(t *testing.T) {
	requireServer(t)

	_, err := buildBinary(t).Validate(t.Context(), e2eConfig(t, nil))

	require.NoError(t, err)
}

func TestE2E_DiscoversTheFolderTree(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)

	assert.ElementsMatch(t,
		[]string{"archive", "archive/2026-08", "incoming", "incoming/2026-09", "staging"},
		e2eAssetNames(result, "Folder"))
}

func TestE2E_DiscoversTheFiles(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)

	assert.ElementsMatch(t, []string{
		"archive/2026-08/orders.csv",
		"incoming/2026-09/events.jsonl",
		"incoming/2026-09/orders.csv",
		"incoming/notes.txt",
	}, e2eAssetNames(result, "File"))
}

func TestE2E_FileMetadataComesFromTheServer(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	assert.Equal(t, testRoot+"/incoming/2026-09/orders.csv", orders.Metadata["path"])
	assert.Equal(t, "incoming/2026-09", orders.Metadata["directory"])
	assert.Equal(t, float64(155), orders.Metadata["size"])
	assert.Equal(t, "csv", orders.Metadata["file_type"])
	assert.Equal(t, "text/csv", orders.Metadata["mime_type"])
	assert.Equal(t, "-rw-r--r--", orders.Metadata["mode"])
	assert.NotEmpty(t, orders.Metadata["modified"])
}

func TestE2E_FileOwnerComesFromTheServer(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	// The container seeds the tree as uid and gid 1000.
	assert.Equal(t, float64(1000), orders.Metadata["owner_uid"])
	assert.Equal(t, float64(1000), orders.Metadata["owner_gid"])
}

func TestE2E_FolderMetadataCountsItsContents(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	incoming := e2eAsset(t, result, "Folder", "incoming")

	assert.Equal(t, testRoot+"/incoming", incoming.Metadata["path"])
	assert.Equal(t, testRoot, incoming.Metadata["root"])
	assert.Equal(t, float64(1), incoming.Metadata["file_count"])
	assert.Equal(t, float64(1), incoming.Metadata["directory_count"])
	assert.NotEmpty(t, incoming.Metadata["modified"])
}

func TestE2E_AnEmptyFolderIsStillCatalogued(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	staging := e2eAsset(t, result, "Folder", "staging")

	assert.Equal(t, float64(0), staging.Metadata["file_count"])
	assert.Equal(t, float64(0), staging.Metadata["directory_count"])
}

func TestE2E_InfersCsvColumns(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	assert.Equal(t, "INT", e2eColumn(t, orders, "order_id")["data_type"])
	assert.Equal(t, "STRING", e2eColumn(t, orders, "customer")["data_type"])
	assert.Equal(t, "FLOAT", e2eColumn(t, orders, "amount")["data_type"])
	assert.Equal(t, "DATETIME", e2eColumn(t, orders, "ordered_on")["data_type"])
	assert.Equal(t, "BOOLEAN", e2eColumn(t, orders, "shipped")["data_type"])
}

func TestE2E_MarksTheColumnWithAnEmptyCellNullable(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	assert.Equal(t, true, e2eColumn(t, orders, "note")["is_nullable"])
	assert.Equal(t, false, e2eColumn(t, orders, "order_id")["is_nullable"])
}

func TestE2E_InfersNestedJsonColumns(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	events := e2eAsset(t, result, "File", "incoming/2026-09/events.jsonl")

	assert.Equal(t, "object", e2eColumn(t, events, "actor")["data_type"])
	assert.Equal(t, "string", e2eColumn(t, events, "actor.id")["data_type"])
	assert.Equal(t, "string|null", e2eColumn(t, events, "actor.name")["data_type"])
	assert.Equal(t, float64(1), e2eColumn(t, events, "score")["occurrence"])
}

func TestE2E_APlainFileIsCataloguedWithoutColumns(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	notes := e2eAsset(t, result, "File", "incoming/notes.txt")

	assert.Equal(t, "other", notes.Metadata["file_type"])
	assert.Empty(t, notes.Schema["columns"])
}

func TestE2E_StructuredOnlyDropsThePlainFile(t *testing.T) {
	requireServer(t)

	result := discover(t, pluginsdk.RawConfig{"structured_only": true})

	assert.NotContains(t, e2eAssetNames(result, "File"), "incoming/notes.txt")
	assert.Contains(t, e2eAssetNames(result, "File"), "incoming/2026-09/orders.csv")
	assert.Contains(t, e2eAssetNames(result, "Folder"), "incoming")
}

func TestE2E_TheSymlinkDoesNotLoopOrDuplicate(t *testing.T) {
	requireServer(t)

	// /config/data/loop points back at /config/data. Following it without
	// a guard would walk the tree again, and again, forever.
	result := discover(t, pluginsdk.RawConfig{"follow_symlinks": true})

	assert.ElementsMatch(t,
		[]string{"archive", "archive/2026-08", "incoming", "incoming/2026-09", "staging"},
		e2eAssetNames(result, "Folder"))
}

func TestE2E_TheSymlinkIsSkippedByDefault(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)

	assert.NotContains(t, e2eAssetNames(result, "Folder"), "loop")
}

func TestE2E_LinksFoldersAndFilesWithContains(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)

	edges := make(map[string]bool, len(result.Lineage))
	for _, e := range result.Lineage {
		assert.Equal(t, "CONTAINS", e.Type)
		edges[e.Source+" -> "+e.Target] = true
	}

	assert.True(t, edges["mrn://folder/sftp/incoming -> mrn://folder/sftp/incoming-2026-09"])
	assert.True(t, edges["mrn://folder/sftp/incoming-2026-09 -> mrn://file/sftp/incoming-2026-09-orders.csv"])
	assert.True(t, edges["mrn://folder/sftp/archive -> mrn://folder/sftp/archive-2026-08"])
	assert.True(t, edges["mrn://folder/sftp/incoming -> mrn://file/sftp/incoming-notes.txt"])
}

func TestE2E_EveryAssetMRNMatchesItsOwnFields(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.Equal(t, []string{"SFTP"}, a.Providers)
	}

	assert.Equal(t, "mrn://file/sftp/incoming-2026-09-orders.csv",
		*e2eAsset(t, result, "File", "incoming/2026-09/orders.csv").MRN)
	assert.Equal(t, "mrn://folder/sftp/incoming-2026-09",
		*e2eAsset(t, result, "Folder", "incoming/2026-09").MRN)
}

func TestE2E_EmitsFileStatistics(t *testing.T) {
	requireServer(t)

	result := discover(t, nil)
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	assert.Equal(t, float64(155), e2eStatistic(t, result, *orders.MRN, "asset.size_bytes"))
	assert.Equal(t, float64(3), e2eStatistic(t, result, *orders.MRN, "asset.row_count"))
	assert.Equal(t, float64(6), e2eStatistic(t, result, *orders.MRN, "asset.column_count"))
	assert.Equal(t, true, orders.Metadata["row_count_exact"])
}

func TestE2E_MarksARowCountInexactWhenTheReadIsCapped(t *testing.T) {
	requireServer(t)

	result := discover(t, pluginsdk.RawConfig{"max_read_bytes": 80})
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	assert.Equal(t, false, orders.Metadata["row_count_exact"])
	assert.Less(t, e2eStatistic(t, result, *orders.MRN, "asset.row_count"), float64(3))
}

func TestE2E_StopsAtMaxDepth(t *testing.T) {
	requireServer(t)

	result := discover(t, pluginsdk.RawConfig{"max_depth": 1})

	assert.ElementsMatch(t, []string{"archive", "incoming", "staging"}, e2eAssetNames(result, "Folder"))
}

func TestE2E_FetchSampleDataForACsvFile(t *testing.T) {
	requireServer(t)

	bin := buildBinary(t)
	result, err := bin.Discover(t.Context(), e2eConfig(t, nil))
	require.NoError(t, err)
	orders := e2eAsset(t, result, "File", "incoming/2026-09/orders.csv")

	names, rows, err := bin.FetchSampleData(t.Context(), e2eConfig(t, nil), &orders)

	require.NoError(t, err)
	assert.Equal(t, []string{"order_id", "customer", "amount", "ordered_on", "shipped", "note"}, names)
	require.Len(t, rows, 3)
	assert.Equal(t, "alice", rows[0][1])
	assert.Equal(t, "", rows[1][5], "the empty cell survives the round trip")
}

func TestE2E_FetchSampleDataForAJsonLinesFile(t *testing.T) {
	requireServer(t)

	bin := buildBinary(t)
	result, err := bin.Discover(t.Context(), e2eConfig(t, nil))
	require.NoError(t, err)
	events := e2eAsset(t, result, "File", "incoming/2026-09/events.jsonl")

	names, rows, err := bin.FetchSampleData(t.Context(), e2eConfig(t, nil), &events)

	require.NoError(t, err)
	assert.Equal(t, []string{"actor", "actor.id", "actor.name", "event_id", "kind", "score", "ts"}, names)
	require.Len(t, rows, 3)
	assert.Equal(t, "alice", rows[0][2])
	assert.Equal(t, float64(1), rows[0][3])
	assert.Nil(t, rows[0][5], "score is only on the second event")
}

func TestE2E_FetchSampleDataRefusesAnUnsupportedFile(t *testing.T) {
	requireServer(t)

	bin := buildBinary(t)
	result, err := bin.Discover(t.Context(), e2eConfig(t, nil))
	require.NoError(t, err)
	notes := e2eAsset(t, result, "File", "incoming/notes.txt")

	_, _, err = bin.FetchSampleData(t.Context(), e2eConfig(t, nil), &notes)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "csv, tsv, json and jsonl")
}

func TestE2E_FetchSampleDataRefusesAFolder(t *testing.T) {
	requireServer(t)

	bin := buildBinary(t)
	result, err := bin.Discover(t.Context(), e2eConfig(t, nil))
	require.NoError(t, err)
	incoming := e2eAsset(t, result, "Folder", "incoming")

	_, _, err = bin.FetchSampleData(t.Context(), e2eConfig(t, nil), &incoming)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "only available for files")
}

func TestE2E_ConnectingWithTheWrongPasswordFails(t *testing.T) {
	requireServer(t)

	_, err := buildBinary(t).Discover(t.Context(), e2eConfig(t, pluginsdk.RawConfig{"password": "wrong"}))

	require.Error(t, err)
}

func TestE2E_AMissingRootProducesNoAssets(t *testing.T) {
	requireServer(t)

	result := discover(t, pluginsdk.RawConfig{"root_directories": []string{"/config/nowhere"}})

	assert.Empty(t, result.Assets)
}
