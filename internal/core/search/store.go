package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/query"
)

type Repository interface {
	Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error)
}

type PostgresRepository struct {
	db       *pgxpool.Pool
	recorder metrics.Recorder
}

func NewPostgresRepository(db *pgxpool.Pool, recorder metrics.Recorder) *PostgresRepository {
	return &PostgresRepository{
		db:       db,
		recorder: recorder,
	}
}

func (r *PostgresRepository) Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
	start := time.Now()
	var results []*Result
	var err error
	var searchMethod string

	if filter.Query != "" {
		results, err = r.searchExactMatch(ctx, filter)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "search_exact_match", time.Since(start), false)
		} else if len(results) > 0 {
			searchMethod = "exact_match"
		}

		if len(results) == 0 && err == nil {
			results, err = r.searchTrigramFuzzy(ctx, filter)
			if err != nil {
				r.recorder.RecordDBQuery(ctx, "search_trigram_fuzzy", time.Since(start), false)
			} else if len(results) > 0 {
				searchMethod = "trigram_fuzzy"
			}
		}

		if len(results) == 0 && err == nil {
			results, err = r.searchFullText(ctx, filter)
			if err != nil {
				r.recorder.RecordDBQuery(ctx, "search_full_text", time.Since(start), false)
				return nil, 0, nil, fmt.Errorf("executing full-text search: %w", err)
			}
			searchMethod = "full_text"
		}

		if err != nil && len(results) == 0 {
			return nil, 0, nil, fmt.Errorf("all search tiers failed: %w", err)
		}
	} else {
		results, err = r.searchFullText(ctx, filter)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "search_no_query", time.Since(start), false)
			return nil, 0, nil, fmt.Errorf("executing search without query: %w", err)
		}
		searchMethod = "no_query"
	}

	facets, err := r.buildFacets(ctx, filter)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("building facets: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "search_"+searchMethod, time.Since(start), true)
	return results, facets.Types[ResultTypeAsset] + facets.Types[ResultTypeGlossary] + facets.Types[ResultTypeTeam], facets, nil
}

