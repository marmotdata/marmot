package sftp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// discoverTree runs the whole discovery pipeline over a tree on disk.
func discoverTree(t *testing.T, root string, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	s := &Source{config: testConfig(t, root, overrides)}
	return s.collect(context.Background(), osFS{})
}

func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	for _, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	t.Fatalf("no %s asset named %q", assetType, name)
	return pluginsdk.Asset{}
}

func hasAsset(result *pluginsdk.DiscoveryResult, assetType, name string) bool {
	for _, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return true
		}
	}
	return false
}

func columnsOf(t *testing.T, asset pluginsdk.Asset) []map[string]any {
	t.Helper()

	encoded, ok := asset.Schema["columns"]
	require.True(t, ok, "asset %q has no columns", *asset.Name)

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(encoded), &columns))
	return columns
}

func columnNamed(t *testing.T, asset pluginsdk.Asset, name string) map[string]any {
	t.Helper()

	for _, column := range columnsOf(t, asset) {
		if column["column_name"] == name {
			return column
		}
	}
	t.Fatalf("asset %q has no column %q", *asset.Name, name)
	return nil
}

func statistic(result *pluginsdk.DiscoveryResult, assetMRN, metric string) (float64, bool) {
	for _, s := range result.Statistics {
		if s.AssetMRN == assetMRN && s.MetricName == metric {
			return s.Value, true
		}
	}
	return 0, false
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == "CONTAINS" {
			return true
		}
	}
	return false
}

func TestCollect_CreatesAFolderPerDirectory(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	for _, name := range []string{"incoming", "incoming/2026-09", "archive", "archive/2026-08", "staging"} {
		assert.True(t, hasAsset(result, typeFolder, name), "expected a Folder named %q", name)
	}
}

func TestCollect_CreatesAFilePerFile(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	for _, name := range []string{
		"incoming/2026-09/orders.csv",
		"incoming/2026-09/events.jsonl",
		"incoming/notes.txt",
		"archive/2026-08/orders.csv",
	} {
		assert.True(t, hasAsset(result, typeFile, name), "expected a File named %q", name)
	}
}

func TestCollect_NamesEveryAssetWithTheSFTPProvider(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		assert.Equal(t, []string{"SFTP"}, a.Providers)
	}
}

func TestCollect_FolderMetadataDescribesItsContents(t *testing.T) {
	root := writeTree(t)
	result := discoverTree(t, root, nil)

	incoming := assetNamed(t, result, typeFolder, "incoming")
	assert.Equal(t, filepath.Join(root, "incoming"), incoming.Metadata["path"])
	assert.Equal(t, root, incoming.Metadata["root"])
	assert.Equal(t, 1, incoming.Metadata["file_count"])
	assert.Equal(t, 1, incoming.Metadata["directory_count"])
	assert.NotEmpty(t, incoming.Metadata["modified"])
}

func TestCollect_NestedFolderRecordsItsParent(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	assert.Equal(t, "incoming", assetNamed(t, result, typeFolder, "incoming/2026-09").Metadata["parent"])
}

func TestCollect_FileMetadataDescribesTheFile(t *testing.T) {
	root := writeTree(t)
	result := discoverTree(t, root, nil)

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")
	assert.Equal(t, filepath.Join(root, "incoming", "2026-09", "orders.csv"), orders.Metadata["path"])
	assert.Equal(t, "incoming/2026-09", orders.Metadata["directory"])
	assert.Equal(t, int64(len(ordersCSV)), orders.Metadata["size"])
	assert.Equal(t, "csv", orders.Metadata["extension"])
	assert.Equal(t, "text/csv", orders.Metadata["mime_type"])
	assert.Equal(t, "csv", orders.Metadata["file_type"])
	assert.Equal(t, "sftp.example.com", orders.Metadata["host"])
	assert.Equal(t, 22, orders.Metadata["port"])
	assert.NotEmpty(t, orders.Metadata["modified"])
}

