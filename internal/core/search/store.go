package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	total := facets.Types[ResultTypeAsset] + facets.Types[ResultTypeGlossary] + facets.Types[ResultTypeTeam] + facets.Types[ResultTypeDataProduct]
	return results, total, facets, nil
}

// scanResults scans search result rows into Result structs.
func scanResults(rows pgx.Rows) ([]*Result, error) {
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
			return nil, err
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
		return nil, rows.Err()
	}

	return results, nil
}

func (r *PostgresRepository) searchFullText(ctx context.Context, filter Filter) ([]*Result, error) {
	q, params := r.buildFullTextQuery(filter)

	rows, err := r.db.Query(ctx, q, params...)
	if err != nil {
		return nil, fmt.Errorf("executing full-text search: %w", err)
	}
	defer rows.Close()

	results, err := scanResults(rows)
	if err != nil {
		return nil, fmt.Errorf("scanning full-text search results: %w", err)
	}
	return results, nil
}

// renumberParameters renumbers SQL parameters from $2, $3, ... to $1, $2, ...
func renumberParameters(sql string) string {
	for i := 2; i <= 20; i++ {
		old := fmt.Sprintf("$%d", i)
		placeholder := fmt.Sprintf("__PARAM_%d__", i-1)
		sql = strings.ReplaceAll(sql, old, placeholder)
	}
	for i := 1; i <= 19; i++ {
		placeholder := fmt.Sprintf("__PARAM_%d__", i)
		newParam := fmt.Sprintf("$%d", i)
		sql = strings.ReplaceAll(sql, placeholder, newParam)
	}
	return sql
}

