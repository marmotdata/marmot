package sftp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const noByteCap = int64(1) << 30

func TestFileKind_RecognisesCSV(t *testing.T) {
	assert.Equal(t, kindCSV, fileKind("orders.csv"))
}

func TestFileKind_RecognisesTSV(t *testing.T) {
	assert.Equal(t, kindTSV, fileKind("orders.tsv"))
}

func TestFileKind_RecognisesJSONLines(t *testing.T) {
	assert.Equal(t, kindJSONLines, fileKind("events.jsonl"))
}

func TestFileKind_TreatsNdjsonAsJSONLines(t *testing.T) {
	assert.Equal(t, kindJSONLines, fileKind("events.ndjson"))
}

func TestFileKind_RecognisesParquet(t *testing.T) {
	assert.Equal(t, kindParquet, fileKind("part-0001.parquet"))
}

func TestFileKind_IgnoresCase(t *testing.T) {
	assert.Equal(t, kindCSV, fileKind("ORDERS.CSV"))
}

func TestFileKind_AnythingElseIsOther(t *testing.T) {
	assert.Equal(t, kindOther, fileKind("notes.txt"))
}

func TestIsStructured_KeepsDataFiles(t *testing.T) {
	assert.True(t, isStructured("orders.csv"))
	assert.True(t, isStructured("part.avro"))
}

func TestIsStructured_DropsEverythingElse(t *testing.T) {
	assert.False(t, isStructured("notes.txt"))
	assert.False(t, isStructured("README"))
}

func TestHasColumns_ParquetIsStructuredButUnreadable(t *testing.T) {
	// Parquet passes structured_only but this plugin carries no reader
	// for it, so it gets no columns.
	assert.True(t, isStructured("part.parquet"))
	assert.False(t, hasColumns(kindParquet))
}

func TestMimeType_IsFixedForDataFormats(t *testing.T) {
	assert.Equal(t, "text/csv", mimeType("orders.csv"))
	assert.Equal(t, "text/tab-separated-values", mimeType("orders.tsv"))
	assert.Equal(t, "application/json", mimeType("catalog.json"))
	assert.Equal(t, "application/x-ndjson", mimeType("events.jsonl"))
}

func TestSeparator_TabForTSVCommaForEverythingElse(t *testing.T) {
	assert.Equal(t, '\t', separator(kindTSV))
	assert.Equal(t, ',', separator(kindCSV))
}

func TestReadDelimited_CountsDataRowsNotTheHeader(t *testing.T) {
	data, err := readDelimited(strings.NewReader(ordersCSV), ',', noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(3), data.RowCount)
	assert.Equal(t, []string{"order_id", "customer", "amount", "ordered_on", "shipped", "note"}, data.Header)
}

func TestReadDelimited_IsExactWhenTheWholeFileFits(t *testing.T) {
	data, err := readDelimited(strings.NewReader(ordersCSV), ',', noByteCap, 200)

	require.NoError(t, err)
	assert.True(t, data.Exact)
}

func TestReadDelimited_IsNotExactWhenTheByteCapIsHit(t *testing.T) {
	data, err := readDelimited(strings.NewReader(ordersCSV), ',', 80, 200)

	require.NoError(t, err)
	assert.False(t, data.Exact)
	assert.Less(t, data.RowCount, int64(3))
}

func TestReadDelimited_DropsTheRowTheByteCapCutInHalf(t *testing.T) {
	// The cap lands inside the second data row, so only the first is data.
	csv := "id,name\n1,alice\n2,bobby\n"
	data, err := readDelimited(strings.NewReader(csv), ',', int64(len("id,name\n1,alice\n2,bo")), 200)

	require.NoError(t, err)
	assert.Equal(t, int64(1), data.RowCount)
	require.Len(t, data.Rows, 1)
	assert.Equal(t, "alice", data.Rows[0][1])
}