func TestCollect_CataloguesAPlainFileWithoutColumns(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	notes := assetNamed(t, result, typeFile, "incoming/notes.txt")
	assert.Equal(t, "other", notes.Metadata["file_type"])
	assert.Empty(t, notes.Schema["columns"])
}

func TestCollect_StructuredOnlyDropsAPlainFile(t *testing.T) {
	result := discoverTree(t, writeTree(t), pluginsdk.RawConfig{"structured_only": true})

	assert.False(t, hasAsset(result, typeFile, "incoming/notes.txt"))
	assert.True(t, hasAsset(result, typeFile, "incoming/2026-09/orders.csv"))
}

func TestCollect_InfersDelimitedColumns(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")

	assert.Equal(t, "INT", columnNamed(t, orders, "order_id")["data_type"])
	assert.Equal(t, "STRING", columnNamed(t, orders, "customer")["data_type"])
	assert.Equal(t, "FLOAT", columnNamed(t, orders, "amount")["data_type"])
	assert.Equal(t, "DATETIME", columnNamed(t, orders, "ordered_on")["data_type"])
	assert.Equal(t, "BOOLEAN", columnNamed(t, orders, "shipped")["data_type"])
}

func TestCollect_MarksAColumnWithAnEmptyCellNullable(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")

	assert.Equal(t, true, columnNamed(t, orders, "note")["is_nullable"])
	assert.Equal(t, false, columnNamed(t, orders, "order_id")["is_nullable"])
}

func TestCollect_InfersNestedJSONColumns(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	events := assetNamed(t, result, typeFile, "incoming/2026-09/events.jsonl")

	assert.Equal(t, "object", columnNamed(t, events, "actor")["data_type"])
	assert.Equal(t, "string", columnNamed(t, events, "actor.id")["data_type"])
	assert.Equal(t, "string|null", columnNamed(t, events, "actor.name")["data_type"])
	assert.Equal(t, "number", columnNamed(t, events, "event_id")["data_type"])
}

func TestCollect_RecordsHowOftenAJSONKeyAppeared(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	events := assetNamed(t, result, typeFile, "incoming/2026-09/events.jsonl")

	// score is only on the second event.
	assert.Equal(t, float64(1), columnNamed(t, events, "score")["occurrence"])
	assert.Equal(t, true, columnNamed(t, events, "score")["is_nullable"])
	assert.Equal(t, float64(3), columnNamed(t, events, "kind")["occurrence"])
}

func TestCollect_SkipsColumnsWhenIncludeColumnsIsOff(t *testing.T) {
	result := discoverTree(t, writeTree(t), pluginsdk.RawConfig{"include_columns": false})

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")
	assert.Empty(t, orders.Schema["columns"])
}

func TestCollect_EmitsASizeStatisticForEveryFile(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	notes := assetNamed(t, result, typeFile, "incoming/notes.txt")
	size, ok := statistic(result, *notes.MRN, "asset.size_bytes")

	require.True(t, ok)
	assert.Equal(t, float64(len("operational notes for the incoming feed\n")), size)
}

func TestCollect_EmitsRowAndColumnCountsForADelimitedFile(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")

	rows, ok := statistic(result, *orders.MRN, "asset.row_count")
	require.True(t, ok)
	assert.Equal(t, float64(3), rows, "the header is not a data row")

	columns, ok := statistic(result, *orders.MRN, "asset.column_count")
	require.True(t, ok)
	assert.Equal(t, float64(6), columns)
}

func TestCollect_MarksARowCountExactWhenTheWholeFileWasRead(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")
	assert.Equal(t, true, orders.Metadata["row_count_exact"])
}

func TestCollect_MarksARowCountInexactWhenTheReadWasCapped(t *testing.T) {
	result := discoverTree(t, writeTree(t), pluginsdk.RawConfig{"max_read_bytes": 80})

	orders := assetNamed(t, result, typeFile, "incoming/2026-09/orders.csv")
	assert.Equal(t, false, orders.Metadata["row_count_exact"])

	rows, ok := statistic(result, *orders.MRN, "asset.row_count")
	require.True(t, ok)
	assert.Less(t, rows, float64(3))
}

