package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/query"
)

var (
	ErrNotFound     = errors.New("asset not found")
	ErrConflict     = errors.New("asset already exists")
	ErrInvalidQuery = errors.New("invalid search query")
)

// Common SQL fragments
const (
	baseSelectAsset = `
		SELECT 
			id, name, mrn, type, providers, environments, external_links,
			description, metadata, schema, sources, tags,
			created_at, created_by, updated_at, last_sync_at
		FROM assets`
)

type Repository interface {
	Create(ctx context.Context, asset *Asset) error
	Get(ctx context.Context, id string) (*Asset, error)
	GetByMRN(ctx context.Context, qualifiedName string) (*Asset, error)
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter, calculateCounts bool) ([]*Asset, int, AvailableFilters, error)
	Summary(ctx context.Context) (*AssetSummary, error)
	Update(ctx context.Context, asset *Asset) error
	Delete(ctx context.Context, id string) error
	ListByPattern(ctx context.Context, pattern string, assetType string) ([]*Asset, error)
	GetByMRNs(ctx context.Context, mrns []string) ([]*Asset, error)
	GetByTypeAndName(ctx context.Context, assetType, name string) (*Asset, error)
	GetMetadataFieldsWithContext(ctx context.Context, queryContext *MetadataContext) ([]MetadataFieldSuggestion, error)
	GetMetadataValuesWithContext(ctx context.Context, field string, prefix string, limit int, queryContext *MetadataContext) ([]MetadataValueSuggestion, error)
	GetMetadataFields(ctx context.Context) ([]MetadataFieldSuggestion, error)
	GetMetadataValues(ctx context.Context, field string, prefix string, limit int) ([]MetadataValueSuggestion, error)
	GetTagSuggestions(ctx context.Context, prefix string, limit int) ([]string, error)
}

type ListResult struct {
	Assets  []*Asset
	Total   int
	Filters AvailableFilters
}

type AvailableFilters struct {
	Types     map[string]int `json:"types"`
	Providers map[string]int `json:"providers"`
	Tags      map[string]int `json:"tags"`
}

type AssetTypeSummary struct {
	Count   int    `json:"count"`
	Service string `json:"service"`
}

type AssetSummary struct {
	Types     map[string]AssetTypeSummary `json:"types"`
	Providers map[string]int              `json:"providers"`
	Tags      map[string]int              `json:"tags"`
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

// TODO: this is ugly and smelly but I dont have any better ideas
// Helper method for JSON marshaling of common fields
func marshalAssetFields(asset *Asset) ([]byte, []byte, []byte, []byte, error) {
	metadataJSON, err := json.Marshal(asset.Metadata)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshaling metadata: %w", err)
	}

	sourcesJSON, err := json.Marshal(asset.Sources)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshaling sources: %w", err)
	}

	environmentsJSON, err := json.Marshal(asset.Environments)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshaling environments: %w", err)
	}

	externalLinksJSON, err := json.Marshal(asset.ExternalLinks)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshaling external links: %w", err)
	}

	return metadataJSON, sourcesJSON, environmentsJSON, externalLinksJSON, nil
}