func TestReadDelimited_KeepsOnlyTheRequestedRows(t *testing.T) {
	data, err := readDelimited(strings.NewReader(ordersCSV), ',', noByteCap, 1)

	require.NoError(t, err)
	assert.Len(t, data.Rows, 1)
	assert.Equal(t, int64(3), data.RowCount, "rows past the sample are still counted")
}

func TestReadDelimited_UsesTabsForTSV(t *testing.T) {
	data, err := readDelimited(strings.NewReader("id\tname\n1\talice\n"), separator(kindTSV), noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name"}, data.Header)
}

func TestReadDelimited_StripsAByteOrderMarkFromTheFirstColumn(t *testing.T) {
	data, err := readDelimited(strings.NewReader("\ufeffid,name\n1,alice\n"), ',', noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, "id", data.Header[0])
}

func TestReadDelimited_NamesAnUnlabelledColumn(t *testing.T) {
	data, err := readDelimited(strings.NewReader("id,,name\n1,x,alice\n"), ',', noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, []string{"id", "column_2", "name"}, data.Header)
}

func TestReadDelimited_KeepsGoingThroughARaggedRow(t *testing.T) {
	data, err := readDelimited(strings.NewReader("id,name\n1\n2,bob\n"), ',', noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(2), data.RowCount)
}

func TestReadDelimited_EmptyFileHasNoHeader(t *testing.T) {
	data, err := readDelimited(strings.NewReader(""), ',', noByteCap, 200)

	require.NoError(t, err)
	assert.Empty(t, data.Header)
	assert.Equal(t, int64(0), data.RowCount)
}

func TestInferDelimitedColumns_WholeNumbersAreIntegers(t *testing.T) {
	columns := inferDelimitedColumns([]string{"id"}, [][]string{{"1"}, {"2"}, {"-3"}})

	assert.Equal(t, typeInt, columns[0].DataType)
}

func TestInferDelimitedColumns_DecimalsAreFloats(t *testing.T) {
	columns := inferDelimitedColumns([]string{"amount"}, [][]string{{"1.5"}, {"2.25"}})

	assert.Equal(t, typeFloat, columns[0].DataType)
}

func TestInferDelimitedColumns_AWholeNumberAmongDecimalsIsStillAFloat(t *testing.T) {
	// "42" is a perfectly good float, so the column is not mixed.
	columns := inferDelimitedColumns([]string{"amount"}, [][]string{{"42"}, {"2.25"}})

	assert.Equal(t, typeFloat, columns[0].DataType)
}

func TestInferDelimitedColumns_TrueAndFalseAreBooleans(t *testing.T) {
	columns := inferDelimitedColumns([]string{"shipped"}, [][]string{{"true"}, {"FALSE"}})

	assert.Equal(t, typeBoolean, columns[0].DataType)
}

func TestInferDelimitedColumns_ZeroAndOneAreIntegersNotBooleans(t *testing.T) {
	columns := inferDelimitedColumns([]string{"flag"}, [][]string{{"0"}, {"1"}})

	assert.Equal(t, typeInt, columns[0].DataType)
}

func TestInferDelimitedColumns_PlainDatesAreDatetimes(t *testing.T) {
	columns := inferDelimitedColumns([]string{"day"}, [][]string{{"2026-09-01"}, {"2026-09-02"}})

	assert.Equal(t, typeDatetime, columns[0].DataType)
}

func TestInferDelimitedColumns_RFC3339TimestampsAreDatetimes(t *testing.T) {
	columns := inferDelimitedColumns([]string{"ts"}, [][]string{{"2026-09-01T10:00:00Z"}})

	assert.Equal(t, typeDatetime, columns[0].DataType)
}

func TestInferDelimitedColumns_SpaceSeparatedTimestampsAreDatetimes(t *testing.T) {
	columns := inferDelimitedColumns([]string{"ts"}, [][]string{{"2026-09-01 10:00:00"}})

	assert.Equal(t, typeDatetime, columns[0].DataType)
}

func TestInferDelimitedColumns_MixedKindsFallBackToString(t *testing.T) {
	columns := inferDelimitedColumns([]string{"value"}, [][]string{{"1"}, {"alice"}})

	assert.Equal(t, typeString, columns[0].DataType)
}

func TestInferDelimitedColumns_AnEmptyCellMakesTheColumnNullable(t *testing.T) {
	columns := inferDelimitedColumns([]string{"note"}, [][]string{{"rush"}, {""}})

	assert.True(t, columns[0].Nullable)
	assert.Equal(t, typeString, columns[0].DataType)
}

func TestInferDelimitedColumns_AnEmptyCellDoesNotChangeTheType(t *testing.T) {
	columns := inferDelimitedColumns([]string{"id"}, [][]string{{"1"}, {""}, {"3"}})

	assert.Equal(t, typeInt, columns[0].DataType)
	assert.True(t, columns[0].Nullable)
}

func TestInferDelimitedColumns_AShortRowMakesTheMissingColumnsNullable(t *testing.T) {
	columns := inferDelimitedColumns([]string{"id", "name"}, [][]string{{"1", "alice"}, {"2"}})

	assert.False(t, columns[0].Nullable)
	assert.True(t, columns[1].Nullable)
}

func TestInferDelimitedColumns_AColumnWithNoValuesIsAString(t *testing.T) {
	columns := inferDelimitedColumns([]string{"note"}, [][]string{{""}, {""}})

	assert.Equal(t, typeString, columns[0].DataType)
	assert.True(t, columns[0].Nullable)
}

func TestInferDelimitedColumns_AHeaderWithNoRowsStillProducesColumns(t *testing.T) {
	columns := inferDelimitedColumns([]string{"id", "name"}, nil)

	require.Len(t, columns, 2)
	assert.Equal(t, typeString, columns[0].DataType)
}

func TestReadJSON_ReadsOneObjectPerLine(t *testing.T) {
	data, err := readJSON(strings.NewReader(eventsJSONL), noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(3), data.Count)
	assert.Len(t, data.Records, 3)
	assert.True(t, data.Exact)
}

func TestReadJSON_ReadsATopLevelArray(t *testing.T) {
	data, err := readJSON(strings.NewReader(`[{"id":1},{"id":2}]`), noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(2), data.Count)
}

func TestReadJSON_ReadsASingleObject(t *testing.T) {
	data, err := readJSON(strings.NewReader(`{"id":1}`), noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(1), data.Count)
}

func TestReadJSON_SkipsALeadingByteOrderMark(t *testing.T) {
	data, err := readJSON(strings.NewReader("\ufeff{\"id\":1}"), noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(1), data.Count)
}

func TestReadJSON_RejectsAFileThatIsNotObjects(t *testing.T) {
	_, err := readJSON(strings.NewReader(`"just a string"`), noByteCap, 200)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "json object")
}

