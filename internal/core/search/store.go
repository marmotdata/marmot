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

const (
	// DefaultSearchTimeout prevents runaway queries from exhausting connections
	DefaultSearchTimeout = 10 * time.Second

	// Maximum results for facet aggregations
	maxFacetResults = 50
)

type Repository interface {
	Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error)
	GetMetadata(ctx context.Context, resultType ResultType, ids []string) (map[string]map[string]interface{}, error)
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

// Search performs a unified search across all entity types using the search_index table.
// It routes queries to optimized paths based on query characteristics.
func (r *PostgresRepository) Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, DefaultSearchTimeout)
	defer cancel()

	kindFilters := extractKindFilters(filter.Query)
	if len(kindFilters) > 0 {
		if len(kindFilters) == 1 && kindFilters[0] == "__CONTRADICTION__" {
			return []*Result{}, 0, emptyFacets(), nil
		}
		filter.Types = kindFilters
	}

	searchQuery := stripKindFilter(filter.Query)

	parser := query.NewParser()
	parsedQuery, err := parser.Parse(searchQuery)
	if err != nil {
		parsedQuery = &query.Query{FreeText: searchQuery}
	}

	sqlQuery, params := r.buildOptimizedSearchQuery(parsedQuery.GetFreeText(), filter, parsedQuery)

	rows, err := r.db.Query(ctx, sqlQuery, params...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("executing unified search: %w", err)
	}
	defer rows.Close()

	results, err := r.scanSearchResults(rows)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("scanning search results: %w", err)
	}

	facets, total, err := r.buildFacetsParallel(ctx, parsedQuery.GetFreeText(), filter, parsedQuery)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), false)
		return nil, 0, nil, fmt.Errorf("building facets: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "unified_search", time.Since(start), true)
	return results, total, facets, nil
}

// queryType determines which search strategy to use
type queryType int

const (
	queryTypeEmpty queryType = iota
	queryTypeExact
	queryTypePrefix
	queryTypeFuzzy
	queryTypeFullText
)

// classifyQuery determines the optimal search strategy based on query characteristics
func classifyQuery(q string) queryType {
	q = strings.TrimSpace(q)
	if q == "" {
		return queryTypeEmpty
	}

	words := strings.Fields(q)
	hasSpecialChars := strings.ContainsAny(q, "\"'*?+-|&()~:")

	if len(q) <= 2 {
		return queryTypePrefix
	}

	if len(words) > 1 || hasSpecialChars {
		return queryTypeFullText
	}

	return queryTypeFuzzy
}

// buildOptimizedSearchQuery routes to the appropriate query builder based on query type
func (r *PostgresRepository) buildOptimizedSearchQuery(searchQuery string, filter Filter, parsedQuery *query.Query) (string, []interface{}) {
	qType := classifyQuery(searchQuery)

	switch qType {
	case queryTypeEmpty:
		return r.buildListingQuery(filter, parsedQuery)
	case queryTypePrefix:
		return r.buildPrefixSearchQuery(searchQuery, filter, parsedQuery)
	case queryTypeFuzzy:
		return r.buildFuzzySearchQuery(searchQuery, filter, parsedQuery)
	case queryTypeFullText:
		return r.buildFullTextSearchQuery(searchQuery, filter, parsedQuery)
	default:
		return r.buildFullTextSearchQuery(searchQuery, filter, parsedQuery)
	}
}