func TestCollect_EmitsNoStatisticsWhenTheyAreTurnedOff(t *testing.T) {
	result := discoverTree(t, writeTree(t), pluginsdk.RawConfig{"include_statistics": false})

	assert.Empty(t, result.Statistics)
}

func TestCollect_LinksAFolderToTheFolderAboveIt(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	assert.True(t, hasEdge(result,
		assetMRN(typeFolder, "incoming"),
		assetMRN(typeFolder, "incoming/2026-09")))
}

func TestCollect_LinksAFileToItsFolder(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	assert.True(t, hasEdge(result,
		assetMRN(typeFolder, "incoming/2026-09"),
		assetMRN(typeFile, "incoming/2026-09/orders.csv")))
}

func TestCollect_DoesNotLinkTheTopOfARootToAnything(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	incoming := assetMRN(typeFolder, "incoming")
	for _, e := range result.Lineage {
		assert.NotEqual(t, incoming, e.Target, "the root is not an asset, so nothing contains %q", incoming)
	}
}

func TestCollect_EmitsOnlyContainsEdges(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	require.NotEmpty(t, result.Lineage)
	for _, e := range result.Lineage {
		assert.Equal(t, "CONTAINS", e.Type)
	}
}

func TestCollect_EveryEdgeEndPointIsAnAssetInTheSameRun(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	known := make(map[string]bool, len(result.Assets))
	for _, a := range result.Assets {
		known[*a.MRN] = true
	}

	for _, e := range result.Lineage {
		assert.True(t, known[e.Source], "edge source %q is not an asset in this run", e.Source)
		assert.True(t, known[e.Target], "edge target %q is not an asset in this run", e.Target)
	}
}

func TestCollect_RecordsTheSourcePropertiesOnEveryAsset(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	for _, a := range result.Assets {
		require.Len(t, a.Sources, 1)
		assert.Equal(t, "SFTP", a.Sources[0].Name)
		assert.Equal(t, a.Metadata, a.Sources[0].Properties)
	}
}

func TestCollect_AppliesConfiguredTags(t *testing.T) {
	result := discoverTree(t, writeTree(t), pluginsdk.RawConfig{"tags": []string{"drop-zone"}})

	assert.Contains(t, assetNamed(t, result, typeFolder, "incoming").Tags, "drop-zone")
}

func TestFetchSample_ReadsTheHeadOfADelimitedFile(t *testing.T) {
	root := writeTree(t)

	names, rows, err := fetchSample(osFS{}, filepath.Join(root, "incoming", "2026-09", "orders.csv"), kindCSV, 1<<20)

	require.NoError(t, err)
	assert.Equal(t, []string{"order_id", "customer", "amount", "ordered_on", "shipped", "note"}, names)
	require.Len(t, rows, 3)
	assert.Equal(t, "alice", rows[0][1])
}

func TestFetchSample_ReadsTheHeadOfAJSONLinesFile(t *testing.T) {
	root := writeTree(t)

	names, rows, err := fetchSample(osFS{}, filepath.Join(root, "incoming", "2026-09", "events.jsonl"), kindJSONLines, 1<<20)

	require.NoError(t, err)
	assert.Contains(t, names, "actor.name")
	require.Len(t, rows, 3)

	actorName := rows[0][indexOf(names, "actor.name")]
	assert.Equal(t, "alice", actorName)
}

func TestFetchSample_StopsAtTwentyRecords(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	b.WriteString("id\n")
	for i := 0; i < 50; i++ {
		b.WriteString("1\n")
	}
	writeFile(t, root, "big.csv", b.String())

	_, rows, err := fetchSample(osFS{}, filepath.Join(root, "big.csv"), kindCSV, 1<<20)

	require.NoError(t, err)
	assert.Len(t, rows, 20)
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}
