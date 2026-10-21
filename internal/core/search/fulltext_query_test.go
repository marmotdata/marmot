package search

import "testing"

func TestFullTextQuery(t *testing.T) {
	tests := []struct {
		q, fn, arg string
	}{
		{"data st", "to_tsquery", `'data' & 'st':*`},
		{"  data   steward  ", "to_tsquery", `'data' & 'steward':*`},
		{`c:\tmp files`, "websearch_to_tsquery", `c:\tmp files`},
		{`back\slash x`, "to_tsquery", `'back\\slash' & 'x':*`},
		{`"data steward"`, "websearch_to_tsquery", `"data steward"`},
		{"data -owner", "websearch_to_tsquery", "data -owner"},
		{"data", "websearch_to_tsquery", "data"},
	}
	for _, tc := range tests {
		fn, arg := fullTextQuery(tc.q)
		if fn != tc.fn || arg != tc.arg {
			t.Errorf("fullTextQuery(%q) = %s(%q), want %s(%q)", tc.q, fn, arg, tc.fn, tc.arg)
		}
	}
}