func (r *PostgresRepository) buildFullTextQuery(filter Filter) (string, []interface{}) {
	kindFilters := extractKindFilters(filter.Query)
	if len(kindFilters) > 0 {
		if len(kindFilters) == 1 && kindFilters[0] == "__CONTRADICTION__" {
			return "SELECT NULL as type, NULL as id, NULL as name, NULL as description, NULL as metadata, NULL as url, 0 as rank, NULL as updated_at WHERE FALSE", []interface{}{}
		}
		filter.Types = kindFilters
	}

	searchQuery := stripKindFilter(filter.Query)

	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)
	includeDataProducts := searchTypeIncluded(filter.Types, ResultTypeDataProduct)

	var unions []string
	var params []interface{}
	paramCount := 0

	if searchQuery != "" {
		params = append(params, searchQuery)
		paramCount = 1
	}

	if includeAssets {
		assetQuery, newParams, newCount := r.buildAssetFullTextQuery(searchQuery, filter, params, paramCount)
		params = newParams
		paramCount = newCount
		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		q := r.buildGlossaryFullTextQuery(searchQuery, filter, paramCount)
		unions = append(unions, q)
	}

	if includeTeams {
		q := r.buildTeamFullTextQuery(searchQuery, filter, paramCount)
		unions = append(unions, q)
	}

	if includeDataProducts {
		q := r.buildDataProductFullTextQuery(searchQuery, filter, paramCount)
		unions = append(unions, q)
	}

	if len(unions) == 0 {
		return "SELECT NULL as type, NULL as id, NULL as name, NULL as description, NULL as metadata, NULL as url, 0 as rank, NULL as updated_at WHERE FALSE", []interface{}{}
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount

	q := fmt.Sprintf(`
		WITH search_results AS (
			%s
		)
		SELECT * FROM search_results
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(unions, " UNION ALL "), limitParam, offsetParam)

	params = append(params, filter.Limit, filter.Offset)
	return q, params
}

func (r *PostgresRepository) buildAssetFullTextQuery(searchQuery string, filter Filter, params []interface{}, paramCount int) (string, []interface{}, int) {
	hasStructuredQuery := searchQuery != "" && (strings.Contains(searchQuery, "@metadata.") ||
		strings.Contains(searchQuery, "@type") ||
		strings.Contains(searchQuery, "@provider") ||
		strings.Contains(searchQuery, "@name"))

	if hasStructuredQuery {
		parser := query.NewParser()
		builder := query.NewBuilder()

		parsedQuery, err := parser.Parse(searchQuery)
		if err == nil {
			baseQuery := fmt.Sprintf(`SELECT
				'asset' as type, id, name, description,
				%s,
				%s,
				1.0 as rank, updated_at
			FROM assets`, assetMetadataColumns, assetURLExpr)

			builtQuery, queryParams, err := builder.BuildSQL(parsedQuery, baseQuery)
			if err == nil {
				builtQuery = strings.TrimPrefix(builtQuery, "WITH search_results AS (")
				builtQuery = strings.TrimSuffix(builtQuery, ") SELECT * FROM search_results ORDER BY search_rank DESC")

				if strings.Contains(builtQuery, "WHERE") {
					builtQuery += " AND is_stub = FALSE"
				} else {
					builtQuery += " WHERE is_stub = FALSE"
				}

				builtQuery = renumberParameters(builtQuery)

				if len(queryParams) > 1 {
					params = queryParams[1:]
					paramCount = len(params)
				} else {
					params = []interface{}{}
					paramCount = 0
				}

				return r.addAssetFilters(builtQuery, filter, params, paramCount)
			}
		}
	}

	var rankExpr, whereClause string
	hasSearchQuery := searchQuery != ""

	if hasSearchQuery {
		rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
		whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1) AND is_stub = FALSE"
	} else {
		rankExpr = "0"
		whereClause = "WHERE is_stub = FALSE"
	}

	assetQuery := fmt.Sprintf(`
		SELECT
			'asset' as type, id, name, description,
			%s,
			%s,
			%s as rank, updated_at
		FROM assets
		%s`, assetMetadataColumns, assetURLExpr, rankExpr, whereClause)

	assetQuery, params, paramCount = r.addAssetFilters(assetQuery, filter, params, paramCount)

	if !hasSearchQuery {
		assetQuery = fmt.Sprintf(`SELECT * FROM (%s ORDER BY updated_at DESC LIMIT %d) sub`, assetQuery, filter.Limit+filter.Offset)
	}

	return assetQuery, params, paramCount
}

func (r *PostgresRepository) addAssetFilters(q string, filter Filter, params []interface{}, paramCount int) (string, []interface{}, int) {
	if len(filter.AssetTypes) > 0 {
		paramCount++
		q += fmt.Sprintf(" AND type = ANY($%d)", paramCount)
		params = append(params, filter.AssetTypes)
	}
	if len(filter.Providers) > 0 {
		paramCount++
		q += fmt.Sprintf(" AND providers && $%d", paramCount)
		params = append(params, filter.Providers)
	}
	if len(filter.Tags) > 0 {
		paramCount++
		q += fmt.Sprintf(" AND tags && $%d", paramCount)
		params = append(params, filter.Tags)
	}
	return q, params, paramCount
}

func (r *PostgresRepository) buildGlossaryFullTextQuery(searchQuery string, filter Filter, paramCount int) string {
	hasStructuredQuery := searchQuery != "" && (strings.Contains(searchQuery, "@metadata.") ||
		strings.Contains(searchQuery, "@name"))

	if hasStructuredQuery {
		parser := query.NewParser()
		builder := query.NewBuilder()

		parsedQuery, err := parser.Parse(searchQuery)
		if err == nil {
			baseQuery := fmt.Sprintf(`SELECT
				'glossary' as type, id::text, name, definition as description,
				%s,
				%s,
				1.0 as rank, updated_at
			FROM glossary_terms`, glossaryMetadataColumns, glossaryURLExpr)

			builtQuery, _, err := builder.BuildSQL(parsedQuery, baseQuery)
			if err == nil {
				builtQuery = strings.TrimPrefix(builtQuery, "WITH search_results AS (")
				builtQuery = strings.TrimSuffix(builtQuery, ") SELECT * FROM search_results ORDER BY search_rank DESC")

				if strings.Contains(builtQuery, "WHERE") {
					builtQuery += " AND deleted_at IS NULL"
				} else {
					builtQuery += " WHERE deleted_at IS NULL"
				}

				return renumberParameters(builtQuery)
			}
		}
	}

	var rankExpr, whereClause string
	hasSearchQuery := searchQuery != ""

	if hasSearchQuery {
		rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
		whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1) AND deleted_at IS NULL"
	} else {
		rankExpr = "0"
		whereClause = "WHERE deleted_at IS NULL"
	}

	q := fmt.Sprintf(`
		SELECT
			'glossary' as type, id::text, name, definition as description,
			%s,
			%s,
			%s as rank, updated_at
		FROM glossary_terms
		%s
	`, glossaryMetadataColumns, glossaryURLExpr, rankExpr, whereClause)

	if !hasSearchQuery {
		q = fmt.Sprintf(`SELECT * FROM (%s ORDER BY updated_at DESC LIMIT %d) sub`, q, filter.Limit+filter.Offset)
	}

	return q
}

func (r *PostgresRepository) buildTeamFullTextQuery(searchQuery string, filter Filter, paramCount int) string {
	hasStructuredQuery := searchQuery != "" && (strings.Contains(searchQuery, "@metadata.") ||
		strings.Contains(searchQuery, "@name"))

	if hasStructuredQuery {
		parser := query.NewParser()
		builder := query.NewBuilder()

		parsedQuery, err := parser.Parse(searchQuery)
		if err == nil {
			baseQuery := fmt.Sprintf(`SELECT
				'team' as type, id::text, name, description,
				%s,
				%s,
				1.0 as rank, updated_at
			FROM teams`, teamMetadataColumns, teamURLExpr)

			builtQuery, _, err := builder.BuildSQL(parsedQuery, baseQuery)
			if err == nil {
				builtQuery = strings.TrimPrefix(builtQuery, "WITH search_results AS (")
				builtQuery = strings.TrimSuffix(builtQuery, ") SELECT * FROM search_results ORDER BY search_rank DESC")
				return renumberParameters(builtQuery)
			}
		}
	}

	var rankExpr, whereClause string
	hasSearchQuery := searchQuery != ""

	if hasSearchQuery {
		rankExpr = "ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32)"
		whereClause = "WHERE search_text @@ websearch_to_tsquery('english', $1)"
	} else {
		rankExpr = "0"
		whereClause = ""
	}

	q := fmt.Sprintf(`
		SELECT
			'team' as type, id::text, name, description,
			%s,
			%s,
			%s as rank, updated_at
		FROM teams
		%s
	`, teamMetadataColumns, teamURLExpr, rankExpr, whereClause)

	if !hasSearchQuery {
		q = fmt.Sprintf(`SELECT * FROM (%s ORDER BY updated_at DESC LIMIT %d) sub`, q, filter.Limit+filter.Offset)
	}

	return q
}

func (r *PostgresRepository) buildDataProductFullTextQuery(searchQuery string, filter Filter, paramCount int) string {
	hasStructuredQuery := searchQuery != "" && (strings.Contains(searchQuery, "@metadata.") ||
		strings.Contains(searchQuery, "@name"))

	if hasStructuredQuery {
		parser := query.NewParser()
		builder := query.NewBuilder()

		parsedQuery, err := parser.Parse(searchQuery)
		if err == nil {
			baseQuery := fmt.Sprintf(`SELECT
				'data_product' as type, dp.id::text, dp.name, dp.description,
				%s,
				%s,
				1.0 as rank, dp.updated_at
			FROM data_products dp
			LEFT JOIN product_images pi ON dp.id = pi.data_product_id AND pi.purpose = 'icon'`, dataProductMetadataColumns, dataProductURLExpr)

			builtQuery, _, err := builder.BuildSQL(parsedQuery, baseQuery)
			if err == nil {
				builtQuery = strings.TrimPrefix(builtQuery, "WITH search_results AS (")
				builtQuery = strings.TrimSuffix(builtQuery, ") SELECT * FROM search_results ORDER BY search_rank DESC")
				return renumberParameters(builtQuery)
			}
		}
	}

	var rankExpr, whereClause string
	hasSearchQuery := searchQuery != ""

	if hasSearchQuery {
		rankExpr = "ts_rank_cd(dp.search_text, websearch_to_tsquery('english', $1), 32)"
		whereClause = "WHERE dp.search_text @@ websearch_to_tsquery('english', $1)"
	} else {
		rankExpr = "0"
		whereClause = ""
	}

	q := fmt.Sprintf(`
		SELECT
			'data_product' as type, dp.id::text, dp.name, dp.description,
			%s,
			%s,
			%s as rank, dp.updated_at
		FROM data_products dp
		LEFT JOIN product_images pi ON dp.id = pi.data_product_id AND pi.purpose = 'icon'
		%s
	`, dataProductMetadataColumns, dataProductURLExpr, rankExpr, whereClause)

	if !hasSearchQuery {
		q = fmt.Sprintf(`SELECT * FROM (%s ORDER BY updated_at DESC LIMIT %d) sub`, q, filter.Limit+filter.Offset)
	}

	return q
}

func (r *PostgresRepository) searchExactMatch(ctx context.Context, filter Filter) ([]*Result, error) {
	if filter.Query == "" {
		return nil, nil
	}

	kindFilters := extractKindFilters(filter.Query)
	if len(kindFilters) > 0 {
		if len(kindFilters) == 1 && kindFilters[0] == "__CONTRADICTION__" {
			return nil, nil
		}
		filter.Types = kindFilters
	}

	searchQuery := stripKindFilter(filter.Query)
	if searchQuery == "" {
		return nil, nil
	}

	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)
	includeDataProducts := searchTypeIncluded(filter.Types, ResultTypeDataProduct)

	var unions []string
	var params []interface{}
	paramCount := 0

	paramLower := strings.ToLower(searchQuery)
	params = append(params, paramLower)
	params = append(params, paramLower+"%")
	paramCount = 2

	if includeAssets {
		assetQuery := fmt.Sprintf(`
			SELECT
				'asset' as type, id, name, description,
				%s,
				%s,
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
			)`, assetMetadataColumns, assetURLExpr)

		assetQuery, params, paramCount = r.addAssetFilters(assetQuery, filter, params, paramCount)
		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		unions = append(unions, fmt.Sprintf(`
			SELECT
				'glossary' as type, id::text, name, definition as description,
				%s,
				%s,
				CASE
					WHEN LOWER(name) = $1 THEN 100
					WHEN LOWER(name) LIKE $2 THEN 50
					ELSE 25
				END as rank,
				updated_at
			FROM glossary_terms
			WHERE deleted_at IS NULL
			AND (LOWER(name) = $1 OR LOWER(name) LIKE $2)
		`, glossaryMetadataColumns, glossaryURLExpr))
	}

	if includeTeams {
		unions = append(unions, fmt.Sprintf(`
			SELECT
				'team' as type, id::text, name, description,
				%s,
				%s,
				CASE
					WHEN LOWER(name) = $1 THEN 100
					WHEN LOWER(name) LIKE $2 THEN 50
					ELSE 25
				END as rank,
				updated_at
			FROM teams
			WHERE (LOWER(name) = $1 OR LOWER(name) LIKE $2)
		`, teamMetadataColumns, teamURLExpr))
	}

	if includeDataProducts {
		unions = append(unions, fmt.Sprintf(`
			SELECT
				'data_product' as type, dp.id::text, dp.name, dp.description,
				%s,
				%s,
				CASE
					WHEN LOWER(dp.name) = $1 THEN 100
					WHEN LOWER(dp.name) LIKE $2 THEN 50
					ELSE 25
				END as rank,
				dp.updated_at
			FROM data_products dp
			LEFT JOIN product_images pi ON dp.id = pi.data_product_id AND pi.purpose = 'icon'
			WHERE (LOWER(dp.name) = $1 OR LOWER(dp.name) LIKE $2)
		`, dataProductMetadataColumns, dataProductURLExpr))
	}

	if len(unions) == 0 {
		return nil, nil
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount

	q := fmt.Sprintf(`
		WITH search_results AS (
			%s
		)
		SELECT * FROM search_results
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(unions, " UNION ALL "), limitParam, offsetParam)

	params = append(params, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, q, params...)
	if err != nil {
		return nil, fmt.Errorf("executing exact match search: %w", err)
	}
	defer rows.Close()

	results, err := scanResults(rows)
	if err != nil {
		return nil, fmt.Errorf("scanning exact match results: %w", err)
	}
	return results, nil
}

