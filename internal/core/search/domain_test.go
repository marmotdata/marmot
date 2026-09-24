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

	refs := extractDomainRefs(query)
	if !reflect.DeepEqual(refs, []string{a, b, "Finanzas", "RRHH/Gestión de empleados"}) {
		t.Fatalf("refs = %v", refs)
	}
	for in, want := range map[string]string{
		query:                                  "orders AND @kind:asset",
		"@kind = asset AND @domain = Finanzas": "@kind = asset",
		"@domain:Finanzas AND @kind:asset":     "@kind:asset",
		"orders AND (@domain:a OR @domain:b)":  "orders",
		"@domain:a":                            "",
		"android @kind:asset":                  "android @kind:asset",
	} {
		if got := stripDomainFilter(in); got != want {
			t.Errorf("stripDomainFilter(%q) = %q, want %q", in, got, want)
		}
	}
	if refs := extractDomainRefs(`@domain != x @domains:y @domain:""`); refs != nil {
		t.Fatalf("refs = %v", refs)
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
}

func TestDomainFilterBypassesCachedFacets(t *testing.T) {
	r := &PostgresRepository{}
	where, params := r.buildListingFacetWhereClause(Filter{Domain: &DomainFilter{Paths: []string{"/a/"}}})
	if !strings.Contains(where, "asset_domains") || len(params) != 1 {
		t.Fatalf("listing facets ignore the domain filter: %s", where)
	}
}
