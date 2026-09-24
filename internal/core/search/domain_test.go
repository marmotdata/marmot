package search

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractAndStripDomainFilter(t *testing.T) {
	a := "0b7e4f7a-0000-4000-8000-000000000001"
	b := "0B7E4F7A-0000-4000-8000-000000000002"
	query := `orders @domain:` + a + ` @DOMAIN = "` + b + `" AND (@domain:Finanzas) @domain = "RRHH/Gestión de empleados" @kind:asset`

	include, exclude := extractDomainRefs(query)
	if !reflect.DeepEqual(include, []string{a, b, "Finanzas", "RRHH/Gestión de empleados"}) || exclude != nil {
		t.Fatalf("include = %v, exclude = %v", include, exclude)
	}
	include, exclude = extractDomainRefs(`@domain:Finanzas NOT @domain:"Finanzas/Pagos" and not @domain:Legal`)
	if !reflect.DeepEqual(include, []string{"Finanzas"}) || !reflect.DeepEqual(exclude, []string{"Finanzas/Pagos", "Legal"}) {
		t.Fatalf("include = %v, exclude = %v", include, exclude)
	}
	for in, want := range map[string]string{
		query:                                  "orders AND @kind:asset",
		"@kind = asset AND @domain = Finanzas": "@kind = asset",
		"@domain:Finanzas AND @kind:asset":     "@kind:asset",
		"orders AND (@domain:a OR @domain:b)":  "orders",
		"@domain:a":                            "",
		"android @kind:asset":                  "android @kind:asset",
		"orders NOT @domain:Legal":             "orders",
		"@kind:asset AND NOT @domain:Legal":    "@kind:asset",
		"NOT @domain:Legal @kind:asset":        "@kind:asset",
		"annotated NOT secret":                 "annotated NOT secret",
	} {
		if got := stripDomainFilter(in); got != want {
			t.Errorf("stripDomainFilter(%q) = %q, want %q", in, got, want)
		}
	}
	if include, exclude := extractDomainRefs(`@domain != x @domains:y @domain:""`); include != nil || exclude != nil {
		t.Fatalf("include = %v, exclude = %v", include, exclude)
	}
}

func TestAppendDomainClauses(t *testing.T) {
	clauses, params := appendDomainClauses(nil, []string{"x"}, []interface{}{"p"})
	if len(clauses) != 1 || len(params) != 1 {
		t.Fatal("a nil filter must be a no-op")
	}

	f := &DomainFilter{Paths: []string{"/a/"}}
	clauses, params = appendDomainClauses(f, []string{"name = $1"}, []interface{}{"existing"})
	if len(params) != 2 || !reflect.DeepEqual(params[1], []string{"/a/"}) {
		t.Fatalf("params = %v", params)
	}
	clause := clauses[1]
	for _, want := range []string{"unnest($2::text[])", "asset_domains", "data_product_domains", "glossary_term_domains"} {
		if !strings.Contains(clause, want) {
			t.Fatalf("clause misses %q: %s", want, clause)
		}
	}
	if strings.Contains(clause, "NOT IN") {
		t.Fatal("unassigned entities must not match unless Unassigned is selected")
	}

	f.Unassigned = true
	clauses, _ = appendDomainClauses(f, nil, nil)
	if !strings.Contains(clauses[0], "NOT IN") {
		t.Fatal("selecting Unassigned must match entities without a membership row")
	}

	only := &DomainFilter{Exclude: &DomainFilter{Paths: []string{"/b/"}}}
	clauses, params = appendDomainClauses(only, nil, nil)
	if len(clauses) != 1 || !strings.HasPrefix(clauses[0], "NOT (") || !reflect.DeepEqual(params, []interface{}{[]string{"/b/"}}) {
		t.Fatalf("exclusion alone: %v %v", clauses, params)
	}
	both := &DomainFilter{Paths: []string{"/a/"}, Exclude: &DomainFilter{Paths: []string{"/a/b/"}}}
	clauses, params = appendDomainClauses(both, nil, nil)
	if len(clauses) != 2 || !strings.Contains(clauses[1], "$2::text[]") || len(params) != 2 {
		t.Fatalf("selection and exclusion: %v %v", clauses, params)
	}
}

func TestDomainFilterBypassesCachedFacets(t *testing.T) {
	r := &PostgresRepository{}
	where, params := r.buildListingFacetWhereClause(Filter{Domain: &DomainFilter{Paths: []string{"/a/"}}})
	if !strings.Contains(where, "asset_domains") || len(params) != 1 {
		t.Fatalf("listing facets ignore the domain filter: %s", where)
	}
}