// searchTypeIncluded checks if a type should be included in search
func searchTypeIncluded(types []ResultType, target ResultType) bool {
	if len(types) == 0 {
		return true // Include all if no filter
	}
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

func (r *PostgresRepository) searchFullText(ctx context.Context, filter Filter) ([]*Result, error) {
	query, params := r.buildFullTextQuery(filter)

	rows, err := r.db.Query(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("executing full-text search: %w", err)
	}
	defer rows.Close()

	var results []*Result
	for rows.Next() {
		var result Result
		var metadataJSON []byte
		var description *string
		var updatedAt *time.Time

		err := rows.Scan(
			&result.Type,
			&result.ID,
			&result.Name,
			&description,
			&metadataJSON,
			&result.URL,
			&result.Rank,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning full-text search result: %w", err)
		}

		result.Description = description
		result.UpdatedAt = updatedAt

		if len(metadataJSON) > 0 {
			result.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		results = append(results, &result)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating full-text search results: %w", rows.Err())
	}

	return results, nil
}

func (r *PostgresRepository) buildFullTextQuery(filter Filter) (string, []interface{}) {
	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)

	var unions []string
	var params []interface{}
	paramCount := 0

	// Only add query parameter if it's being used
	if filter.Query != "" {
		params = append(params, filter.Query)
		paramCount = 1
	}

	if includeAssets {
		var assetQuery string

		// Check if query has structured syntax (@metadata., @kind, @type, @provider, @name, etc.)
		hasStructuredQuery := filter.Query != "" && (strings.Contains(filter.Query, "@metadata.") ||
			strings.Contains(filter.Query, "@kind") ||
			strings.Contains(filter.Query, "@type") ||
			strings.Contains(filter.Query, "@provider") ||
			strings.Contains(filter.Query, "@name"))

		if hasStructuredQuery {
			// Use query parser for structured queries
			parser := query.NewParser()
			builder := query.NewBuilder()

			parsedQuery, err := parser.Parse(filter.Query)
			if err != nil {
				// If parsing fails, fall back to full-text search
				hasStructuredQuery = false
			} else {
				baseQuery := `SELECT
					'asset' as type,
					id,
					name,
					description,
					jsonb_build_object(
						'id', id,
						'name', name,
						'mrn', mrn,
						'type', type,
						'providers', providers,
						'environments', environments,
						'external_links', external_links,
						'description', description,
						'user_description', user_description,
						'metadata', metadata,
						'schema', schema,
						'sources', sources,
						'tags', tags,
						'created_at', created_at,
						'created_by', created_by,
						'updated_at', updated_at,
						'last_sync_at', last_sync_at,
						'query', query,
						'query_language', query_language,
						'is_stub', is_stub
					) as metadata,
					'/discover/' || type || '/' || name as url,
					ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as rank,
					updated_at
				FROM assets`

				builtQuery, queryParams, err := builder.BuildSQL(parsedQuery, baseQuery)
				if err == nil {
					// Extract the inner query from the CTE wrapper if present
					builtQuery = strings.TrimPrefix(builtQuery, "WITH search_results AS (")
					builtQuery = strings.TrimSuffix(builtQuery, ") SELECT * FROM search_results ORDER BY search_rank DESC")

					// Add is_stub filter
					if strings.Contains(builtQuery, "WHERE") {
						builtQuery += " AND is_stub = FALSE"
					} else {
						builtQuery += " WHERE is_stub = FALSE"
					}

					assetQuery = builtQuery

					// Update params to include the query params
					if len(queryParams) > 0 {
						params = queryParams
						paramCount = len(params)
					}
				} else {
					// If building fails, fall back to full-text search
					hasStructuredQuery = false
				}
			}
		}

		if !hasStructuredQuery {
			var rankExpr string
			var whereClause string

			if filter.Query != "" {
				rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
				whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1) AND is_stub = FALSE"
			} else {
				rankExpr = "0"
				whereClause = "WHERE is_stub = FALSE"
			}

			assetQuery = fmt.Sprintf(`
				SELECT
					'asset' as type,
					id,
					name,
					description,
					jsonb_build_object(
						'id', id,
						'name', name,
						'mrn', mrn,
						'type', type,
						'providers', providers,
						'environments', environments,
						'external_links', external_links,
						'description', description,
						'user_description', user_description,
						'metadata', metadata,
						'schema', schema,
						'sources', sources,
						'tags', tags,
						'created_at', created_at,
						'created_by', created_by,
						'updated_at', updated_at,
						'last_sync_at', last_sync_at,
						'query', query,
						'query_language', query_language,
						'is_stub', is_stub
					) as metadata,
					'/discover/' || type || '/' || name as url,
					%s as rank,
					updated_at
				FROM assets
				%s`, rankExpr, whereClause)
		}

		// Add asset type filter
		if len(filter.AssetTypes) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND type = ANY($%d)", paramCount)
			params = append(params, filter.AssetTypes)
		}

		// Add provider filter
		if len(filter.Providers) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND providers && $%d", paramCount)
			params = append(params, filter.Providers)
		}

		// Add tags filter
		if len(filter.Tags) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND tags && $%d", paramCount)
			params = append(params, filter.Tags)
		}

		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		var rankExpr, whereClause string
		if filter.Query != "" {
			rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
			whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1) AND deleted_at IS NULL"
		} else {
			rankExpr = "0"
			whereClause = "WHERE deleted_at IS NULL"
		}

		unions = append(unions, fmt.Sprintf(`
			SELECT
				'glossary' as type,
				id::text,
				name,
				definition as description,
				jsonb_build_object('parent_term_id', parent_term_id) as metadata,
				'/glossary/' || id::text as url,
				%s as rank,
				updated_at
			FROM glossary_terms
			%s
		`, rankExpr, whereClause))
	}

	if includeTeams {
		var rankExpr, whereClause string
		if filter.Query != "" {
			rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
			whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1)"
		} else {
			rankExpr = "0"
			whereClause = ""
		}

		unions = append(unions, fmt.Sprintf(`
			SELECT
				'team' as type,
				id::text,
				name,
				description,
				jsonb_build_object('created_via_sso', created_via_sso) as metadata,
				'/teams/' || id as url,
				%s as rank,
				updated_at
			FROM teams
			%s
		`, rankExpr, whereClause))
	}

	if len(unions) == 0 {
		// No types selected, return empty query
		return "SELECT NULL as type, NULL as id, NULL as name, NULL as description, NULL as metadata, NULL as url, 0 as rank, NULL as updated_at WHERE FALSE", []interface{}{}
	}

	// Add limit and offset parameters
	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount

	query := fmt.Sprintf(`
		WITH search_results AS (
			%s
		)
		SELECT * FROM search_results
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(unions, " UNION ALL "), limitParam, offsetParam)

	params = append(params, filter.Limit, filter.Offset)
	return query, params
}

func (r *PostgresRepository) buildFacets(ctx context.Context, filter Filter) (*Facets, error) {
	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)

	var unions []string
	var params []interface{}
	paramCount := 0

	if filter.Query != "" {
		params = append(params, filter.Query)
		paramCount = 1
	}

	if includeAssets {
		var assetQuery string
		if filter.Query != "" {
			assetQuery = `SELECT 'asset' as type, type as asset_type, providers, tags FROM assets WHERE search_text @@ websearch_to_tsquery('english', $1) AND is_stub = FALSE`
		} else {
			assetQuery = `SELECT 'asset' as type, type as asset_type, providers, tags FROM assets WHERE is_stub = FALSE`
		}

		if len(filter.AssetTypes) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND type = ANY($%d)", paramCount)
			params = append(params, filter.AssetTypes)
		}

		if len(filter.Providers) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND providers && $%d", paramCount)
			params = append(params, filter.Providers)
		}

		if len(filter.Tags) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND tags && $%d", paramCount)
			params = append(params, filter.Tags)
		}

		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		if filter.Query != "" {
			unions = append(unions, `SELECT 'glossary' as type, NULL as asset_type, NULL::text[] as providers, NULL::text[] as tags FROM glossary_terms WHERE search_text @@ websearch_to_tsquery('english', $1) AND deleted_at IS NULL`)
		} else {
			unions = append(unions, `SELECT 'glossary' as type, NULL as asset_type, NULL::text[] as providers, NULL::text[] as tags FROM glossary_terms WHERE deleted_at IS NULL`)
		}
	}

	if includeTeams {
		if filter.Query != "" {
			unions = append(unions, `SELECT 'team' as type, NULL as asset_type, NULL::text[] as providers, NULL::text[] as tags FROM teams WHERE search_text @@ websearch_to_tsquery('english', $1)`)
		} else {
			unions = append(unions, `SELECT 'team' as type, NULL as asset_type, NULL::text[] as providers, NULL::text[] as tags FROM teams`)
		}
	}

	if len(unions) == 0 {
		return &Facets{
			Types:      map[ResultType]int{},
			AssetTypes: []FacetValue{},
			Providers:  []FacetValue{},
			Tags:       []FacetValue{},
		}, nil
	}

	query := fmt.Sprintf(`
		WITH matching_results AS (
			%s
		),
		type_counts AS (
			SELECT type, COUNT(*) as count
			FROM matching_results
			GROUP BY type
		),
		asset_type_counts AS (
			SELECT asset_type, COUNT(*) as count
			FROM matching_results
			WHERE type = 'asset' AND asset_type IS NOT NULL
			GROUP BY asset_type
			ORDER BY count DESC
			LIMIT 50
		),
		provider_counts AS (
			SELECT UNNEST(providers) as provider, COUNT(*) as count
			FROM matching_results
			WHERE type = 'asset' AND providers IS NOT NULL
			GROUP BY provider
			ORDER BY count DESC
			LIMIT 50
		),
		tag_counts AS (
			SELECT UNNEST(tags) as tag, COUNT(*) as count
			FROM matching_results
			WHERE type = 'asset' AND tags IS NOT NULL
			GROUP BY tag
			ORDER BY count DESC
			LIMIT 50
		)
		SELECT
			(SELECT COALESCE(json_object_agg(type, count), '{}'::json) FROM type_counts) as type_facets,
			(SELECT COALESCE(json_agg(json_build_object('value', asset_type, 'count', count) ORDER BY count DESC), '[]'::json) FROM asset_type_counts) as asset_type_facets,
			(SELECT COALESCE(json_agg(json_build_object('value', provider, 'count', count) ORDER BY count DESC), '[]'::json) FROM provider_counts) as provider_facets,
			(SELECT COALESCE(json_agg(json_build_object('value', tag, 'count', count) ORDER BY count DESC), '[]'::json) FROM tag_counts) as tag_facets
	`, strings.Join(unions, " UNION ALL "))

	var typeFacetsJSON, assetTypeFacetsJSON, providerFacetsJSON, tagFacetsJSON []byte
	err := r.db.QueryRow(ctx, query, params...).Scan(&typeFacetsJSON, &assetTypeFacetsJSON, &providerFacetsJSON, &tagFacetsJSON)
	if err != nil {
		return nil, fmt.Errorf("querying facets: %w", err)
	}

	facets := &Facets{
		Types:      make(map[ResultType]int),
		AssetTypes: []FacetValue{},
		Providers:  []FacetValue{},
		Tags:       []FacetValue{},
	}

	if len(typeFacetsJSON) > 0 {
		var typeMap map[string]int
		if err := json.Unmarshal(typeFacetsJSON, &typeMap); err != nil {
			return nil, fmt.Errorf("unmarshaling type facets: %w", err)
		}
		for k, v := range typeMap {
			facets.Types[ResultType(k)] = v
		}
	}

	if len(assetTypeFacetsJSON) > 0 {
		if err := json.Unmarshal(assetTypeFacetsJSON, &facets.AssetTypes); err != nil {
			return nil, fmt.Errorf("unmarshaling asset type facets: %w", err)
		}
	}

	if len(providerFacetsJSON) > 0 {
		if err := json.Unmarshal(providerFacetsJSON, &facets.Providers); err != nil {
			return nil, fmt.Errorf("unmarshaling provider facets: %w", err)
		}
	}

	if len(tagFacetsJSON) > 0 {
		if err := json.Unmarshal(tagFacetsJSON, &facets.Tags); err != nil {
			return nil, fmt.Errorf("unmarshaling tag facets: %w", err)
		}
	}

	return facets, nil
}

func (r *PostgresRepository) searchExactMatch(ctx context.Context, filter Filter) ([]*Result, error) {
	if filter.Query == "" {
		return nil, nil
	}

	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)

	var unions []string
	var params []interface{}
	paramCount := 0

	paramLower := strings.ToLower(filter.Query)
	params = append(params, paramLower)
	params = append(params, paramLower+"%")
	paramCount = 2

	if includeAssets {
		assetQuery := `
			SELECT
				'asset' as type,
				id,
				name,
				description,
				jsonb_build_object(
					'id', id,
					'name', name,
					'mrn', mrn,
					'type', type,
					'providers', providers,
					'environments', environments,
					'external_links', external_links,
					'description', description,
					'user_description', user_description,
					'metadata', metadata,
					'schema', schema,
					'sources', sources,
					'tags', tags,
					'created_at', created_at,
					'created_by', created_by,
					'updated_at', updated_at,
					'last_sync_at', last_sync_at,
					'query', query,
					'query_language', query_language,
					'is_stub', is_stub
				) as metadata,
				'/discover/' || type || '/' || name as url,
				CASE
					WHEN LOWER(name) = $1 THEN 100
					WHEN LOWER(mrn) = $1 THEN 100
					WHEN LOWER(name) LIKE $2 THEN 50
					WHEN LOWER(mrn) LIKE $2 THEN 50
					ELSE 25
				END as rank,
				updated_at
			FROM assets
			WHERE is_stub = FALSE
			AND (
				LOWER(name) = $1 OR
				LOWER(mrn) = $1 OR
				LOWER(type) = $1 OR
				LOWER(name) LIKE $2 OR
				LOWER(mrn) LIKE $2 OR
				LOWER(type) LIKE $2
			)`

		if len(filter.AssetTypes) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND type = ANY($%d)", paramCount)
			params = append(params, filter.AssetTypes)
		}

		if len(filter.Providers) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND providers && $%d", paramCount)
			params = append(params, filter.Providers)
		}

		if len(filter.Tags) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND tags && $%d", paramCount)
			params = append(params, filter.Tags)
		}

		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		unions = append(unions, `
			SELECT
				'glossary' as type,
				id::text,
				name,
				definition as description,
				jsonb_build_object('parent_term_id', parent_term_id) as metadata,
				'/glossary/' || id::text as url,
				CASE
					WHEN LOWER(name) = $1 THEN 100
					WHEN LOWER(name) LIKE $2 THEN 50
					ELSE 25
				END as rank,
				updated_at
			FROM glossary_terms
			WHERE deleted_at IS NULL
			AND (LOWER(name) = $1 OR LOWER(name) LIKE $2)
		`)
	}

	if includeTeams {
		unions = append(unions, `
			SELECT
				'team' as type,
				id::text,
				name,
				description,
				jsonb_build_object('created_via_sso', created_via_sso) as metadata,
				'/teams/' || id as url,
				CASE
					WHEN LOWER(name) = $1 THEN 100
					WHEN LOWER(name) LIKE $2 THEN 50
					ELSE 25
				END as rank,
				updated_at
			FROM teams
			WHERE (LOWER(name) = $1 OR LOWER(name) LIKE $2)
		`)
	}

	if len(unions) == 0 {
		return nil, nil
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount

	query := fmt.Sprintf(`
		WITH search_results AS (
			%s
		)
		SELECT * FROM search_results
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(unions, " UNION ALL "), limitParam, offsetParam)

	params = append(params, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("executing exact match search: %w", err)
	}
	defer rows.Close()

	var results []*Result
	for rows.Next() {
		var result Result
		var metadataJSON []byte
		var description *string
		var updatedAt *time.Time

		err := rows.Scan(
			&result.Type,
			&result.ID,
			&result.Name,
			&description,
			&metadataJSON,
			&result.URL,
			&result.Rank,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning exact match result: %w", err)
		}

		result.Description = description
		result.UpdatedAt = updatedAt

		if len(metadataJSON) > 0 {
			result.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		results = append(results, &result)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating exact match results: %w", rows.Err())
	}

	return results, nil
}