// buildListingQuery handles empty queries with filters and sorts by recency.
func (r *PostgresRepository) buildListingQuery(filter Filter, parsedQuery *query.Query) (string, []interface{}) {
	var params []interface{}
	paramCount := 0

	whereClauses, params, paramCount := r.buildFilterClauses(filter, parsedQuery, params, paramCount)

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount
	params = append(params, filter.Limit, filter.Offset)

	canUseIndexSort := r.canUseIndexedSort(parsedQuery, filter)

	if canUseIndexSort {
		sqlQuery := fmt.Sprintf(`
			SELECT
				type, entity_id, name, description, url_path,
				0.0::real as rank,
				updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
			FROM search_index
			%s
			ORDER BY updated_at DESC
			LIMIT $%d OFFSET $%d
		`, whereSQL, limitParam, offsetParam)
		return sqlQuery, params
	}

	if parsedQuery.HasStructuredFilters() || len(filter.Tags) > 0 {
		sqlQuery := fmt.Sprintf(`
			SELECT
				type, entity_id, name, description, url_path,
				0.0::real as rank,
				updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
			FROM (
				SELECT entity_id, type, name, description, url_path,
				       updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
				FROM search_index
				%s
				LIMIT 1000
			) candidates
			ORDER BY updated_at DESC
			LIMIT $%d OFFSET $%d
		`, whereSQL, limitParam, offsetParam)
		return sqlQuery, params
	}

	// Simple filters (types, asset_types, providers from API params) or no filters
	// These can use existing indexes effectively
	sqlQuery := fmt.Sprintf(`
		SELECT
			type, entity_id, name, description, url_path,
			0.0::real as rank,
			updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
		FROM search_index
		%s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, limitParam, offsetParam)

	return sqlQuery, params
}

// canUseIndexedSort checks if the filter combination can use a composite index
// that covers both the filter predicate AND the sort order (updated_at DESC).
// This avoids expensive in-memory sorts for high-cardinality matches.
func (r *PostgresRepository) canUseIndexedSort(parsedQuery *query.Query, filter Filter) bool {
	// Check if query can use composite index (single @type or @provider exact match)
	if parsedQuery != nil && parsedQuery.CanUseCompositeIndex() && len(filter.Tags) == 0 {
		return true
	}

	// API-level AssetTypes filter (single value) can use composite index
	if parsedQuery != nil && !parsedQuery.HasStructuredFilters() && len(filter.AssetTypes) == 1 && len(filter.Tags) == 0 {
		return true
	}

	// Unfiltered queries can use idx_search_index_updated_at_browse
	// Note: selecting all 4 entity types is functionally equivalent to no type filter
	allTypesSelected := len(filter.Types) == 4
	noTypeFilter := len(filter.Types) == 0 || allTypesSelected
	if (parsedQuery == nil || !parsedQuery.HasStructuredFilters()) &&
		noTypeFilter && len(filter.AssetTypes) == 0 &&
		len(filter.Providers) == 0 && len(filter.Tags) == 0 {
		return true
	}

	return false
}

// buildPrefixSearchQuery handles short queries with exact/prefix matching.
func (r *PostgresRepository) buildPrefixSearchQuery(searchQuery string, filter Filter, parsedQuery *query.Query) (string, []interface{}) {
	var params []interface{}
	paramCount := 0

	// Add search query parameter
	paramCount++
	queryParam := paramCount
	params = append(params, strings.ToLower(searchQuery))

	whereClauses, params, paramCount := r.buildFilterClauses(filter, parsedQuery, params, paramCount)

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "AND " + strings.Join(whereClauses, " AND ")
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount
	params = append(params, filter.Limit, filter.Offset)

	// Rank: exact match = 1000, prefix match = 500
	sqlQuery := fmt.Sprintf(`
		SELECT
			type, entity_id, name, description, url_path,
			CASE
				WHEN lower(name) = $%d THEN 1000.0
				ELSE 500.0
			END::real as rank,
			updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
		FROM search_index
		WHERE (lower(name) = $%d OR lower(name) LIKE $%d || '%%')
		%s
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, queryParam, queryParam, queryParam, whereSQL, limitParam, offsetParam)

	return sqlQuery, params
}

