package search

import (
	"reflect"
	"testing"
)

func TestKindFilters(t *testing.T) {
	cases := []struct {
		query    string
		kinds    []ResultType
		stripped string
	}{
		{"orders @kind:asset", []ResultType{ResultTypeAsset}, "orders"},
		{`@kind = "data_product" sales`, []ResultType{ResultTypeDataProduct}, "sales"},
		{"@kind:TEAM", []ResultType{ResultTypeTeam}, ""},
		// A type the catalogue does not define filters like its own, so an edition's records can be asked for by name
		{"churn @kind:task", []ResultType{"task"}, "churn"},
		{"@kind:asset AND @kind:asset", []ResultType{ResultTypeAsset}, ""},
		{"@kind:asset AND @kind:glossary", []ResultType{"__CONTRADICTION__"}, ""},
		{"no kind here", nil, "no kind here"},
	}
	for _, c := range cases {
		if got := extractKindFilters(c.query); !reflect.DeepEqual(got, c.kinds) {
			t.Errorf("extractKindFilters(%q) = %v, want %v", c.query, got, c.kinds)
		}
		if got := stripKindFilter(c.query); got != c.stripped {
			t.Errorf("stripKindFilter(%q) = %q, want %q", c.query, got, c.stripped)
		}
	}
}