func (r *PostgresRepository) searchTrigramFuzzy(ctx context.Context, filter Filter) ([]*Result, error) {
	if filter.Query == "" {
		return nil, nil
	}

	kindFilters := extractKindFilters(filter.Query)
	if len(kindFilters) > 0 {
		if len(kindFilters) == 1 && kindFilters[0] == "__CONTRADICTION__" {
			return nil, nil
		}
		filter.Types = kindFilters
	}

	searchQuery := stripKindFilter(filter.Query)
	if searchQuery == "" {
		return nil, nil
	}

	includeAssets := searchTypeIncluded(filter.Types, ResultTypeAsset)
	includeGlossary := searchTypeIncluded(filter.Types, ResultTypeGlossary)
	includeTeams := searchTypeIncluded(filter.Types, ResultTypeTeam)
	includeDataProducts := searchTypeIncluded(filter.Types, ResultTypeDataProduct)

	var unions []string
	var params []interface{}
	paramCount := 0

	params = append(params, searchQuery)
	paramCount = 1

	if includeAssets {
		assetQuery := fmt.Sprintf(`
			SELECT
				'asset' as type, id, name, description,
				%s,
				%s,
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
			)`, assetMetadataColumns, assetURLExpr)

		assetQuery, params, paramCount = r.addAssetFilters(assetQuery, filter, params, paramCount)
		unions = append(unions, assetQuery)
	}

	if includeGlossary {
		unions = append(unions, fmt.Sprintf(`
			SELECT
				'glossary' as type, id::text, name, definition as description,
				%s,
				%s,
				(word_similarity($1, name) * 30) as rank,
				updated_at
			FROM glossary_terms
			WHERE deleted_at IS NULL
			AND word_similarity($1, name) > 0.3
		`, glossaryMetadataColumns, glossaryURLExpr))
	}

	if includeTeams {
		unions = append(unions, fmt.Sprintf(`
			SELECT
				'team' as type, id::text, name, description,
				%s,
				%s,
				(word_similarity($1, name) * 30) as rank,
				updated_at
			FROM teams
			WHERE word_similarity($1, name) > 0.3
		`, teamMetadataColumns, teamURLExpr))
	}

	if includeDataProducts {
		unions = append(unions, fmt.Sprintf(`
			SELECT
				'data_product' as type, dp.id::text, dp.name, dp.description,
				%s,
				%s,
				(word_similarity($1, dp.name) * 30) as rank,
				dp.updated_at
			FROM data_products dp
			LEFT JOIN product_images pi ON dp.id = pi.data_product_id AND pi.purpose = 'icon'
			WHERE word_similarity($1, dp.name) > 0.3
		`, dataProductMetadataColumns, dataProductURLExpr))
	}

	if len(unions) == 0 {
		return nil, nil
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount

	q := fmt.Sprintf(`
		WITH search_results AS (
			%s
		)
		SELECT * FROM search_results
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(unions, " UNION ALL "), limitParam, offsetParam)

	params = append(params, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, q, params...)
	if err != nil {
		return nil, fmt.Errorf("executing trigram fuzzy search: %w", err)
	}
	defer rows.Close()

	results, err := scanResults(rows)
	if err != nil {
		return nil, fmt.Errorf("scanning trigram fuzzy results: %w", err)
	}
	return results, nil
}
