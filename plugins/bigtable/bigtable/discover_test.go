package bigtable

import (
	"context"
	"encoding/binary"
	"errors"

	"testing"

	bt "cloud.google.com/go/bigtable"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeReader replays a fixed set of rows the way a table would. The row
// limit is enforced by the Bigtable server, so a test supplies the rows
// the server would have returned rather than expecting the fake to cut
// them off.
type fakeReader struct {
	rows []bt.Row
	err  error
}

func (f *fakeReader) ReadRows(_ context.Context, _ bt.RowSet, fn func(bt.Row) bool, _ ...bt.ReadOption) error {
	if f.err != nil {
		return f.err
	}
	for _, row := range f.rows {
		if !fn(row) {
			break
		}
	}
	return nil
}

// testRow builds one row out of cells, keyed by column family the way the
// client hands rows back.
func testRow(key string, cells ...bt.ReadItem) bt.Row {
	row := bt.Row{}
	for _, cell := range cells {
		cell.Row = key
		family, _ := splitColumn(cell.Column)
		row[family] = append(row[family], cell)
	}
	return row
}

func textCell(column, value string) bt.ReadItem {
	return bt.ReadItem{Column: column, Value: []byte(value)}
}

func counterCell(column string, value uint64) bt.ReadItem {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, value)
	return bt.ReadItem{Column: column, Value: encoded}
}

func columnsByName(columns []column) map[string]column {
	byName := make(map[string]column, len(columns))
	for _, c := range columns {
		byName[c.Name] = c
	}
	return byName
}

func TestSampleColumns_PutsTheRowKeyFirst(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{testRow("user#1", textCell("d:name", "alice"))}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	require.NotEmpty(t, columns)
	assert.Equal(t, "row_key", columns[0].Name)
	assert.True(t, columns[0].PrimaryKey)
}

func TestSampleColumns_FindsEveryFamilyAndQualifier(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1", textCell("d:name", "alice"), counterCell("m:count", 3)),
		testRow("user#2", textCell("d:email", "bob@example.com")),
	}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	names := make([]string, 0, len(columns))
	for _, c := range columns {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"row_key", "d:email", "d:name", "m:count"}, names)
}

func TestSampleColumns_SplitsTheNameIntoFamilyAndQualifier(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{testRow("user#1", textCell("d:name", "alice"))}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	found := columnsByName(columns)["d:name"]
	assert.Equal(t, "d", found.ColumnFamily)
	assert.Equal(t, "name", found.Qualifier)
}

func TestSampleColumns_CountsTheRowsAColumnAppearedIn(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1", textCell("d:name", "alice")),
		testRow("user#2", textCell("d:name", "bob")),
		testRow("user#3", textCell("d:email", "carol@example.com")),
	}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	byName := columnsByName(columns)
	assert.Equal(t, 2, byName["d:name"].Occurrence)
	assert.Equal(t, 1, byName["d:email"].Occurrence)
}

func TestSampleColumns_CountsARowOnceWhenItHoldsSeveralCellsForAColumn(t *testing.T) {
	// Occurrence answers "how many rows have this column", so two cells in
	// one row still count once.
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1", textCell("d:name", "alice"), textCell("d:name", "alicia")),
	}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	assert.Equal(t, 1, columnsByName(columns)["d:name"].Occurrence)
}

func TestSampleColumns_InfersTheTypeOfEachColumn(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1",
			textCell("d:name", "alice"),
			counterCell("m:count", 7),
			bt.ReadItem{Column: "d:blob", Value: []byte{0x00, 0xff, 0x10}},
		),
	}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	byName := columnsByName(columns)
	assert.Equal(t, "text", byName["d:name"].InferredType)
	assert.Equal(t, "int64", byName["m:count"].InferredType)
	assert.Equal(t, "binary", byName["d:blob"].InferredType)
}

func TestSampleColumns_FallsBackToBinaryWhenRowsDisagreeOnAType(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1", textCell("d:mixed", "alice")),
		testRow("user#2", counterCell("d:mixed", 9)),
	}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	assert.Equal(t, "binary", columnsByName(columns)["d:mixed"].InferredType)
}

func TestSampleColumns_MarksSampledColumnsNullable(t *testing.T) {
	// Bigtable rows are sparse, so nothing found by sampling is guaranteed
	// to be in the next row.
	reader := &fakeReader{rows: []bt.Row{testRow("user#1", textCell("d:name", "alice"))}}

	columns, _, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	found := columnsByName(columns)["d:name"]
	assert.True(t, found.Nullable)
	assert.Equal(t, "bytes", found.DataType)
}

func TestSampleColumns_ReportsHowManyRowsItRead(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1", textCell("d:name", "alice")),
		testRow("user#2", textCell("d:name", "bob")),
	}}

	_, sampled, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	assert.Equal(t, 2, sampled)
}

