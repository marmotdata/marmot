package elasticsearch

import (
	"encoding/json"
	"testing"

	"github.com/marmotdata/marmot/internal/core/search"
)

func TestBuildSearchQuery_TextQuery(t *testing.T) {
	filter := search.Filter{
		Query:  "my table",
		Limit:  20,
		Offset: 0,
	}

	body := buildSearchQuery(filter)

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal query: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal query: %v", err)
	}

	query, ok := result["query"].(map[string]interface{})
	if !ok {
		t.Fatal("expected query field")
	}

	// Text queries should be wrapped in function_score
	fs, ok := query["function_score"].(map[string]interface{})
	if !ok {
		t.Fatal("expected function_score wrapper for text query")
	}

	innerQuery, ok := fs["query"].(map[string]interface{})
	if !ok {
		t.Fatal("expected inner query in function_score")
	}

	boolQ, ok := innerQuery["bool"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bool query")
	}

	must, ok := boolQ["must"].([]interface{})
	if !ok || len(must) == 0 {
		t.Fatal("expected must clause with multi_match")
	}

	// Verify multi_match is present with correct field boosts
	mm, ok := must[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected multi_match object")
	}
	mmInner, ok := mm["multi_match"].(map[string]interface{})
	if !ok {
		t.Fatal("expected multi_match key")
	}
	fields, ok := mmInner["fields"].([]interface{})
	if !ok {
		t.Fatal("expected fields array")
	}
	// name should have the highest boost
	if fields[0] != "name^5" {
		t.Errorf("expected name^5 as first field, got %v", fields[0])
	}

	// Verify entity type boost functions
	functions, ok := fs["functions"].([]interface{})
	if !ok || len(functions) != 2 {
		t.Fatalf("expected 2 boost functions, got %v", functions)
	}
	if fs["boost_mode"] != "multiply" {
		t.Errorf("expected boost_mode=multiply, got %v", fs["boost_mode"])
	}
}

func TestBuildSearchQuery_EmptyQuery(t *testing.T) {
	filter := search.Filter{
		Limit:  20,
		Offset: 0,
	}

	body := buildSearchQuery(filter)

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal query: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal query: %v", err)
	}

	query := result["query"].(map[string]interface{})
	if _, ok := query["match_all"]; !ok {
		t.Fatal("expected match_all for empty query")
	}
}

func TestBuildSearchQuery_WithFilters(t *testing.T) {
	filter := search.Filter{
		Query:      "test",
		Types:      []search.ResultType{search.ResultTypeAsset, search.ResultTypeGlossary},
		AssetTypes: []string{"TABLE"},
		Providers:  []string{"postgresql"},
		Tags:       []string{"production"},
		Limit:      10,
		Offset:     5,
	}

	body := buildSearchQuery(filter)

	// Check size and from
	if body["size"] != 10 {
		t.Errorf("expected size=10, got %v", body["size"])
	}
	if body["from"] != 5 {
		t.Errorf("expected from=5, got %v", body["from"])
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal query: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal query: %v", err)
	}

	query := result["query"].(map[string]interface{})

	// Text queries are wrapped in function_score
	fs := query["function_score"].(map[string]interface{})
	innerQuery := fs["query"].(map[string]interface{})
	boolQ := innerQuery["bool"].(map[string]interface{})

	filterClauses, ok := boolQ["filter"].([]interface{})
	if !ok {
		t.Fatal("expected filter clauses")
	}

	// Should have 4 filter clauses: types, asset_types, providers, tags
	if len(filterClauses) != 4 {
		t.Errorf("expected 4 filter clauses, got %d", len(filterClauses))
	}
}

func TestBuildSearchQuery_Aggregations(t *testing.T) {
	filter := search.Filter{
		Query: "test",
		Limit: 20,
	}

	body := buildSearchQuery(filter)

	aggs, ok := body["aggs"].(map[string]interface{})
	if !ok {
		t.Fatal("expected aggs field")
	}

	expectedAggs := []string{"types", "asset_types", "providers", "tags"}
	for _, name := range expectedAggs {
		if _, ok := aggs[name]; !ok {
			t.Errorf("expected aggregation %q", name)
		}
	}
}
