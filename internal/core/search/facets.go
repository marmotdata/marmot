package search

import (
	"context"
	"fmt"
	"strings"
)

type facetResult struct {
	typeCounts      map[ResultType]int
	assetTypeCounts []FacetValue
	providerCounts  []FacetValue
	tagCounts       []FacetValue
	err             error
}

func (r *PostgresRepository) buildFacets(ctx context.Context, filter Filter) (*Facets, error) {
	kindFilters := extractKindFilters(filter.Query)
	if len(kindFilters) > 0 {
		if len(kindFilters) == 1 && kindFilters[0] == "__CONTRADICTION__" {
			return &Facets{
				Types:      make(map[ResultType]int),
				AssetTypes: []FacetValue{},
				Providers:  []FacetValue{},
				Tags:       []FacetValue{},
			}, nil
		}
		filter.Types = kindFilters
	}

	searchQuery := stripKindFilter(filter.Query)

	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)
	includeDataProducts := searchTypeIncluded(filter.Types, ResultTypeDataProduct)

	facets := &Facets{
		Types:      make(map[ResultType]int),
		AssetTypes: []FacetValue{},
		Providers:  []FacetValue{},
		Tags:       []FacetValue{},
	}

	assetWhereClause, assetParams := r.buildAssetFacetWhereClause(searchQuery, filter)

	resultChan := make(chan facetResult, 1)

	go func() {
		result := facetResult{typeCounts: make(map[ResultType]int)}

		if err := r.countTypes(ctx, &result, searchQuery, assetWhereClause, assetParams,
			includeAssets, includeGlossary, includeTeams, includeDataProducts); err != nil {
			result.err = err
			resultChan <- result
			return
		}

		if includeAssets {
			if err := r.countAssetTypes(ctx, &result, assetWhereClause, assetParams); err != nil {
				result.err = err
				resultChan <- result
				return
			}
			if err := r.countProviders(ctx, &result, assetWhereClause, assetParams); err != nil {
				result.err = err
				resultChan <- result
				return
			}
		}

		if err := r.countTags(ctx, &result, searchQuery, filter, assetWhereClause, assetParams,
			includeAssets, includeGlossary, includeTeams, includeDataProducts); err != nil {
			result.err = err
			resultChan <- result
			return
		}

		resultChan <- result
	}()

	result := <-resultChan
	if result.err != nil {
		return nil, result.err
	}

	facets.Types = result.typeCounts
	facets.AssetTypes = result.assetTypeCounts
	facets.Providers = result.providerCounts
	facets.Tags = result.tagCounts

	return facets, nil
}

func (r *PostgresRepository) countTypes(ctx context.Context, result *facetResult, searchQuery, assetWhereClause string, assetParams []interface{},
	includeAssets, includeGlossary, includeTeams, includeDataProducts bool) error {

	if includeAssets {
		var count int
		q := "SELECT COUNT(*) FROM assets WHERE is_stub = FALSE" + assetWhereClause
		if err := r.db.QueryRow(ctx, q, assetParams...).Scan(&count); err != nil {
			return fmt.Errorf("counting assets: %w", err)
		}
		result.typeCounts[ResultTypeAsset] = count
	}

	if includeGlossary {
		var count int
		q := "SELECT COUNT(*) FROM glossary_terms WHERE deleted_at IS NULL"
		if searchQuery != "" {
			q += " AND search_text @@ websearch_to_tsquery('english', $1)"
			if err := r.db.QueryRow(ctx, q, searchQuery).Scan(&count); err != nil {
				return fmt.Errorf("counting glossary: %w", err)
			}
		} else {
			if err := r.db.QueryRow(ctx, q).Scan(&count); err != nil {
				return fmt.Errorf("counting glossary: %w", err)
			}
		}
		result.typeCounts[ResultTypeGlossary] = count
	}

	if includeTeams {
		var count int
		q := "SELECT COUNT(*) FROM teams"
		if searchQuery != "" {
			q += " WHERE search_text @@ websearch_to_tsquery('english', $1)"
			if err := r.db.QueryRow(ctx, q, searchQuery).Scan(&count); err != nil {
				return fmt.Errorf("counting teams: %w", err)
			}
		} else {
			if err := r.db.QueryRow(ctx, q).Scan(&count); err != nil {
				return fmt.Errorf("counting teams: %w", err)
			}
		}
		result.typeCounts[ResultTypeTeam] = count
	}

	if includeDataProducts {
		var count int
		q := "SELECT COUNT(*) FROM data_products"
		if searchQuery != "" {
			q += " WHERE search_text @@ websearch_to_tsquery('english', $1)"
			if err := r.db.QueryRow(ctx, q, searchQuery).Scan(&count); err != nil {
				return fmt.Errorf("counting data_products: %w", err)
			}
		} else {
			if err := r.db.QueryRow(ctx, q).Scan(&count); err != nil {
				return fmt.Errorf("counting data_products: %w", err)
			}
		}
		result.typeCounts[ResultTypeDataProduct] = count
	}

	return nil
}

