package assetdetail

import (
	"encoding/json"
	"math"
	"strconv"
)

// rowCountKeys are the metadata keys plugins use for a table's row count, most common first.
var rowCountKeys = []string{"row_count", "num_rows", "numRows", "rowCount", "rows", "record_count", "estimated_rows"}

// RowCount reads a table's row count from the metadata its plugin wrote, as plugins name the key differently. It reports false when no plugin recorded one.
func RowCount(metadata map[string]any) (int64, bool) {
	for _, k := range rowCountKeys {
		if n, ok := toInt(metadata[k]); ok {
			return n, true
		}
	}
	return 0, false
}

// ColumnCount counts the top-level fields of an asset's schema, preferring the schema called columns that table plugins write. It understands a column list, JSON Schema properties and Avro fields, and reports false for anything else.
func ColumnCount(schema map[string]string) (int, bool) {
	if raw, ok := schema["columns"]; ok {
		if n, ok := fieldCount(raw); ok {
			return n, true
		}
	}
	for _, raw := range schema {
		if n, ok := fieldCount(raw); ok {
			return n, true
		}
	}
	return 0, false
}

func fieldCount(raw string) (int, bool) {
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return 0, false
	}
	switch d := doc.(type) {
	case []any:
		return len(d), len(d) > 0
	case map[string]any:
		if props, ok := d["properties"].(map[string]any); ok {
			return len(props), true
		}
		if fields, ok := d["fields"].([]any); ok {
			return len(fields), true
		}
		if cols, ok := d["columns"].([]any); ok {
			return len(cols), true
		}
	}
	return 0, false
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), n >= 0
	case int32:
		return int64(n), n >= 0
	case int64:
		return n, n >= 0
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float64:
		if n < 0 || n > math.MaxInt64 || math.IsNaN(n) {
			return 0, false
		}
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil && i >= 0
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil && i >= 0
	}
	return 0, false
}
