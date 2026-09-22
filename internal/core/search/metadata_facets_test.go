package search

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/query"
)

func TestAppendMetadataFilterClauses(t *testing.T) {
	clauses, params := appendMetadataFilterClauses(map[string][]string{
		"empty":                           nil,
		"metadata.example.classification": {`{"example":{"classification":"public"}}`, `{"example":{"classification":"confidential"}}`},
		"metadata.example.contains_pii":   {`{"example":{"contains_pii":true}}`},
	}, []string{"name = $1"}, []interface{}{"existing"})

	wantClauses := []string{"name = $1", "(metadata @> $2::jsonb OR metadata @> $3::jsonb)", "(metadata @> $4::jsonb)"}
	if !reflect.DeepEqual(clauses, wantClauses) {
		t.Fatalf("clauses = %v, want %v", clauses, wantClauses)
	}
	wantParams := []interface{}{"existing", `{"example":{"classification":"public"}}`, `{"example":{"classification":"confidential"}}`, `{"example":{"contains_pii":true}}`}
	if !reflect.DeepEqual(params, wantParams) {
		t.Fatalf("params = %v, want %v", params, wantParams)
	}
}

func TestAppendMetadataFilterClausesEmpty(t *testing.T) {
	for _, filters := range []map[string][]string{nil, {}, {"empty": {}}} {
		clauses, params := appendMetadataFilterClauses(filters, nil, []interface{}{"existing"})
		if clauses != nil || !reflect.DeepEqual(params, []interface{}{"existing"}) {
			t.Fatalf("expected a no-op for empty filters, got clauses=%v params=%v", clauses, params)
		}
	}
}

func TestMetadataFiltersInSearchQueries(t *testing.T) {
	r := &PostgresRepository{}
	literal := `{"example":{"classification":"public"}}`
	filter := Filter{
		Tags: []string{"demo"}, Limit: 20, Offset: 5,
		MetadataFilters: map[string][]string{"metadata.example.classification": {literal}},
	}
	for _, text := range []string{"", "ab", "catalog", "data catalog"} {
		t.Run(text, func(t *testing.T) {
			parsed, err := query.NewParser().Parse(text)
			if err != nil {
				t.Fatal(err)
			}
			sql, params := r.buildOptimizedSearchQuery(text, filter, parsed)
			wantParams := []interface{}{filter.Tags, literal, 20, 5}
			if text != "" {
				wantParams = append([]interface{}{text}, wantParams...)
			}
			if !reflect.DeepEqual(params, wantParams) {
				t.Fatalf("params = %v, want %v", params, wantParams)
			}
			n := len(wantParams)
			for _, clause := range []string{
				fmt.Sprintf("tags && $%d AND (metadata @> $%d::jsonb)", n-3, n-2),
				fmt.Sprintf("LIMIT $%d OFFSET $%d", n-1, n),
			} {
				if !strings.Contains(sql, clause) {
					t.Fatalf("missing %q in %s", clause, sql)
				}
			}
		})
	}
	where, params := r.buildListingFacetWhereClause(filter)
	if where != "WHERE tags && $1 AND (metadata @> $2::jsonb)" || !reflect.DeepEqual(params, []interface{}{filter.Tags, literal}) {
		t.Fatalf("unexpected facet filters: %s, %v", where, params)
	}
	parsed, err := query.NewParser().Parse(`@metadata.owner: "demo"`)
	if err != nil {
		t.Fatal(err)
	}
	clauses, params, count := r.buildFilterClauses(filter, parsed, nil, 0)
	if count != 3 || len(params) != 3 || !strings.Contains(strings.Join(clauses, " AND "), "metadata @> $3::jsonb") {
		t.Fatalf("structured filter lost its parameter offset: %v, %v, %d", clauses, params, count)
	}
}