func (r *PostgresRepository) Create(ctx context.Context, asset *Asset) error {
	metadataJSON, sourcesJSON, environmentsJSON, externalLinksJSON, err := marshalAssetFields(asset)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO assets (
			id, name, mrn, type, providers, environments, description,
			metadata, schema, sources, tags, external_links,
			created_by, created_at, updated_at, last_sync_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err = r.db.Exec(ctx, query,
		asset.ID, asset.Name, asset.MRN, asset.Type, asset.Providers,
		environmentsJSON, asset.Description, metadataJSON, asset.Schema,
		sourcesJSON, asset.Tags, externalLinksJSON,
		asset.CreatedBy, asset.CreatedAt, asset.UpdatedAt, asset.LastSyncAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("inserting asset: %w", err)
	}

	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*Asset, error) {
	return r.scanSingleAsset(ctx, baseSelectAsset+" WHERE id = $1", id)
}

func (r *PostgresRepository) GetByMRN(ctx context.Context, qualifiedName string) (*Asset, error) {
	return r.scanSingleAsset(ctx, baseSelectAsset+" WHERE mrn = $1", qualifiedName)
}

func (r *PostgresRepository) GetByTypeAndName(ctx context.Context, assetType, name string) (*Asset, error) {
	return r.scanSingleAsset(ctx,
		baseSelectAsset+" WHERE LOWER(type) = LOWER($1) AND LOWER(name) = LOWER($2)",
		assetType, name)
}

func (r *PostgresRepository) GetByMRNs(ctx context.Context, mrns []string) ([]*Asset, error) {
	return r.scanMultipleAssets(ctx, baseSelectAsset+" WHERE mrn = ANY($1)", mrns)
}

func (r *PostgresRepository) ListByPattern(ctx context.Context, pattern string, assetType string) ([]*Asset, error) {
	assets, err := r.scanMultipleAssets(ctx,
		baseSelectAsset+` WHERE type = $1 AND name ~ $2`,
		assetType, fmt.Sprintf("^%s$", pattern))

	if err != nil {
		return nil, fmt.Errorf("scanning assets: %w", err)
	}

	if len(assets) > 1 {
		return nil, fmt.Errorf("found %d matches for pattern %s, expected 1", len(assets), pattern)
	}

	return assets, nil
}

func (r *PostgresRepository) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var total int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM assets").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("counting assets: %w", err)
	}

	assets, err := r.scanMultipleAssets(ctx,
		baseSelectAsset+" ORDER BY created_at DESC LIMIT $1 OFFSET $2",
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("scanning assets: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &ListResult{
		Assets: assets,
		Total:  total,
	}, nil
}