func (r *PostgresRepository) searchTrigramFuzzy(ctx context.Context, filter Filter) ([]*Result, error) {
	if filter.Query == "" {
		return nil, nil
	}

	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)

	var unions []string
	var params []interface{}
	paramCount := 0

	params = append(params, filter.Query)
	paramCount = 1

	if includeAssets {
		assetQuery := `
			SELECT
				'asset' as type,
				id,
				name,
				description,
				jsonb_build_object(
					'id', id,
					'name', name,
					'mrn', mrn,
					'type', type,
					'providers', providers,
					'environments', environments,
					'external_links', external_links,
					'description', description,
					'user_description', user_description,
					'metadata', metadata,
					'schema', schema,
					'sources', sources,
					'tags', tags,
					'created_at', created_at,
					'created_by', created_by,
					'updated_at', updated_at,
					'last_sync_at', last_sync_at,
					'query', query,
					'query_language', query_language,
					'is_stub', is_stub
				) as metadata,
				'/discover/' || type || '/' || name as url,
				(
					GREATEST(
						word_similarity($1, name),
						word_similarity($1, mrn),
						similarity($1, COALESCE(name, '') || ' ' || COALESCE(mrn, '') || ' ' || COALESCE(type, ''))
					) * 30 +
					(EXTRACT(EPOCH FROM NOW() - updated_at) / -86400.0 / 365.0)::numeric * 5
				) as rank,
				updated_at
			FROM assets
			WHERE is_stub = FALSE
			AND (
				word_similarity($1, name) > 0.3 OR
				word_similarity($1, mrn) > 0.3 OR
				similarity($1, COALESCE(name, '') || ' ' || COALESCE(mrn, '') || ' ' || COALESCE(type, '')) > 0.25
			)`

		if len(filter.AssetTypes) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND type = ANY($%d)", paramCount)
			params = append(params, filter.AssetTypes)
		}

		if len(filter.Providers) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND providers && $%d", paramCount)
			params = append(params, filter.Providers)
		}

		if len(filter.Tags) > 0 {
			paramCount++
			assetQuery += fmt.Sprintf(" AND tags && $%d", paramCount)
			params = append(params, filter.Tags)
		}

		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		unions = append(unions, `
			SELECT
				'glossary' as type,
				id::text,
				name,
				definition as description,
				jsonb_build_object('parent_term_id', parent_term_id) as metadata,
				'/glossary/' || id::text as url,
				(word_similarity($1, name) * 30) as rank,
				updated_at
			FROM glossary_terms
			WHERE deleted_at IS NULL
			AND word_similarity($1, name) > 0.3
		`)
	}

	if includeTeams {
		unions = append(unions, `
			SELECT
				'team' as type,
				id::text,
				name,
				description,
				jsonb_build_object('created_via_sso', created_via_sso) as metadata,
				'/teams/' || id as url,
				(word_similarity($1, name) * 30) as rank,
				updated_at
			FROM teams
			WHERE word_similarity($1, name) > 0.3
		`)
	}

	if len(unions) == 0 {
		return nil, nil
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount

	query := fmt.Sprintf(`
		WITH search_results AS (
			%s
		)
		SELECT * FROM search_results
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(unions, " UNION ALL "), limitParam, offsetParam)

	params = append(params, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("executing trigram fuzzy search: %w", err)
	}
	defer rows.Close()

	var results []*Result
	for rows.Next() {
		var result Result
		var metadataJSON []byte
		var description *string
		var updatedAt *time.Time

		err := rows.Scan(
			&result.Type,
			&result.ID,
			&result.Name,
			&description,
			&metadataJSON,
			&result.URL,
			&result.Rank,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning trigram fuzzy result: %w", err)
		}

		result.Description = description
		result.UpdatedAt = updatedAt

		if len(metadataJSON) > 0 {
			result.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		results = append(results, &result)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating trigram fuzzy results: %w", rows.Err())
	}

	return results, nil
}