// buildFuzzySearchQuery handles single-word queries with trigram similarity.
func (r *PostgresRepository) buildFuzzySearchQuery(searchQuery string, filter Filter, parsedQuery *query.Query) (string, []interface{}) {
	var params []interface{}
	paramCount := 0

	// Add search query parameter
	paramCount++
	queryParam := paramCount
	params = append(params, searchQuery)

	whereClauses, params, paramCount := r.buildFilterClauses(filter, parsedQuery, params, paramCount)

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "AND " + strings.Join(whereClauses, " AND ")
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount
	params = append(params, filter.Limit, filter.Offset)

	sqlQuery := fmt.Sprintf(`
		SELECT type, entity_id, name, description, url_path,
		       (similarity($%d, name) * 100.0)::real as rank,
		       updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
		FROM search_index
		WHERE name %% $%d
		%s
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, queryParam, queryParam, whereSQL, limitParam, offsetParam)

	return sqlQuery, params
}

// buildFullTextSearchQuery handles multi-word and complex queries.
func (r *PostgresRepository) buildFullTextSearchQuery(searchQuery string, filter Filter, parsedQuery *query.Query) (string, []interface{}) {
	var params []interface{}
	paramCount := 0

	// Add search query parameter
	paramCount++
	queryParam := paramCount
	params = append(params, searchQuery)

	whereClauses, params, paramCount := r.buildFilterClauses(filter, parsedQuery, params, paramCount)

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "AND " + strings.Join(whereClauses, " AND ")
	}

	paramCount++
	limitParam := paramCount
	paramCount++
	offsetParam := paramCount
	params = append(params, filter.Limit, filter.Offset)

	// Full-text search with candidate limiting to avoid expensive scans on large result sets
	// The CTE grabs first 1000 index matches (fast), then we rank within that set
	// This trades "most recent" for speed - acceptable for high-cardinality matches
	sqlQuery := fmt.Sprintf(`
		WITH candidates AS (
			SELECT entity_id, type, name, description, url_path, search_text,
			       updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
			FROM search_index
			WHERE search_text @@ websearch_to_tsquery('english', $%d)
			%s
			LIMIT 1000
		)
		SELECT
			type, entity_id, name, description, url_path,
			ts_rank_cd(search_text, websearch_to_tsquery('english', $%d), 32)::real as rank,
			updated_at, asset_type, primary_provider, providers, tags, mrn, created_by, created_at
		FROM candidates
		ORDER BY rank DESC, updated_at DESC
		LIMIT $%d OFFSET $%d
	`, queryParam, whereSQL, queryParam, limitParam, offsetParam)

	return sqlQuery, params
}

// buildFilterClauses constructs WHERE clause conditions for filters
func (r *PostgresRepository) buildFilterClauses(filter Filter, parsedQuery *query.Query, params []interface{}, paramCount int) ([]string, []interface{}, int) {
	var whereClauses []string

	// Type filter - skip if all 4 types are selected (functionally equivalent to no filter)
	if len(filter.Types) > 0 && len(filter.Types) < 4 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("type = ANY($%d)", paramCount))
		params = append(params, resultTypesToStrings(filter.Types))
	}

	// Asset-specific filters (applied only to assets, other types pass through)
	if len(filter.AssetTypes) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("(type != 'asset' OR asset_type = ANY($%d))", paramCount))
		params = append(params, filter.AssetTypes)
	}

	if len(filter.Providers) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("(type != 'asset' OR providers && $%d)", paramCount))
		params = append(params, filter.Providers)
	}

	if len(filter.Tags) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("tags && $%d", paramCount))
		params = append(params, filter.Tags)
	}

	// Add structured query conditions from the query package
	if parsedQuery != nil && parsedQuery.HasStructuredFilters() {
		builder := query.NewSearchIndexBuilder()
		structuredConditions, structuredParams, newParamCount, err := builder.BuildSearchConditions(parsedQuery, paramCount)
		if err == nil && len(structuredConditions) > 0 {
			whereClauses = append(whereClauses, structuredConditions...)
			params = append(params, structuredParams...)
			paramCount = newParamCount
		}
	}

	return whereClauses, params, paramCount
}

// scanSearchResults scans rows from the search_index table into Result structs.
func (r *PostgresRepository) scanSearchResults(rows pgx.Rows) ([]*Result, error) {
	var results []*Result

	for rows.Next() {
		var (
			resultType      string
			entityID        string
			name            string
			description     *string
			urlPath         string
			rank            float32
			updatedAt       *time.Time
			assetType       *string
			primaryProvider *string
			providers       []string
			tags            []string
			mrn             *string
			createdBy       *string
			createdAt       *time.Time
		)

		err := rows.Scan(
			&resultType,
			&entityID,
			&name,
			&description,
			&urlPath,
			&rank,
			&updatedAt,
			&assetType,
			&primaryProvider,
			&providers,
			&tags,
			&mrn,
			&createdBy,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		// Build metadata from indexed fields
		metadata := make(map[string]interface{})
		metadata["id"] = entityID
		metadata["name"] = name
		if description != nil {
			metadata["description"] = *description
		}
		if updatedAt != nil {
			metadata["updated_at"] = *updatedAt
		}
		if assetType != nil {
			metadata["type"] = *assetType
		}
		if primaryProvider != nil {
			metadata["primary_provider"] = *primaryProvider
		}
		if len(providers) > 0 {
			metadata["providers"] = providers
		}
		if len(tags) > 0 {
			metadata["tags"] = tags
		}
		if mrn != nil {
			metadata["mrn"] = *mrn
		}
		if createdBy != nil {
			metadata["created_by"] = *createdBy
		}
		if createdAt != nil {
			metadata["created_at"] = *createdAt
		}

		results = append(results, &Result{
			Type:        ResultType(resultType),
			ID:          entityID,
			Name:        name,
			Description: description,
			Metadata:    metadata,
			URL:         urlPath,
			Rank:        rank,
			UpdatedAt:   updatedAt,
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}

// buildFacets computes facet counts.
// For search queries with text, we skip facet computation entirely (too expensive).
// Facets are only computed for listing queries (no search text).
func (r *PostgresRepository) buildFacetsParallel(ctx context.Context, searchQuery string, filter Filter, parsedQuery *query.Query) (*Facets, int, error) {
	facets := &Facets{
		Types:      make(map[ResultType]int),
		AssetTypes: []FacetValue{},
		Providers:  []FacetValue{},
		Tags:       []FacetValue{},
	}

	// For search queries, skip expensive facet computation
	// The search results themselves provide the filtering - facets aren't needed
	if searchQuery != "" || (parsedQuery != nil && parsedQuery.HasStructuredFilters()) {
		// Just get a quick count from the search results
		// We'll estimate total from the search query instead
		return facets, 0, nil
	}

	// For unfiltered empty queries, use cached facets from summary_counts
	// This avoids expensive GROUP BY and UNNEST queries on the full table
	// Note: selecting all 4 entity types is functionally equivalent to no type filter
	allTypesSelected := len(filter.Types) == 4
	noTypeFilter := len(filter.Types) == 0 || allTypesSelected
	if noTypeFilter && len(filter.AssetTypes) == 0 && len(filter.Providers) == 0 && len(filter.Tags) == 0 {
		return r.buildCachedFacets(ctx, filter)
	}

	// For listing queries with filters, compute facets (filtered queries are fast)
	baseWhere, baseParams := r.buildListingFacetWhereClause(filter)

	// Type and asset_type facets (single query, no unnest)
	typeQuery := fmt.Sprintf(`
		SELECT type, asset_type, COUNT(*) as cnt
		FROM search_index
		%s
		GROUP BY type, asset_type
	`, baseWhere)

	rows, err := r.db.Query(ctx, typeQuery, baseParams...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying type facets: %w", err)
	}
	defer rows.Close()

	total := 0
	assetTypeCounts := make(map[string]int)

	for rows.Next() {
		var t string
		var assetType *string
		var count int
		if err := rows.Scan(&t, &assetType, &count); err != nil {
			return nil, 0, fmt.Errorf("scanning type facet: %w", err)
		}
		facets.Types[ResultType(t)] += count
		total += count
		if assetType != nil && t == "asset" {
			assetTypeCounts[*assetType] += count
		}
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	// Convert asset type counts to sorted slice
	for at, count := range assetTypeCounts {
		facets.AssetTypes = append(facets.AssetTypes, FacetValue{Value: at, Count: count})
	}
	sortFacetValues(facets.AssetTypes)
	if len(facets.AssetTypes) > maxFacetResults {
		facets.AssetTypes = facets.AssetTypes[:maxFacetResults]
	}

	// Provider and tag facets (with unnest, but no search filter so faster)
	if err := r.computeArrayFacets(ctx, baseWhere, baseParams, facets); err != nil {
		// Non-fatal: return partial facets
		return facets, total, nil
	}

	return facets, total, nil
}

// sortFacetValues sorts facet values by count descending
func sortFacetValues(fv []FacetValue) {
	for i := 0; i < len(fv); i++ {
		for j := i + 1; j < len(fv); j++ {
			if fv[j].Count > fv[i].Count {
				fv[i], fv[j] = fv[j], fv[i]
			}
		}
	}
}

// buildListingFacetWhereClause builds WHERE clause for listing facets (no search text)
func (r *PostgresRepository) buildListingFacetWhereClause(filter Filter) (string, []interface{}) {
	var params []interface{}
	paramCount := 0
	var whereClauses []string

	if len(filter.Types) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("type = ANY($%d)", paramCount))
		params = append(params, resultTypesToStrings(filter.Types))
	}

	if len(filter.AssetTypes) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("(type != 'asset' OR asset_type = ANY($%d))", paramCount))
		params = append(params, filter.AssetTypes)
	}

	if len(filter.Providers) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("(type != 'asset' OR providers && $%d)", paramCount))
		params = append(params, filter.Providers)
	}

	if len(filter.Tags) > 0 {
		paramCount++
		whereClauses = append(whereClauses, fmt.Sprintf("tags && $%d", paramCount))
		params = append(params, filter.Tags)
	}

	whereSQL := "WHERE true"
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	return whereSQL, params
}

// computeArrayFacets computes provider and tag facets
func (r *PostgresRepository) computeArrayFacets(ctx context.Context, baseWhere string, baseParams []interface{}, facets *Facets) error {
	// Provider facets
	providerQuery := fmt.Sprintf(`
		SELECT p, COUNT(*) as cnt
		FROM (
			SELECT unnest(providers) as p
			FROM search_index
			%s
			AND type = 'asset' AND providers IS NOT NULL
		) sub
		GROUP BY p
		ORDER BY cnt DESC
		LIMIT %d
	`, baseWhere, maxFacetResults)

	rows, err := r.db.Query(ctx, providerQuery, baseParams...)
	if err != nil {
		return fmt.Errorf("querying provider facets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var val string
		var count int
		if err := rows.Scan(&val, &count); err != nil {
			return fmt.Errorf("scanning provider facet: %w", err)
		}
		facets.Providers = append(facets.Providers, FacetValue{Value: val, Count: count})
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	// Tag facets
	tagQuery := fmt.Sprintf(`
		SELECT t, COUNT(*) as cnt
		FROM (
			SELECT unnest(tags) as t
			FROM search_index
			%s
			AND tags IS NOT NULL AND array_length(tags, 1) > 0
		) sub
		GROUP BY t
		ORDER BY cnt DESC
		LIMIT %d
	`, baseWhere, maxFacetResults)

	rows2, err := r.db.Query(ctx, tagQuery, baseParams...)
	if err != nil {
		return fmt.Errorf("querying tag facets: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var val string
		var count int
		if err := rows2.Scan(&val, &count); err != nil {
			return fmt.Errorf("scanning tag facet: %w", err)
		}
		facets.Tags = append(facets.Tags, FacetValue{Value: val, Count: count})
	}

	return rows2.Err()
}

// GetMetadata fetches full metadata for a set of results by type and IDs.
// This is used for lazy loading detailed information after initial search.
func (r *PostgresRepository) GetMetadata(ctx context.Context, resultType ResultType, ids []string) (map[string]map[string]interface{}, error) {
	if len(ids) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	var query string
	switch resultType {
	case ResultTypeAsset:
		query = fmt.Sprintf(`
			SELECT id, %s
			FROM assets
			WHERE id = ANY($1)
		`, assetMetadataColumns)
	case ResultTypeGlossary:
		query = fmt.Sprintf(`
			SELECT id::text, %s
			FROM glossary_terms
			WHERE id = ANY($1::uuid[])
		`, glossaryMetadataColumns)
	case ResultTypeTeam:
		query = fmt.Sprintf(`
			SELECT id::text, %s
			FROM teams
			WHERE id = ANY($1::uuid[])
		`, teamMetadataColumns)
	case ResultTypeDataProduct:
		query = fmt.Sprintf(`
			SELECT dp.id::text, %s
			FROM data_products dp
			LEFT JOIN product_images pi ON dp.id = pi.data_product_id AND pi.purpose = 'icon'
			WHERE dp.id = ANY($1::uuid[])
		`, dataProductMetadataColumns)
	default:
		return nil, fmt.Errorf("unknown result type: %s", resultType)
	}

	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("querying %s metadata: %w", resultType, err)
	}
	defer rows.Close()

	result := make(map[string]map[string]interface{})
	for rows.Next() {
		var id string
		var metadataBytes []byte
		if err := rows.Scan(&id, &metadataBytes); err != nil {
			return nil, fmt.Errorf("scanning %s metadata row: %w", resultType, err)
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling %s metadata for id %s: %w", resultType, id, err)
		}
		result[id] = metadata
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating %s metadata rows: %w", resultType, rows.Err())
	}

	return result, nil
}

// resultTypesToStrings converts ResultType slice to string slice for SQL.
func resultTypesToStrings(types []ResultType) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = string(t)
	}
	return result
}

// emptyFacets returns an initialized empty Facets struct.
func emptyFacets() *Facets {
	return &Facets{
		Types:      make(map[ResultType]int),
		AssetTypes: []FacetValue{},
		Providers:  []FacetValue{},
		Tags:       []FacetValue{},
	}
}

// buildCachedFacets reads pre-computed facet counts from summary_counts table.
// This is used for empty/browse queries where computing facets on-the-fly is expensive.
// The summary_counts table is maintained by triggers on the source tables.
func (r *PostgresRepository) buildCachedFacets(ctx context.Context, filter Filter) (*Facets, int, error) {
	facets := &Facets{
		Types:      make(map[ResultType]int),
		AssetTypes: []FacetValue{},
		Providers:  []FacetValue{},
		Tags:       []FacetValue{},
	}

	rows, err := r.db.Query(ctx, `
		SELECT dimension, key, count
		FROM summary_counts
		WHERE count > 0
		ORDER BY
			CASE dimension
				WHEN 'entity_type' THEN 1
				WHEN 'type' THEN 2
				WHEN 'provider' THEN 3
				WHEN 'tag' THEN 4
			END,
			count DESC, key ASC
	`)
	if err != nil {
		return nil, 0, fmt.Errorf("querying cached facets: %w", err)
	}
	defer rows.Close()

	total := 0
	tagCount := 0

	for rows.Next() {
		var dimension, key string
		var count int
		if err := rows.Scan(&dimension, &key, &count); err != nil {
			return nil, 0, fmt.Errorf("scanning cached facet: %w", err)
		}

		switch dimension {
		case "entity_type":
			// Entity types for the Types facet (asset, glossary, team, data_product)
			facets.Types[ResultType(key)] = count
			total += count
		case "type":
			// Asset types for the AssetTypes facet (table, dashboard, etc.)
			facets.AssetTypes = append(facets.AssetTypes, FacetValue{Value: key, Count: count})
		case "provider":
			// Providers for the Providers facet
			facets.Providers = append(facets.Providers, FacetValue{Value: key, Count: count})
		case "tag":
			// Tags for the Tags facet (limit to top 50)
			if tagCount < maxFacetResults {
				facets.Tags = append(facets.Tags, FacetValue{Value: key, Count: count})
				tagCount++
			}
		}
	}

	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	// Limit facet slices to maxFacetResults
	if len(facets.AssetTypes) > maxFacetResults {
		facets.AssetTypes = facets.AssetTypes[:maxFacetResults]
	}
	if len(facets.Providers) > maxFacetResults {
		facets.Providers = facets.Providers[:maxFacetResults]
	}

	return facets, total, nil
}
