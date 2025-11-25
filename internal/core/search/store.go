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
	Search(ctx context.Context, filter Filter) ([]*Result, int, map[ResultType]int, error)
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

func (r *PostgresRepository) Search(ctx context.Context, filter Filter) ([]*Result, int, map[ResultType]int, error) {
	start := time.Now()

	// Build the unified search query with type filtering
	query, params := r.buildUnifiedSearchQuery(filter)

	rows, err := r.db.Query(ctx, query, params...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("executing unified search: %w", err)
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
			r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
			return nil, 0, nil, fmt.Errorf("scanning search result: %w", err)
		}

		result.Description = description
		result.UpdatedAt = updatedAt

		if len(metadataJSON) > 0 {
			result.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
				return nil, 0, nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		results = append(results, &result)
	}

	if rows.Err() != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("iterating search results: %w", rows.Err())
	}

	// Get total count and facets
	countQuery, countParams := r.buildCountQuery(filter)

	var total, assetCount, glossaryCount, teamCount, userCount int
	err = r.db.QueryRow(ctx, countQuery, countParams...).Scan(&total, &assetCount, &glossaryCount, &teamCount, &userCount)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search_count", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("counting search results: %w", err)
	}

	facets := map[ResultType]int{
		ResultTypeAsset:    assetCount,
		ResultTypeGlossary: glossaryCount,
		ResultTypeTeam:     teamCount,
		ResultTypeUser:     userCount,
	}

	r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), true)
	return results, total, facets, nil
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

// BuildUnifiedSearchQuery builds the unified search query with proper type filtering
func (r *PostgresRepository) buildUnifiedSearchQuery(filter Filter) (string, []interface{}) {
	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)
	includeUsers := searchTypeIncluded(filter.Types, ResultTypeUser)

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
					'/assets/' || id as url,
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
			// Use traditional full-text search
			var rankExpr string
			var whereClause string

			if filter.Query != "" {
				// With search query - use full-text search and ranking
				rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
				whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1) AND is_stub = FALSE"
			} else {
				// Without search query - return all, order by updated_at
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
					'/assets/' || id as url,
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

	if includeUsers {
		var rankExpr, whereClause string
		if filter.Query != "" {
			rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
			whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1) AND active = TRUE"
		} else {
			rankExpr = "0"
			whereClause = "WHERE active = TRUE"
		}

		unions = append(unions, fmt.Sprintf(`
			SELECT
				'user' as type,
				id::text,
				name,
				username as description,
				jsonb_build_object('username', username, 'active', active) as metadata,
				'/users/' || id as url,
				%s as rank,
				updated_at
			FROM users
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

// buildCountQuery builds the count query with facets for search results
func (r *PostgresRepository) buildCountQuery(filter Filter) (string, []interface{}) {
	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)
	includeUsers := searchTypeIncluded(filter.Types, ResultTypeUser)

	var unions []string
	var params []interface{}
	paramCount := 0

	// Only add query parameter if it's being used
	if filter.Query != "" {
		params = append(params, filter.Query)
		paramCount = 1
	}

	if includeAssets {
		var assetCountQuery string
		if filter.Query != "" {
			assetCountQuery = `SELECT 'asset' as type FROM assets WHERE search_text @@ websearch_to_tsquery('english', $1) AND is_stub = FALSE`
		} else {
			assetCountQuery = `SELECT 'asset' as type FROM assets WHERE is_stub = FALSE`
		}

		// Add asset type filter
		if len(filter.AssetTypes) > 0 {
			paramCount++
			assetCountQuery += fmt.Sprintf(" AND type = ANY($%d)", paramCount)
			params = append(params, filter.AssetTypes)
		}

		// Add provider filter
		if len(filter.Providers) > 0 {
			paramCount++
			assetCountQuery += fmt.Sprintf(" AND providers && $%d", paramCount)
			params = append(params, filter.Providers)
		}

		// Add tags filter
		if len(filter.Tags) > 0 {
			paramCount++
			assetCountQuery += fmt.Sprintf(" AND tags && $%d", paramCount)
			params = append(params, filter.Tags)
		}

		unions = append(unions, assetCountQuery)
	}

	if includeGlossary {
		if filter.Query != "" {
			unions = append(unions, `
				SELECT 'glossary' as type FROM glossary_terms
				WHERE search_text @@ websearch_to_tsquery('english', $1) AND deleted_at IS NULL
			`)
		} else {
			unions = append(unions, `
				SELECT 'glossary' as type FROM glossary_terms
				WHERE deleted_at IS NULL
			`)
		}
	}

	if includeTeams {
		if filter.Query != "" {
			unions = append(unions, `
				SELECT 'team' as type FROM teams
				WHERE search_text @@ websearch_to_tsquery('english', $1)
			`)
		} else {
			unions = append(unions, `
				SELECT 'team' as type FROM teams
			`)
		}
	}

	if includeUsers {
		if filter.Query != "" {
			unions = append(unions, `
				SELECT 'user' as type FROM users
				WHERE search_text @@ websearch_to_tsquery('english', $1) AND active = TRUE
			`)
		} else {
			unions = append(unions, `
				SELECT 'user' as type FROM users
				WHERE active = TRUE
			`)
		}
	}

	if len(unions) == 0 {
		// No types selected
		return "SELECT 0 as total, 0 as asset_count, 0 as glossary_count, 0 as team_count, 0 as user_count", []interface{}{}
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN type = 'asset' THEN 1 ELSE 0 END), 0) as asset_count,
			COALESCE(SUM(CASE WHEN type = 'glossary' THEN 1 ELSE 0 END), 0) as glossary_count,
			COALESCE(SUM(CASE WHEN type = 'team' THEN 1 ELSE 0 END), 0) as team_count,
			COALESCE(SUM(CASE WHEN type = 'user' THEN 1 ELSE 0 END), 0) as user_count
		FROM (
			%s
		) counts
	`, strings.Join(unions, " UNION ALL "))

	return query, params
}
