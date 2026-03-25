package elasticsearch

import (
	"github.com/marmotdata/marmot/internal/core/search"
)

// buildSearchQuery translates a search.Filter into an Elasticsearch query body.
func buildSearchQuery(filter search.Filter) map[string]interface{} {
	must := []interface{}{}
	filterClauses := []interface{}{}

	// Text query: multi_match with fuzziness
	if filter.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     filter.Query,
				"fields":    []string{"name^5", "description^3", "tags^3", "mrn^2", "metadata.*^1.5", "documentation"},
				"fuzziness": "AUTO",
				"type":      "best_fields",
			},
		})
	}

	// Type filter
	if len(filter.Types) > 0 {
		types := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			types[i] = string(t)
		}
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{
				"type": types,
			},
		})
	}

	// Asset type filter
	if len(filter.AssetTypes) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{
						"terms": map[string]interface{}{
							"asset_type": filter.AssetTypes,
						},
					},
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must_not": []interface{}{
								map[string]interface{}{
									"term": map[string]interface{}{
										"type": "asset",
									},
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Provider filter
	if len(filter.Providers) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{
				"providers": filter.Providers,
			},
		})
	}

	// Tag filter
	if len(filter.Tags) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{
				"tags": filter.Tags,
			},
		})
	}

	boolQuery := map[string]interface{}{}
	if len(must) > 0 {
		boolQuery["must"] = must
	}
	if len(filterClauses) > 0 {
		boolQuery["filter"] = filterClauses
	}

	// If no clauses, match all
	query := map[string]interface{}{}
	if len(boolQuery) == 0 {
		query["match_all"] = map[string]interface{}{}
	} else {
		query["bool"] = boolQuery
	}

	if filter.Query != "" {
		query = map[string]interface{}{
			"function_score": map[string]interface{}{
				"query": query,
				"functions": []interface{}{
					map[string]interface{}{
						"filter": map[string]interface{}{
							"term": map[string]interface{}{"type": "glossary"},
						},
						"weight": 1.5,
					},
					map[string]interface{}{
						"filter": map[string]interface{}{
							"term": map[string]interface{}{"type": "data_product"},
						},
						"weight": 1.2,
					},
				},
				"boost_mode": "multiply",
				"score_mode": "first",
			},
		}
	}

	// Sort: _score desc then updated_at desc
	sort := []interface{}{
		map[string]interface{}{"_score": map[string]interface{}{"order": "desc"}},
		map[string]interface{}{"updated_at": map[string]interface{}{"order": "desc"}},
	}

	// Aggregations for facets
	aggs := map[string]interface{}{
		"types": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "type",
				"size":  10,
			},
		},
		"asset_types": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "asset_type",
				"size":  50,
			},
		},
		"providers": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "providers",
				"size":  50,
			},
		},
		"tags": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "tags",
				"size":  50,
			},
		},
	}

	body := map[string]interface{}{
		"query": query,
		"sort":  sort,
		"from":  filter.Offset,
		"size":  filter.Limit,
		"aggs":  aggs,
	}

	return body
}