func TestSampleColumns_LeavesAnEmptyTableWithOnlyTheRowKey(t *testing.T) {
	reader := &fakeReader{}

	columns, sampled, err := sampleColumns(t.Context(), reader, 100)

	require.NoError(t, err)
	require.Len(t, columns, 1)
	assert.Equal(t, "row_key", columns[0].Name)
	assert.Zero(t, sampled)
}

func TestSampleColumns_ReturnsTheReadError(t *testing.T) {
	reader := &fakeReader{err: errors.New("permission denied")}

	_, _, err := sampleColumns(t.Context(), reader, 100)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCountRows_CountsASmallTableExactly(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{
		testRow("user#1"), testRow("user#2"), testRow("user#3"),
	}}

	count, complete, err := countRows(t.Context(), reader, 100)

	require.NoError(t, err)
	assert.True(t, complete)
	assert.Equal(t, int64(3), count)
}

func TestCountRows_CountsAnEmptyTableAsZero(t *testing.T) {
	reader := &fakeReader{}

	count, complete, err := countRows(t.Context(), reader, 100)

	require.NoError(t, err)
	assert.True(t, complete)
	assert.Zero(t, count)
}

func TestCountRows_IsCompleteWhenTheTableIsExactlyTheLimit(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{testRow("a"), testRow("b"), testRow("c")}}

	count, complete, err := countRows(t.Context(), reader, 3)

	require.NoError(t, err)
	assert.True(t, complete, "the read asks for one row past the limit, so hitting it exactly is a real count")
	assert.Equal(t, int64(3), count)
}

func TestCountRows_ReportsAnIncompleteCountPastTheLimit(t *testing.T) {
	reader := &fakeReader{rows: []bt.Row{testRow("a"), testRow("b"), testRow("c"), testRow("d")}}

	_, complete, err := countRows(t.Context(), reader, 3)

	require.NoError(t, err)
	assert.False(t, complete, "a count that stopped at the limit is a floor, not a row count")
}

func TestCountRows_ReturnsTheReadError(t *testing.T) {
	reader := &fakeReader{err: errors.New("deadline exceeded")}

	_, _, err := countRows(t.Context(), reader, 100)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestInstanceAsset_CarriesTheInstanceDetails(t *testing.T) {
	s := &Source{config: &Config{ProjectID: "analytics"}}

	asset := s.instanceAsset(instanceDetail{
		ID:          "prod-metrics",
		DisplayName: "Production metrics",
		State:       "READY",
		Type:        "PRODUCTION",
		Labels:      map[string]string{"team": "platform"},
		Clusters:    []clusterDetail{{Name: "prod-metrics-c1", Zone: "us-central1-b", Nodes: 3, StorageType: "SSD"}},
	}, 4, 0)

	assert.Equal(t, "Instance", asset.Type)
	assert.Equal(t, "analytics", asset.Metadata["project_id"])
	assert.Equal(t, "prod-metrics", asset.Metadata["instance_id"])
	assert.Equal(t, "Production metrics", asset.Metadata["display_name"])
	assert.Equal(t, "READY", asset.Metadata["state"])
	assert.Equal(t, "PRODUCTION", asset.Metadata["instance_type"])
	assert.Equal(t, 4, asset.Metadata["table_count"])
	assert.Equal(t, map[string]string{"team": "platform"}, asset.Metadata["labels"])
}

func TestInstanceAsset_OmitsDetailsAnEmulatorCannotSupply(t *testing.T) {
	s := &Source{config: &Config{ProjectID: "test-project", EmulatorHost: "localhost:8086"}}

	asset := s.instanceAsset(instanceDetail{ID: "test-instance"}, 2, 0)

	assert.Equal(t, true, asset.Metadata["emulator"])
	assert.NotContains(t, asset.Metadata, "display_name")
	assert.NotContains(t, asset.Metadata, "clusters")
	assert.NotContains(t, asset.Metadata, "labels")
}

func TestInstanceAsset_ReportsBackupsOnlyWhenAskedFor(t *testing.T) {
	// Listing backups is an extra call per cluster, so it stays off unless
	// the config asks, and then the count is worth showing even at zero.
	off := &Source{config: &Config{ProjectID: "analytics"}}
	on := &Source{config: &Config{ProjectID: "analytics", IncludeBackups: true}}

	assert.NotContains(t, off.instanceAsset(instanceDetail{ID: "prod-metrics"}, 1, 0).Metadata, "backup_count")
	assert.Equal(t, 0, on.instanceAsset(instanceDetail{ID: "prod-metrics"}, 1, 0).Metadata["backup_count"])
}

func TestInstanceAsset_InterpolatesTagsFromItsMetadata(t *testing.T) {
	s := &Source{config: &Config{
		ProjectID: "analytics",
		BaseConfig: pluginsdk.BaseConfig{
			Tags: pluginsdk.TagsConfig{"instance:${instance_id}"},
		},
	}}

	asset := s.instanceAsset(instanceDetail{ID: "prod-metrics"}, 1, 0)

	assert.Contains(t, asset.Tags, "instance:prod-metrics")
}