func TestReadJSON_AnEmptyFileReadsAsNoRecords(t *testing.T) {
	data, err := readJSON(strings.NewReader(""), noByteCap, 200)

	require.NoError(t, err)
	assert.Equal(t, int64(0), data.Count)
}

func TestReadJSON_IsNotExactWhenTheByteCapIsHit(t *testing.T) {
	data, err := readJSON(strings.NewReader(eventsJSONL), 120, 200)

	require.NoError(t, err)
	assert.False(t, data.Exact)
	assert.Less(t, data.Count, int64(3))
}

func TestReadJSON_KeepsOnlyTheRequestedRecords(t *testing.T) {
	data, err := readJSON(strings.NewReader(eventsJSONL), noByteCap, 1)

	require.NoError(t, err)
	assert.Len(t, data.Records, 1)
	assert.Equal(t, int64(3), data.Count)
}

func TestInferJSONColumns_FlattensOneLevelOfNesting(t *testing.T) {
	data, err := readJSON(strings.NewReader(eventsJSONL), noByteCap, 200)
	require.NoError(t, err)

	columns := inferJSONColumns(data.Records)

	assert.Equal(t, "object", jsonColumnNamed(t, columns, "actor").DataType)
	assert.Equal(t, "string", jsonColumnNamed(t, columns, "actor.id").DataType)
}