func (r *PostgresRepository) Update(ctx context.Context, asset *Asset) error {
	metadataJSON, sourcesJSON, environmentsJSON, externalLinksJSON, err := marshalAssetFields(asset)
	if err != nil {
		return err
	}

	query := `
		UPDATE assets 
		SET name = $1, description = $2, metadata = $3, schema = $4,
			tags = $5, updated_at = $6, sources = $7, environments = $8,
			external_links = $9, providers = $10, mrn = $11,
			type = $12
		WHERE id = $13`

	commandTag, err := r.db.Exec(ctx, query,
		asset.Name, asset.Description, metadataJSON, asset.Schema,
		asset.Tags, asset.UpdatedAt, sourcesJSON, environmentsJSON,
		externalLinksJSON, asset.Providers, asset.MRN,
		asset.Type, asset.ID)

	if err != nil {
		return fmt.Errorf("updating asset: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	commandTag, err := r.db.Exec(ctx, "DELETE FROM assets WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting asset: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) scanAsset(row pgx.Row) (*Asset, error) {
	var asset Asset
	var metadataJSON, sourcesJSON, environmentsJSON, externalLinksJSON, schemaJSON []byte

	err := row.Scan(
		&asset.ID, &asset.Name, &asset.MRN, &asset.Type, &asset.Providers,
		&environmentsJSON, &externalLinksJSON, &asset.Description,
		&metadataJSON, &schemaJSON, &sourcesJSON,
		&asset.Tags, &asset.CreatedAt, &asset.CreatedBy, &asset.UpdatedAt,
		&asset.LastSyncAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning asset: %w", err)
	}

	if asset.Metadata == nil {
		asset.Metadata = make(map[string]interface{})
	}
	if asset.Environments == nil {
		asset.Environments = make(map[string]Environment)
	}
	if asset.Sources == nil {
		asset.Sources = make([]AssetSource, 0)
	}
	if asset.ExternalLinks == nil {
		asset.ExternalLinks = make([]ExternalLink, 0)
	}
	if asset.Schema == nil {
		asset.Schema = make(map[string]string)
	}
	if asset.Tags == nil {
		asset.Tags = make([]string, 0)
	}
	if asset.Providers == nil {
		asset.Providers = make([]string, 0)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &asset.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	if len(schemaJSON) > 0 {
		if err := json.Unmarshal(schemaJSON, &asset.Schema); err != nil {
			return nil, fmt.Errorf("unmarshaling schema: %w", err)
		}
	}

	if len(sourcesJSON) > 0 {
		if err := json.Unmarshal(sourcesJSON, &asset.Sources); err != nil {
			return nil, fmt.Errorf("unmarshaling sources: %w", err)
		}
	}
	if len(environmentsJSON) > 0 {
		if err := json.Unmarshal(environmentsJSON, &asset.Environments); err != nil {
			return nil, fmt.Errorf("unmarshaling environments: %w", err)
		}
	}
	if len(externalLinksJSON) > 0 {
		if err := json.Unmarshal(externalLinksJSON, &asset.ExternalLinks); err != nil {
			return nil, fmt.Errorf("unmarshaling external links: %w", err)
		}
	}

	return &asset, nil
}

func (r *PostgresRepository) scanSingleAsset(ctx context.Context, query string, args ...interface{}) (*Asset, error) {
	return r.scanAsset(r.db.QueryRow(ctx, query, args...))
}

func (r *PostgresRepository) scanMultipleAssets(ctx context.Context, query string, args ...interface{}) ([]*Asset, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying assets: %w", err)
	}
	defer rows.Close()

	var assets []*Asset
	for rows.Next() {
		asset, err := r.scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating asset rows: %w", rows.Err())
	}

	return assets, nil
}

func (r *PostgresRepository) GetMetadataFields(ctx context.Context) ([]MetadataFieldSuggestion, error) {
	query := `
		WITH RECURSIVE all_metadata_keys AS (
			SELECT 
				key as path,
				key as field,
				value,
				jsonb_typeof(value) as type,
				1 as depth,
				ARRAY[key] as path_parts,
				ARRAY[jsonb_typeof(value)] as types
			FROM assets,
				jsonb_each(metadata)
			WHERE metadata != '{}'::jsonb
			
			UNION ALL
			
			SELECT 
				mk.path || '.' || e.key,
				e.key,
				e.value,
				jsonb_typeof(e.value),
				mk.depth + 1,
				mk.path_parts || e.key,
				mk.types || jsonb_typeof(e.value)
			FROM all_metadata_keys mk,
				jsonb_each(mk.value) e
			WHERE mk.type = 'object'
		)
		SELECT DISTINCT ON (path)
			path as field,
			type,
			count(*) as count,
			CASE WHEN type != 'object' THEN MIN(value::text) ELSE NULL END as example,
			array_agg(DISTINCT path_parts[1]) as path_parts,
			array_agg(DISTINCT types[1]) as types
		FROM all_metadata_keys
		GROUP BY path, type
		ORDER BY path, count DESC;`

	return r.scanMetadataFields(ctx, query)
}

func (r *PostgresRepository) GetMetadataFieldsWithContext(ctx context.Context, queryContext *MetadataContext) ([]MetadataFieldSuggestion, error) {
	query := `
		WITH matching_assets AS (
			SELECT id FROM assets
			WHERE search_text @@ websearch_to_tsquery('english', $1)
		),
		metadata_stats AS (
			SELECT 
				key as field,
				jsonb_typeof(value) as type,
				COUNT(*) as count,
				MODE() WITHIN GROUP (ORDER BY value) as example
			FROM assets a
			JOIN matching_assets ma ON a.id = ma.id,
				jsonb_each(metadata)
			WHERE metadata != '{}'::jsonb
			GROUP BY key, jsonb_typeof(value)
		)
		SELECT 
			field,
			type,
			count,
			example,
			ARRAY[field] as path_parts,
			ARRAY[type] as types
		FROM metadata_stats
		ORDER BY count DESC, field ASC`

	return r.scanMetadataFields(ctx, query, queryContext.Query)
}

func (r *PostgresRepository) scanMetadataFields(ctx context.Context, query string, args ...interface{}) ([]MetadataFieldSuggestion, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying metadata fields: %w", err)
	}
	defer rows.Close()

	var suggestions []MetadataFieldSuggestion
	for rows.Next() {
		var s MetadataFieldSuggestion
		//var exampleJSON string // Remove this line
		if err := rows.Scan(&s.Field, &s.Type, &s.Count, &s.Example, &s.PathParts, &s.Types); err != nil {
			return nil, fmt.Errorf("scanning metadata field: %w", err)
		}

		// Remove the Unmarshal part, as Example is already a pgtype.Text
		suggestions = append(suggestions, s)
	}

	return suggestions, nil
}

func (r *PostgresRepository) GetMetadataValues(ctx context.Context, field string, prefix string, limit int) ([]MetadataValueSuggestion, error) {
	pathArray := strings.Split(field, ".")
	query := `
		WITH RECURSIVE MetadataValues AS (
			SELECT
				a.id,
				jsonb_extract_path(a.metadata, VARIADIC $1::text[]) as value,
				1 as level
			FROM assets a
			WHERE jsonb_typeof(jsonb_extract_path(a.metadata, VARIADIC $1::text[])) != 'null'
		)
		SELECT
			value::text,
			COUNT(DISTINCT id)
		FROM MetadataValues
		WHERE (
			$2 = '' OR
			CASE
				WHEN jsonb_typeof(value) = 'string' THEN value::text ILIKE $2 || '%'
				WHEN jsonb_typeof(value) IN ('number', 'boolean') THEN value::text ILIKE $2 || '%'
				ELSE FALSE
			END
		)
		AND jsonb_typeof(value) != 'null'
		GROUP BY value
		ORDER BY COUNT(DISTINCT id) DESC, value ASC
		LIMIT $3`

	return r.scanMetadataValues(ctx, query, pathArray, prefix, limit)
}

func (r *PostgresRepository) GetMetadataValuesWithContext(ctx context.Context, field string, prefix string, limit int, queryContext *MetadataContext) ([]MetadataValueSuggestion, error) {
	query := `
		WITH matching_assets AS (
			SELECT id FROM assets
			WHERE search_text @@ websearch_to_tsquery('english', $1)
		),
		MetadataValues AS (
			SELECT
				a.id,
				je.key,
				je.value
			FROM assets a
			JOIN matching_assets ma ON a.id = ma.id
			CROSS JOIN LATERAL jsonb_each(a.metadata) AS je
			WHERE a.metadata IS NOT NULL
		)
		SELECT
			value::text,
			COUNT(DISTINCT id)
		FROM MetadataValues
		WHERE key = $2
		AND (
			$3 = '' OR
			CASE
				WHEN jsonb_typeof(value) = 'string' THEN value::text ILIKE $3 || '%'
				WHEN jsonb_typeof(value) IN ('number', 'boolean') THEN value::text ILIKE $3 || '%'
				ELSE FALSE
			END
		)
		GROUP BY value
		ORDER BY COUNT(DISTINCT id) DESC, value ASC
		LIMIT $4`

	return r.scanMetadataValues(ctx, query, queryContext.Query, field, prefix, limit)
}

func (r *PostgresRepository) scanMetadataValues(ctx context.Context, query string, args ...interface{}) ([]MetadataValueSuggestion, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying metadata values: %w", err)
	}
	defer rows.Close()

	var suggestions []MetadataValueSuggestion
	for rows.Next() {
		var s MetadataValueSuggestion
		if err := rows.Scan(&s.Value, &s.Count); err != nil {
			return nil, fmt.Errorf("scanning metadata value: %w", err)
		}
		s.Value = strings.Trim(s.Value, "\"")
		suggestions = append(suggestions, s)
	}

	return suggestions, nil
}

