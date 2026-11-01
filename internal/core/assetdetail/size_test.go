package assetdetail

import (
	"encoding/json"
	"testing"
)

func TestRowCount(t *testing.T) {
	cases := []struct {
		name string
		meta map[string]any
		want int64
		ok   bool
	}{
		{"postgres estimate", map[string]any{"row_count": float64(1284)}, 1284, true},
		{"bigquery", map[string]any{"num_rows": uint64(42)}, 42, true},
		{"json number", map[string]any{"numRows": json.Number("7")}, 7, true},
		{"string", map[string]any{"rows": "12"}, 12, true},
		{"first key wins", map[string]any{"rows": 1, "row_count": 2}, 2, true},
		{"negative is unknown", map[string]any{"row_count": float64(-1)}, 0, false},
		{"not a number", map[string]any{"rows": []any{1, 2}}, 0, false},
		{"missing", map[string]any{"owner": "x"}, 0, false},
		{"nil", nil, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := RowCount(c.meta)
			if got != c.want || ok != c.ok {
				t.Fatalf("RowCount = %d, %v; want %d, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestColumnCount(t *testing.T) {
	cases := []struct {
		name   string
		schema map[string]string
		want   int
		ok     bool
	}{
		{"column list", map[string]string{"columns": `[{"name":"a"},{"name":"b"}]`}, 2, true},
		{"json schema", map[string]string{"value": `{"type":"object","properties":{"a":{},"b":{},"c":{}}}`}, 3, true},
		{"avro", map[string]string{"value": `{"type":"record","fields":[{"name":"a"}]}`}, 1, true},
		{"columns preferred", map[string]string{"columns": `[{"name":"a"}]`, "other": `[1,2,3]`}, 1, true},
		{"protobuf text", map[string]string{"proto": "message A { string a = 1; }"}, 0, false},
		{"empty list", map[string]string{"columns": `[]`}, 0, false},
		{"none", nil, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ColumnCount(c.schema)
			if got != c.want || ok != c.ok {
				t.Fatalf("ColumnCount = %d, %v; want %d, %v", got, ok, c.want, c.ok)
			}
		})
	}
}