func TestInferJSONColumns_DoesNotFlattenASecondLevel(t *testing.T) {
	records := []map[string]any{{"a": map[string]any{"b": map[string]any{"c": 1.0}}}}

	columns := inferJSONColumns(records)

	assert.Equal(t, "object", jsonColumnNamed(t, columns, "a.b").DataType)
	for _, column := range columns {
		assert.NotEqual(t, "a.b.c", column.Name)
	}
}

func TestInferJSONColumns_JoinsMixedTypesInAFixedOrder(t *testing.T) {
	records := []map[string]any{{"v": "text"}, {"v": nil}, {"v": 1.0}}

	columns := inferJSONColumns(records)

	assert.Equal(t, "string|number|null", jsonColumnNamed(t, columns, "v").DataType)
}

func TestInferJSONColumns_CountsHowOftenAKeyAppeared(t *testing.T) {
	records := []map[string]any{{"a": 1.0, "b": 2.0}, {"a": 3.0}}

	columns := inferJSONColumns(records)

	assert.Equal(t, 2, jsonColumnNamed(t, columns, "a").Occurrence)
	assert.Equal(t, 1, jsonColumnNamed(t, columns, "b").Occurrence)
}

func TestInferJSONColumns_AKeyMissingFromSomeRecordsIsNullable(t *testing.T) {
	records := []map[string]any{{"a": 1.0, "b": 2.0}, {"a": 3.0}}

	columns := inferJSONColumns(records)

	assert.False(t, jsonColumnNamed(t, columns, "a").Nullable)
	assert.True(t, jsonColumnNamed(t, columns, "b").Nullable)
}

func TestInferJSONColumns_ANullValueIsNullable(t *testing.T) {
	records := []map[string]any{{"a": nil}}

	assert.True(t, jsonColumnNamed(t, inferJSONColumns(records), "a").Nullable)
}

func TestInferJSONColumns_RecognisesArraysAndBooleans(t *testing.T) {
	records := []map[string]any{{"items": []any{1.0}, "ok": true}}

	columns := inferJSONColumns(records)

	assert.Equal(t, "array", jsonColumnNamed(t, columns, "items").DataType)
	assert.Equal(t, "boolean", jsonColumnNamed(t, columns, "ok").DataType)
}

func TestInferJSONColumns_AreSortedSoTheSchemaIsStable(t *testing.T) {
	records := []map[string]any{{"z": 1.0, "a": 2.0, "m": 3.0}}

	columns := inferJSONColumns(records)

	require.Len(t, columns, 3)
	assert.Equal(t, "a", columns[0].Name)
	assert.Equal(t, "m", columns[1].Name)
	assert.Equal(t, "z", columns[2].Name)
}

func TestJSONValue_ReadsATopLevelKey(t *testing.T) {
	assert.Equal(t, 1.0, jsonValue(map[string]any{"a": 1.0}, "a"))
}

func TestJSONValue_ReadsANestedKey(t *testing.T) {
	record := map[string]any{"actor": map[string]any{"id": "u1"}}

	assert.Equal(t, "u1", jsonValue(record, "actor.id"))
}

func TestJSONValue_MissingKeyIsNil(t *testing.T) {
	assert.Nil(t, jsonValue(map[string]any{"a": 1.0}, "b.c"))
}

func jsonColumnNamed(t *testing.T, columns []jsonColumn, name string) jsonColumn {
	t.Helper()

	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("no json column named %q", name)
	return jsonColumn{}
}