func (r *PostgresRepository) Summary(ctx context.Context) (*AssetSummary, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	summary := &AssetSummary{
		Types:     make(map[string]AssetTypeSummary),
		Providers: make(map[string]int),
		Tags:      make(map[string]int),
	}

	// Get types summary with their corresponding providers
	typeRows, err := tx.Query(ctx, `
		WITH TypeCounts AS (
			SELECT 
				type,
				COUNT(*) as count,
				array_agg(DISTINCT s.service) as providers
			FROM assets
			CROSS JOIN LATERAL unnest(providers) as s(service)
			GROUP BY type
		)
		SELECT type, count, providers[1] as primary_service
		FROM TypeCounts`)
	if err != nil {
		return nil, fmt.Errorf("querying types summary: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var t string
		var count int
		var service string
		if err := typeRows.Scan(&t, &count, &service); err != nil {
			return nil, fmt.Errorf("scanning type summary: %w", err)
		}
		summary.Types[t] = AssetTypeSummary{Count: count, Service: service}
	}

	// Get providers summary
	serviceRows, err := tx.Query(ctx, `
		SELECT s.service, COUNT(*) as count
		FROM assets
		CROSS JOIN LATERAL unnest(providers) as s(service)
		GROUP BY s.service`)
	if err != nil {
		return nil, fmt.Errorf("querying providers summary: %w", err)
	}
	defer serviceRows.Close()

	for serviceRows.Next() {
		var service string
		var count int
		if err := serviceRows.Scan(&service, &count); err != nil {
			return nil, fmt.Errorf("scanning service summary: %w", err)
		}
		summary.Providers[service] = count
	}

	// Get tags summary
	tagRows, err := tx.Query(ctx, `
		SELECT tag, COUNT(*) as count
		FROM (
			SELECT DISTINCT id, unnest(tags) as tag
			FROM assets
			WHERE tags IS NOT NULL AND array_length(tags, 1) > 0
		) t
		GROUP BY tag`)
	if err != nil {
		return nil, fmt.Errorf("querying tags summary: %w", err)
	}
	defer tagRows.Close()

	for tagRows.Next() {
		var tag string
		var count int
		if err := tagRows.Scan(&tag, &count); err != nil {
			return nil, fmt.Errorf("scanning tag summary: %w", err)
		}
		summary.Tags[tag] = count
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return summary, nil
}

func (r *PostgresRepository) GetTagSuggestions(ctx context.Context, prefix string, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		WITH tag_counts AS (
			SELECT DISTINCT unnest(tags) as tag, COUNT(*) as count
			FROM assets
			WHERE tags IS NOT NULL 
				AND array_length(tags, 1) > 0
				AND ($1 = '' OR unnest(tags) ILIKE $1 || '%
		GROUP BY unnest(tags)
			ORDER BY count DESC, tag ASC
			LIMIT $2
		)
		SELECT tag FROM tag_counts ORDER BY tag ASC`,
		prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("querying tag suggestions: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// This makes me want to cry, but it works and I'm too scared to touch it.
// Eventually this should be broken into smaller, more readable functions and DB transactions
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter, calculateCounts bool) ([]*Asset, int, AvailableFilters, error) {
	parser := query.NewParser()
	builder := query.NewBuilder()

	searchQuery, err := parser.Parse(filter.Query)
	if err != nil {
		return nil, 0, AvailableFilters{}, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}

	baseQuery := `SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets`
	query, params, err := builder.BuildSQL(searchQuery, baseQuery)
	if err != nil {
		return nil, 0, AvailableFilters{}, fmt.Errorf("building query: %w", err)
	}

	query = strings.TrimPrefix(query, "WITH search_results AS (")
	query = strings.TrimSuffix(query, ") SELECT * FROM search_results ORDER BY search_rank DESC")

	if len(filter.Types) > 0 {
		if strings.Contains(query, "WHERE") {
			query += fmt.Sprintf(" AND type = ANY($%d)", len(params)+1)
		} else {
			query += fmt.Sprintf(" WHERE type = ANY($%d)", len(params)+1)
		}
		params = append(params, filter.Types)
	}

	if len(filter.Providers) > 0 {
		if strings.Contains(query, "WHERE") {
			query += fmt.Sprintf(" AND providers && $%d", len(params)+1)
		} else {
			query += fmt.Sprintf(" WHERE providers && $%d", len(params)+1)
		}
		params = append(params, filter.Providers)
	}

	if len(filter.Tags) > 0 {
		if strings.Contains(query, "WHERE") {
			query += fmt.Sprintf(" AND tags @> $%d", len(params)+1)
		} else {
			query += fmt.Sprintf(" WHERE tags @> $%d", len(params)+1)
		}
		params = append(params, filter.Tags)
	}

	wrappedQuery := fmt.Sprintf("WITH search_results AS (%s)", query)

	var total int
	err = r.db.QueryRow(ctx, wrappedQuery+" SELECT COUNT(*) FROM search_results", params...).Scan(&total)
	if err != nil {
		return nil, 0, AvailableFilters{}, fmt.Errorf("counting results: %w", err)
	}

	wrappedQuery += `
       SELECT 
           id, name, mrn, type, providers, environments, external_links,
           description, metadata, schema, sources, tags,
           created_at, created_by, updated_at, last_sync_at
       FROM search_results
       ORDER BY 
           CASE WHEN name_similarity > 0.8 THEN name_similarity * 2
           ELSE search_rank END DESC
       LIMIT $%d OFFSET $%d
   `
	params = append(params, filter.Limit, filter.Offset)
	wrappedQuery = fmt.Sprintf(wrappedQuery, len(params)-1, len(params))

	assets, err := r.scanMultipleAssets(ctx, wrappedQuery, params...)
	if err != nil {
		return nil, 0, AvailableFilters{}, fmt.Errorf("executing search: %w", err)
	}

	availableFilters := AvailableFilters{
		Types:     make(map[string]int),
		Providers: make(map[string]int),
		Tags:      make(map[string]int),
	}

	if calculateCounts {
		countQuery := `
        WITH filtered_results AS (
            SELECT *
            FROM assets
            WHERE 1=1
        `
		countParams := []interface{}{}

		if filter.Query != "" && !strings.HasPrefix(filter.Query, "@metadata") {
			countQuery += " AND search_text @@ websearch_to_tsquery('english', $1)"
			countParams = append(countParams, filter.Query)
		} else if filter.Query != "" {
			searchQ, err := parser.Parse(filter.Query)
			if err == nil && searchQ.Bool != nil {
				conditions, qParams, _ := builder.BuildConditions(searchQ.Bool)
				if len(conditions) > 0 {
					countQuery += " AND " + strings.Join(conditions, " AND ")
					countParams = append(countParams, qParams...)
				}
			}
		}

		if len(filter.Types) > 0 {
			countQuery += fmt.Sprintf(" AND type = ANY($%d)", len(countParams)+1)
			countParams = append(countParams, filter.Types)
		}
		if len(filter.Providers) > 0 {
			countQuery += fmt.Sprintf(" AND providers && $%d", len(countParams)+1)
			countParams = append(countParams, filter.Providers)
		}
		if len(filter.Tags) > 0 {
			countQuery += fmt.Sprintf(" AND tags @> $%d", len(countParams)+1)
			countParams = append(countParams, filter.Tags)
		}

		countQuery += `
        )
        SELECT 
            (
                SELECT COALESCE(jsonb_object_agg(type, count), '{}'::jsonb)
                FROM (
                    SELECT type, COUNT(*) as count 
                    FROM filtered_results
                    GROUP BY type
                ) type_counts
            ) as types,
            (
                SELECT COALESCE(jsonb_object_agg(service, count), '{}'::jsonb)
                FROM (
                    SELECT service, COUNT(*) as count 
                    FROM filtered_results,
                    unnest(providers) as service
                    WHERE array_length(providers, 1) > 0
                    GROUP BY service
                ) service_counts
            ) as providers,
            (
                SELECT COALESCE(jsonb_object_agg(tag, count), '{}'::jsonb)
                FROM (
                    SELECT tag, COUNT(*) as count 
                    FROM filtered_results,
                    unnest(tags) as tag
                    WHERE array_length(tags, 1) > 0
                    GROUP BY tag
                ) tag_counts
            ) as tags
        `

		var types, providers, tags pgtype.JSONB
		err = r.db.QueryRow(ctx, countQuery, countParams...).Scan(&types, &providers, &tags)
		if err != nil {
			return nil, 0, AvailableFilters{}, fmt.Errorf("getting counts: %w", err)
		}

		if err := json.Unmarshal(types.Bytes, &availableFilters.Types); err != nil {
			return nil, 0, availableFilters, fmt.Errorf("unmarshaling type counts: %w", err)
		}
		if err := json.Unmarshal(providers.Bytes, &availableFilters.Providers); err != nil {
			return nil, 0, availableFilters, fmt.Errorf("unmarshaling service counts: %w", err)
		}
		if err := json.Unmarshal(tags.Bytes, &availableFilters.Tags); err != nil {
			return nil, 0, availableFilters, fmt.Errorf("unmarshaling tag counts: %w", err)
		}
	}

	return assets, total, availableFilters, nil
}