func (r *PostgresRepository) countAssetTypes(ctx context.Context, result *facetResult, assetWhereClause string, assetParams []interface{}) error {
	q := `SELECT type, COUNT(*) as count FROM assets WHERE is_stub = FALSE` + assetWhereClause + ` GROUP BY type ORDER BY count DESC LIMIT 50`
	rows, err := r.db.Query(ctx, q, assetParams...)
	if err != nil {
		return fmt.Errorf("querying asset types: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var assetType string
		var count int
		if err := rows.Scan(&assetType, &count); err != nil {
			return fmt.Errorf("scanning asset type: %w", err)
		}
		result.assetTypeCounts = append(result.assetTypeCounts, FacetValue{Value: assetType, Count: count})
	}
	return nil
}

func (r *PostgresRepository) countProviders(ctx context.Context, result *facetResult, assetWhereClause string, assetParams []interface{}) error {
	q := `SELECT provider, COUNT(*) as count FROM (SELECT UNNEST(providers) as provider FROM assets WHERE is_stub = FALSE` + assetWhereClause + `) sub GROUP BY provider ORDER BY count DESC LIMIT 50`
	rows, err := r.db.Query(ctx, q, assetParams...)
	if err != nil {
		return fmt.Errorf("querying providers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err != nil {
			return fmt.Errorf("scanning provider: %w", err)
		}
		result.providerCounts = append(result.providerCounts, FacetValue{Value: provider, Count: count})
	}
	return nil
}

func (r *PostgresRepository) countTags(ctx context.Context, result *facetResult, searchQuery string, filter Filter, assetWhereClause string, assetParams []interface{},
	includeAssets, includeGlossary, includeTeams, includeDataProducts bool) error {

	tagQuery := r.buildTagFacetQuery(searchQuery, includeAssets, includeGlossary, includeTeams, includeDataProducts, assetWhereClause)
	if tagQuery == "" {
		return nil
	}

	rows, err := r.db.Query(ctx, tagQuery, assetParams...)
	if err != nil {
		return fmt.Errorf("querying tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return fmt.Errorf("scanning tag: %w", err)
		}
		result.tagCounts = append(result.tagCounts, FacetValue{Value: tag, Count: count})
	}
	return nil
}

func (r *PostgresRepository) buildAssetFacetWhereClause(searchQuery string, filter Filter) (string, []interface{}) {
	var whereParts []string
	var params []interface{}
	paramCount := 0

	if searchQuery != "" {
		paramCount++
		whereParts = append(whereParts, fmt.Sprintf("search_text @@ websearch_to_tsquery('english', $%d)", paramCount))
		params = append(params, searchQuery)
	}

	if len(filter.AssetTypes) > 0 {
		paramCount++
		whereParts = append(whereParts, fmt.Sprintf("type = ANY($%d)", paramCount))
		params = append(params, filter.AssetTypes)
	}

	if len(filter.Providers) > 0 {
		paramCount++
		whereParts = append(whereParts, fmt.Sprintf("providers && $%d", paramCount))
		params = append(params, filter.Providers)
	}

	if len(filter.Tags) > 0 {
		paramCount++
		whereParts = append(whereParts, fmt.Sprintf("tags && $%d", paramCount))
		params = append(params, filter.Tags)
	}

	if len(whereParts) == 0 {
		return "", params
	}

	return " AND " + strings.Join(whereParts, " AND "), params
}

func (r *PostgresRepository) buildTagFacetQuery(searchQuery string, includeAssets, includeGlossary, includeTeams, includeDataProducts bool, assetWhereClause string) string {
	var unions []string

	if includeAssets {
		unions = append(unions, `SELECT UNNEST(tags) as tag FROM assets WHERE is_stub = FALSE`+assetWhereClause)
	}

	if includeGlossary {
		if searchQuery != "" {
			unions = append(unions, `SELECT UNNEST(tags) as tag FROM glossary_terms WHERE deleted_at IS NULL AND search_text @@ websearch_to_tsquery('english', $1)`)
		} else {
			unions = append(unions, `SELECT UNNEST(tags) as tag FROM glossary_terms WHERE deleted_at IS NULL`)
		}
	}

	if includeTeams {
		if searchQuery != "" {
			unions = append(unions, `SELECT UNNEST(tags) as tag FROM teams WHERE search_text @@ websearch_to_tsquery('english', $1)`)
		} else {
			unions = append(unions, `SELECT UNNEST(tags) as tag FROM teams`)
		}
	}

	if includeDataProducts {
		if searchQuery != "" {
			unions = append(unions, `SELECT UNNEST(tags) as tag FROM data_products WHERE search_text @@ websearch_to_tsquery('english', $1)`)
		} else {
			unions = append(unions, `SELECT UNNEST(tags) as tag FROM data_products`)
		}
	}

	if len(unions) == 0 {
		return ""
	}

	return fmt.Sprintf(`SELECT tag, COUNT(*) as count FROM (%s) sub WHERE tag IS NOT NULL GROUP BY tag ORDER BY count DESC LIMIT 50`, strings.Join(unions, " UNION ALL "))
}
