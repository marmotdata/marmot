package dataproduct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/query"
)

var (
	ErrNotFound     = errors.New("data product not found")
	ErrConflict     = errors.New("data product with this name already exists")
	ErrInvalidInput = errors.New("invalid input")
	ErrRuleNotFound = errors.New("rule not found")
)

type RuleType string

const (
	RuleTypeQuery         RuleType = "query"
	RuleTypeMetadataMatch RuleType = "metadata_match"
)

const (
	PatternTypeExact    = "exact"
	PatternTypeWildcard = "wildcard"
	PatternTypeRegex    = "regex"
	PatternTypePrefix   = "prefix"
)

const (
	SourceManual = "manual"
	SourceRule   = "rule"
)

const (
	TargetTypeAssetType   = "asset_type"
	TargetTypeProvider    = "provider"
	TargetTypeTag         = "tag"
	TargetTypeMetadataKey = "metadata_key"
	TargetTypeQuery       = "query"
)

type DataProduct struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Owners      []Owner                `json:"owners"`
	Rules       []Rule                 `json:"rules,omitempty"`
	CreatedBy   *string                `json:"created_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`

	AssetCount       int `json:"asset_count,omitempty"`
	ManualAssetCount int `json:"manual_asset_count,omitempty"`
	RuleAssetCount   int `json:"rule_asset_count,omitempty"`

	IconURL *string `json:"icon_url,omitempty"`
}

type Owner struct {
	ID             string  `json:"id"`
	Username       *string `json:"username,omitempty"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Email          *string `json:"email,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
}

type OwnerInput struct {
	ID   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required,oneof=user team"`
}

type Rule struct {
	ID              string    `json:"id"`
	DataProductID   string    `json:"data_product_id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	RuleType        RuleType  `json:"rule_type"`
	QueryExpression *string   `json:"query_expression,omitempty"`
	MetadataField   *string   `json:"metadata_field,omitempty"`
	PatternType     *string   `json:"pattern_type,omitempty"`
	PatternValue    *string   `json:"pattern_value,omitempty"`
	Priority        int       `json:"priority"`
	IsEnabled       bool      `json:"is_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	MatchedAssetCount int `json:"matched_asset_count,omitempty"`
}

type RuleInput struct {
	ID              *string  `json:"id,omitempty"`
	Name            string   `json:"name" validate:"required,min=1,max=255"`
	Description     *string  `json:"description,omitempty"`
	RuleType        RuleType `json:"rule_type" validate:"required,oneof=query metadata_match"`
	QueryExpression *string  `json:"query_expression,omitempty"`
	MetadataField   *string  `json:"metadata_field,omitempty"`
	PatternType     *string  `json:"pattern_type,omitempty" validate:"omitempty,oneof=exact wildcard regex prefix"`
	PatternValue    *string  `json:"pattern_value,omitempty"`
	Priority        int      `json:"priority"`
	IsEnabled       bool     `json:"is_enabled"`
}

type SearchFilter struct {
	Query    string   `json:"query,omitempty"`
	OwnerIDs []string `json:"owner_ids,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Limit    int      `json:"limit,omitempty" validate:"omitempty,gte=0,lte=100"`
	Offset   int      `json:"offset,omitempty" validate:"omitempty,gte=0"`
}

type ListResult struct {
	DataProducts []*DataProduct `json:"data_products"`
	Total        int            `json:"total"`
}

type ResolvedAssets struct {
	ManualAssets  []string `json:"manual_assets"`
	DynamicAssets []string `json:"dynamic_assets"`
	AllAssets     []string `json:"all_assets"`
	Total         int      `json:"total"`
}

type RulePreview struct {
	AssetIDs   []string `json:"asset_ids"`
	AssetCount int      `json:"asset_count"`
	Errors     []string `json:"errors,omitempty"`
}

type AssetsResult struct {
	AssetIDs []string `json:"asset_ids"`
	Total    int      `json:"total"`
}

type Repository interface {
	Create(ctx context.Context, dp *DataProduct, owners []OwnerInput) error
	Get(ctx context.Context, id string) (*DataProduct, error)
	Update(ctx context.Context, dp *DataProduct, owners []OwnerInput) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter) (*ListResult, error)

	AddAssets(ctx context.Context, dataProductID string, assetIDs []string, createdBy string) error
	RemoveAsset(ctx context.Context, dataProductID string, assetID string) error
	GetManualAssets(ctx context.Context, dataProductID string, limit, offset int) (*AssetsResult, error)

	CreateRule(ctx context.Context, dataProductID string, rule *RuleInput) (*Rule, error)
	UpdateRule(ctx context.Context, ruleID string, rule *RuleInput) (*Rule, error)
	DeleteRule(ctx context.Context, ruleID string) error
	GetRules(ctx context.Context, dataProductID string) ([]Rule, error)
	GetRule(ctx context.Context, ruleID string) (*Rule, error)

	ResolveAssets(ctx context.Context, dataProductID string, limit, offset int) (*ResolvedAssets, error)
	ExecuteRule(ctx context.Context, rule *Rule) ([]string, error)
	PreviewRule(ctx context.Context, rule *RuleInput, limit int) (*RulePreview, error)

	GetDataProductsForAsset(ctx context.Context, assetID string) ([]*DataProduct, error)

	UploadProductImage(ctx context.Context, dataProductID string, purpose ImagePurpose, input UploadImageInput, createdBy *string) (*ProductImage, error)
	GetProductImage(ctx context.Context, imageID string) (*ProductImage, error)
	GetProductImageByPurpose(ctx context.Context, dataProductID string, purpose ImagePurpose) (*ProductImage, error)
	GetProductImageMeta(ctx context.Context, dataProductID string, purpose ImagePurpose) (*ProductImageMeta, error)
	DeleteProductImage(ctx context.Context, dataProductID string, purpose ImagePurpose) error
	ListProductImages(ctx context.Context, dataProductID string) ([]*ProductImageMeta, error)
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

func (r *PostgresRepository) loadOwners(ctx context.Context, dataProductID string) ([]Owner, error) {
	q := `
		SELECT
			COALESCE(u.id::text, t.id::text) as id,
			u.username,
			COALESCE(u.name, t.name) as name,
			CASE WHEN u.id IS NOT NULL THEN 'user' ELSE 'team' END as type,
			ui.provider_email,
			u.profile_picture
		FROM data_product_owners dpo
		LEFT JOIN users u ON dpo.user_id = u.id
		LEFT JOIN teams t ON dpo.team_id = t.id
		LEFT JOIN user_identities ui ON u.id = ui.user_id
		WHERE dpo.data_product_id = $1
		ORDER BY type, COALESCE(u.username, t.name)`

	rows, err := r.db.Query(ctx, q, dataProductID)
	if err != nil {
		return nil, fmt.Errorf("loading owners: %w", err)
	}
	defer rows.Close()

	owners := []Owner{}
	for rows.Next() {
		var owner Owner
		if err := rows.Scan(&owner.ID, &owner.Username, &owner.Name, &owner.Type, &owner.Email, &owner.ProfilePicture); err != nil {
			return nil, fmt.Errorf("scanning owner: %w", err)
		}
		owners = append(owners, owner)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating owners: %w", err)
	}

	return owners, nil
}

func (r *PostgresRepository) setOwners(ctx context.Context, tx pgx.Tx, dataProductID string, owners []OwnerInput) error {
	_, err := tx.Exec(ctx, "DELETE FROM data_product_owners WHERE data_product_id = $1", dataProductID)
	if err != nil {
		return fmt.Errorf("deleting existing owners: %w", err)
	}

	for _, owner := range owners {
		var q string
		if owner.Type == "user" {
			q = "INSERT INTO data_product_owners (data_product_id, user_id) VALUES ($1, $2)"
		} else {
			q = "INSERT INTO data_product_owners (data_product_id, team_id) VALUES ($1, $2)"
		}

		_, err := tx.Exec(ctx, q, dataProductID, owner.ID)
		if err != nil {
			return fmt.Errorf("inserting owner: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) loadRules(ctx context.Context, dataProductID string) ([]Rule, error) {
	q := `
		SELECT id, data_product_id, name, description, rule_type, query_expression,
			   metadata_field, pattern_type, pattern_value, priority, is_enabled,
			   created_at, updated_at
		FROM data_product_rules
		WHERE data_product_id = $1
		ORDER BY priority ASC, created_at ASC`

	rows, err := r.db.Query(ctx, q, dataProductID)
	if err != nil {
		return nil, fmt.Errorf("loading rules: %w", err)
	}
	defer rows.Close()

	rules := []Rule{}
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(
			&rule.ID, &rule.DataProductID, &rule.Name, &rule.Description,
			&rule.RuleType, &rule.QueryExpression, &rule.MetadataField,
			&rule.PatternType, &rule.PatternValue, &rule.Priority,
			&rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rules: %w", err)
	}

	return rules, nil
}

func (r *PostgresRepository) getAssetCounts(ctx context.Context, dataProductID string) (int, int, error) {
	var manualCount, ruleCount int
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE source = 'manual'),
			COUNT(*) FILTER (WHERE source = 'rule')
		FROM data_product_memberships
		WHERE data_product_id = $1`, dataProductID).Scan(&manualCount, &ruleCount)
	if err != nil {
		return 0, 0, fmt.Errorf("counting assets: %w", err)
	}

	return manualCount, ruleCount, nil
}

func (r *PostgresRepository) Create(ctx context.Context, dp *DataProduct, owners []OwnerInput) error {
	start := time.Now()

	metadataJSON, err := json.Marshal(dp.Metadata)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_create", time.Since(start), false)
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_create", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
		INSERT INTO data_products (name, description, metadata, tags, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	err = tx.QueryRow(ctx, q,
		dp.Name, dp.Description, metadataJSON, dp.Tags,
		dp.CreatedBy, dp.CreatedAt, dp.UpdatedAt,
	).Scan(&dp.ID)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_create", time.Since(start), false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("creating data product: %w", err)
	}

	if err := r.setOwners(ctx, tx, dp.ID, owners); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_create", time.Since(start), false)
		return fmt.Errorf("setting owners: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_create", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_create", time.Since(start), true)
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*DataProduct, error) {
	start := time.Now()

	q := `
		SELECT id, name, description, metadata, tags, created_by, created_at, updated_at
		FROM data_products
		WHERE id = $1`

	var dp DataProduct
	var metadataJSON []byte

	err := r.db.QueryRow(ctx, q, id).Scan(
		&dp.ID, &dp.Name, &dp.Description, &metadataJSON,
		&dp.Tags, &dp.CreatedBy, &dp.CreatedAt, &dp.UpdatedAt,
	)

	duration := time.Since(start)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.recorder.RecordDBQuery(ctx, "dataproduct_get", duration, true)
			return nil, ErrNotFound
		}
		r.recorder.RecordDBQuery(ctx, "dataproduct_get", duration, false)
		return nil, fmt.Errorf("getting data product: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &dp.Metadata); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get", duration, false)
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	dp.Owners, err = r.loadOwners(ctx, id)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get", duration, false)
		return nil, fmt.Errorf("loading owners: %w", err)
	}

	dp.Rules, err = r.loadRules(ctx, id)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get", duration, false)
		return nil, fmt.Errorf("loading rules: %w", err)
	}

	dp.ManualAssetCount, dp.RuleAssetCount, _ = r.getAssetCounts(ctx, id)
	dp.AssetCount = dp.ManualAssetCount + dp.RuleAssetCount

	if iconMeta, err := r.GetProductImageMeta(ctx, id, ImagePurposeIcon); err == nil {
		dp.IconURL = &iconMeta.URL
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_get", duration, true)
	return &dp, nil
}

func (r *PostgresRepository) Update(ctx context.Context, dp *DataProduct, owners []OwnerInput) error {
	start := time.Now()

	metadataJSON, err := json.Marshal(dp.Metadata)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update", time.Since(start), false)
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
		UPDATE data_products
		SET name = $1, description = $2, metadata = $3, tags = $4, updated_at = $5
		WHERE id = $6`

	result, err := tx.Exec(ctx, q,
		dp.Name, dp.Description, metadataJSON, dp.Tags, dp.UpdatedAt, dp.ID,
	)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update", duration, false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("updating data product: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update", duration, true)
		return ErrNotFound
	}

	if owners != nil {
		if err := r.setOwners(ctx, tx, dp.ID, owners); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_update", duration, false)
			return fmt.Errorf("setting owners: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update", duration, false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_update", duration, true)
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()

	result, err := r.db.Exec(ctx, "DELETE FROM data_products WHERE id = $1", id)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_delete", duration, false)
		return fmt.Errorf("deleting data product: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "dataproduct_delete", duration, true)
		return ErrNotFound
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_delete", duration, true)
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	start := time.Now()

	countQuery := `SELECT COUNT(*) FROM data_products`

	var total int
	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list_count", time.Since(start), false)
		return nil, fmt.Errorf("counting data products: %w", err)
	}

	q := `
		SELECT id, name, description, metadata, tags, created_by, created_at, updated_at
		FROM data_products
		ORDER BY name ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list", time.Since(start), false)
		return nil, fmt.Errorf("listing data products: %w", err)
	}
	defer rows.Close()

	var products []*DataProduct
	for rows.Next() {
		var dp DataProduct
		var metadataJSON []byte

		if err := rows.Scan(
			&dp.ID, &dp.Name, &dp.Description, &metadataJSON,
			&dp.Tags, &dp.CreatedBy, &dp.CreatedAt, &dp.UpdatedAt,
		); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_list", time.Since(start), false)
			return nil, fmt.Errorf("scanning data product: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &dp.Metadata); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_list", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		dp.Owners, err = r.loadOwners(ctx, dp.ID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_list", time.Since(start), false)
			return nil, fmt.Errorf("loading owners for %s: %w", dp.ID, err)
		}

		dp.ManualAssetCount, dp.RuleAssetCount, _ = r.getAssetCounts(ctx, dp.ID)
		dp.AssetCount = dp.ManualAssetCount + dp.RuleAssetCount

		if iconMeta, err := r.GetProductImageMeta(ctx, dp.ID, ImagePurposeIcon); err == nil {
			dp.IconURL = &iconMeta.URL
		}

		products = append(products, &dp)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list", time.Since(start), false)
		return nil, fmt.Errorf("iterating data products: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_list", time.Since(start), true)
	return &ListResult{DataProducts: products, Total: total}, nil
}

func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (*ListResult, error) {
	start := time.Now()

	args := []interface{}{}
	argCount := 1

	baseWhere := "WHERE 1=1"
	conditions := []string{}

	if filter.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(search_text @@ plainto_tsquery('english', $%d) OR name ILIKE $%d)", argCount, argCount+1))
		args = append(args, filter.Query)
		args = append(args, "%"+filter.Query+"%")
		argCount += 2
	}

	if len(filter.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags @> $%d", argCount))
		args = append(args, filter.Tags)
		argCount++
	}

	if len(filter.OwnerIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT data_product_id FROM data_product_owners WHERE user_id = ANY($%d) OR team_id = ANY($%d))", argCount, argCount))
		args = append(args, filter.OwnerIDs)
		argCount++
	}

	where := baseWhere
	if len(conditions) > 0 {
		where = fmt.Sprintf("%s AND %s", baseWhere, strings.Join(conditions, " AND "))
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM data_products %s", where)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_search_count", time.Since(start), false)
		return nil, fmt.Errorf("counting search results: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT id, name, description, metadata, tags, created_by, created_at, updated_at
		FROM data_products
		%s
		ORDER BY name ASC
		LIMIT $%d OFFSET $%d`, where, argCount, argCount+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_search", time.Since(start), false)
		return nil, fmt.Errorf("searching data products: %w", err)
	}
	defer rows.Close()

	var products []*DataProduct
	for rows.Next() {
		var dp DataProduct
		var metadataJSON []byte

		if err := rows.Scan(
			&dp.ID, &dp.Name, &dp.Description, &metadataJSON,
			&dp.Tags, &dp.CreatedBy, &dp.CreatedAt, &dp.UpdatedAt,
		); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_search", time.Since(start), false)
			return nil, fmt.Errorf("scanning search result: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &dp.Metadata); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_search", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		dp.Owners, err = r.loadOwners(ctx, dp.ID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_search", time.Since(start), false)
			return nil, fmt.Errorf("loading owners for %s: %w", dp.ID, err)
		}

		dp.ManualAssetCount, dp.RuleAssetCount, _ = r.getAssetCounts(ctx, dp.ID)
		dp.AssetCount = dp.ManualAssetCount + dp.RuleAssetCount

		if iconMeta, err := r.GetProductImageMeta(ctx, dp.ID, ImagePurposeIcon); err == nil {
			dp.IconURL = &iconMeta.URL
		}

		products = append(products, &dp)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_search", time.Since(start), false)
		return nil, fmt.Errorf("iterating search results: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_search", time.Since(start), true)
	return &ListResult{DataProducts: products, Total: total}, nil
}

func (r *PostgresRepository) AddAssets(ctx context.Context, dataProductID string, assetIDs []string, createdBy string) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_add_assets", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, assetID := range assetIDs {
		q := `
			INSERT INTO data_product_memberships (data_product_id, asset_id, source, rule_id)
			VALUES ($1, $2, $3, NULL)
			ON CONFLICT (data_product_id, asset_id) DO UPDATE SET source = $3, rule_id = NULL`

		_, err := tx.Exec(ctx, q, dataProductID, assetID, SourceManual)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_add_assets", time.Since(start), false)
			return fmt.Errorf("adding asset %s: %w", assetID, err)
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE data_products
		SET membership_count = (
			SELECT COUNT(*) FROM data_product_memberships WHERE data_product_id = $1
		),
		memberships_updated_at = NOW()
		WHERE id = $1`, dataProductID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_add_assets", time.Since(start), false)
		return fmt.Errorf("updating membership stats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_add_assets", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_add_assets", time.Since(start), true)
	return nil
}

func (r *PostgresRepository) RemoveAsset(ctx context.Context, dataProductID string, assetID string) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_remove_asset", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx,
		"DELETE FROM data_product_memberships WHERE data_product_id = $1 AND asset_id = $2 AND source = $3",
		dataProductID, assetID, SourceManual)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_remove_asset", time.Since(start), false)
		return fmt.Errorf("removing asset: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "dataproduct_remove_asset", time.Since(start), true)
		return ErrNotFound
	}

	_, err = tx.Exec(ctx, `
		UPDATE data_products
		SET membership_count = (
			SELECT COUNT(*) FROM data_product_memberships WHERE data_product_id = $1
		),
		memberships_updated_at = NOW()
		WHERE id = $1`, dataProductID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_remove_asset", time.Since(start), false)
		return fmt.Errorf("updating membership stats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_remove_asset", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_remove_asset", time.Since(start), true)
	return nil
}

func (r *PostgresRepository) GetManualAssets(ctx context.Context, dataProductID string, limit, offset int) (*AssetsResult, error) {
	start := time.Now()

	var total int
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM data_product_memberships WHERE data_product_id = $1 AND source = $2",
		dataProductID, SourceManual).Scan(&total)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_manual_assets", time.Since(start), false)
		return nil, fmt.Errorf("counting assets: %w", err)
	}

	q := `
		SELECT asset_id FROM data_product_memberships
		WHERE data_product_id = $1 AND source = $4
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, dataProductID, limit, offset, SourceManual)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_manual_assets", time.Since(start), false)
		return nil, fmt.Errorf("querying assets: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	for rows.Next() {
		var assetID string
		if err := rows.Scan(&assetID); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_get_manual_assets", time.Since(start), false)
			return nil, fmt.Errorf("scanning asset ID: %w", err)
		}
		assetIDs = append(assetIDs, assetID)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_manual_assets", time.Since(start), false)
		return nil, fmt.Errorf("iterating assets: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_get_manual_assets", time.Since(start), true)
	return &AssetsResult{AssetIDs: assetIDs, Total: total}, nil
}

func (r *PostgresRepository) CreateRule(ctx context.Context, dataProductID string, rule *RuleInput) (*Rule, error) {
	start := time.Now()

	q := `
		INSERT INTO data_product_rules (
			data_product_id, name, description, rule_type, query_expression,
			metadata_field, pattern_type, pattern_value, priority, is_enabled,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`

	now := time.Now().UTC()
	var id string
	err := r.db.QueryRow(ctx, q,
		dataProductID, rule.Name, rule.Description, rule.RuleType, rule.QueryExpression,
		rule.MetadataField, rule.PatternType, rule.PatternValue, rule.Priority, rule.IsEnabled,
		now, now,
	).Scan(&id)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_create_rule", duration, false)
		return nil, fmt.Errorf("creating rule: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_create_rule", duration, true)
	return r.GetRule(ctx, id)
}

func (r *PostgresRepository) UpdateRule(ctx context.Context, ruleID string, rule *RuleInput) (*Rule, error) {
	start := time.Now()

	q := `
		UPDATE data_product_rules
		SET name = $1, description = $2, rule_type = $3, query_expression = $4,
			metadata_field = $5, pattern_type = $6, pattern_value = $7,
			priority = $8, is_enabled = $9, updated_at = $10
		WHERE id = $11`

	result, err := r.db.Exec(ctx, q,
		rule.Name, rule.Description, rule.RuleType, rule.QueryExpression,
		rule.MetadataField, rule.PatternType, rule.PatternValue,
		rule.Priority, rule.IsEnabled, time.Now().UTC(), ruleID,
	)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update_rule", duration, false)
		return nil, fmt.Errorf("updating rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "dataproduct_update_rule", duration, true)
		return nil, ErrRuleNotFound
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_update_rule", duration, true)
	return r.GetRule(ctx, ruleID)
}

func (r *PostgresRepository) DeleteRule(ctx context.Context, ruleID string) error {
	start := time.Now()

	result, err := r.db.Exec(ctx, "DELETE FROM data_product_rules WHERE id = $1", ruleID)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_delete_rule", duration, false)
		return fmt.Errorf("deleting rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "dataproduct_delete_rule", duration, true)
		return ErrRuleNotFound
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_delete_rule", duration, true)
	return nil
}

func (r *PostgresRepository) GetRules(ctx context.Context, dataProductID string) ([]Rule, error) {
	return r.loadRules(ctx, dataProductID)
}

func (r *PostgresRepository) GetRule(ctx context.Context, ruleID string) (*Rule, error) {
	start := time.Now()

	q := `
		SELECT id, data_product_id, name, description, rule_type, query_expression,
			   metadata_field, pattern_type, pattern_value, priority, is_enabled,
			   created_at, updated_at
		FROM data_product_rules
		WHERE id = $1`

	var rule Rule
	err := r.db.QueryRow(ctx, q, ruleID).Scan(
		&rule.ID, &rule.DataProductID, &rule.Name, &rule.Description,
		&rule.RuleType, &rule.QueryExpression, &rule.MetadataField,
		&rule.PatternType, &rule.PatternValue, &rule.Priority,
		&rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt,
	)

	duration := time.Since(start)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.recorder.RecordDBQuery(ctx, "dataproduct_get_rule", duration, true)
			return nil, ErrRuleNotFound
		}
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_rule", duration, false)
		return nil, fmt.Errorf("getting rule: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_get_rule", duration, true)
	return &rule, nil
}

func (r *PostgresRepository) ResolveAssets(ctx context.Context, dataProductID string, limit, offset int) (*ResolvedAssets, error) {
	start := time.Now()

	var manualCount, ruleCount int
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE source = 'manual'),
			COUNT(*) FILTER (WHERE source = 'rule')
		FROM data_product_memberships
		WHERE data_product_id = $1`, dataProductID).Scan(&manualCount, &ruleCount)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_resolve_assets", time.Since(start), false)
		return nil, fmt.Errorf("counting memberships: %w", err)
	}

	total := manualCount + ruleCount

	q := `
		SELECT asset_id, source
		FROM data_product_memberships
		WHERE data_product_id = $1
		ORDER BY source, created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, dataProductID, limit, offset)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_resolve_assets", time.Since(start), false)
		return nil, fmt.Errorf("querying memberships: %w", err)
	}
	defer rows.Close()

	allAssets := []string{}
	manualAssets := []string{}
	dynamicAssets := []string{}
	for rows.Next() {
		var assetID, source string
		if err := rows.Scan(&assetID, &source); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_resolve_assets", time.Since(start), false)
			return nil, fmt.Errorf("scanning membership: %w", err)
		}
		allAssets = append(allAssets, assetID)
		if source == "manual" {
			manualAssets = append(manualAssets, assetID)
		} else {
			dynamicAssets = append(dynamicAssets, assetID)
		}
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_resolve_assets", time.Since(start), false)
		return nil, fmt.Errorf("iterating memberships: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_resolve_assets", time.Since(start), true)
	return &ResolvedAssets{
		ManualAssets:  manualAssets,
		DynamicAssets: dynamicAssets,
		AllAssets:     allAssets,
		Total:         total,
	}, nil
}

func (r *PostgresRepository) ExecuteRule(ctx context.Context, rule *Rule) ([]string, error) {
	start := time.Now()

	var assetIDs []string
	var err error

	switch {
	case rule.RuleType == RuleTypeQuery && rule.QueryExpression != nil:
		assetIDs, err = r.executeQueryRule(ctx, *rule.QueryExpression)
	case rule.RuleType == RuleTypeMetadataMatch:
		assetIDs, err = r.executeMetadataMatchRule(ctx, rule)
	default:
		return nil, fmt.Errorf("unsupported rule type: %s", rule.RuleType)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_execute_rule", time.Since(start), err == nil)
	return assetIDs, err
}

func (r *PostgresRepository) executeQueryRule(ctx context.Context, queryExpression string) ([]string, error) {
	parser := query.NewParser()
	builder := query.NewBuilder()

	parsedQuery, err := parser.Parse(queryExpression)
	if err != nil {
		return nil, fmt.Errorf("parsing query: %w", err)
	}

	// Base query without WHERE - BuildSQL will add WHERE clause
	baseQuery := `WITH search_results AS (SELECT id, 1.0 as search_rank FROM assets`

	sqlQuery, queryParams, err := builder.BuildSQL(parsedQuery, baseQuery)
	if err != nil {
		return nil, fmt.Errorf("building SQL: %w", err)
	}

	// Add is_stub filter after BuildSQL constructs the query
	// We need to inject it into the CTE before the closing paren
	sqlQuery = strings.Replace(sqlQuery,
		") SELECT * FROM search_results",
		" AND is_stub = FALSE) SELECT id, search_rank FROM search_results",
		1)

	// If there was no WHERE clause added by BuildSQL, we need to add WHERE instead of AND
	if !strings.Contains(sqlQuery, "WHERE") {
		sqlQuery = strings.Replace(sqlQuery,
			" AND is_stub = FALSE)",
			" WHERE is_stub = FALSE)",
			1)
	}

	// Query builder uses $2, $3, ... with empty $1 placeholder - renumber to $1, $2, ...
	sqlQuery = renumberParameters(sqlQuery)

	// Skip first element (empty placeholder) from builder params
	var params []interface{}
	if len(queryParams) > 1 {
		params = queryParams[1:]
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.db.Query(ctx, sqlQuery, params...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	for rows.Next() {
		var id string
		var rank float64
		if err := rows.Scan(&id, &rank); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		assetIDs = append(assetIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating results: %w", err)
	}

	return assetIDs, nil
}

func (r *PostgresRepository) executeMetadataMatchRule(ctx context.Context, rule *Rule) ([]string, error) {
	if rule.MetadataField == nil || rule.PatternType == nil || rule.PatternValue == nil {
		return nil, fmt.Errorf("metadata match rule missing required fields")
	}

	var condition string
	var args []interface{}

	fieldPath := strings.Split(*rule.MetadataField, ".")
	var columnRef string
	if len(fieldPath) > 1 {
		jsonPath := ""
		for i, field := range fieldPath[:len(fieldPath)-1] {
			if i > 0 {
				jsonPath += "->"
			}
			jsonPath += fmt.Sprintf("'%s'", field)
		}
		columnRef = fmt.Sprintf("metadata->%s->>'%s'", jsonPath, fieldPath[len(fieldPath)-1])
	} else {
		columnRef = fmt.Sprintf("metadata->>'%s'", fieldPath[0])
	}

	switch *rule.PatternType {
	case PatternTypeExact:
		condition = fmt.Sprintf("%s = $1", columnRef)
		args = append(args, *rule.PatternValue)
	case PatternTypeWildcard:
		pattern := strings.ReplaceAll(*rule.PatternValue, "*", "%")
		condition = fmt.Sprintf("%s ILIKE $1", columnRef)
		args = append(args, pattern)
	case PatternTypeRegex:
		if _, err := regexp.Compile(*rule.PatternValue); err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		condition = fmt.Sprintf("%s ~ $1", columnRef)
		args = append(args, *rule.PatternValue)
	case PatternTypePrefix:
		condition = fmt.Sprintf("%s LIKE $1", columnRef)
		args = append(args, *rule.PatternValue+"%")
	default:
		return nil, fmt.Errorf("unsupported pattern type: %s", *rule.PatternType)
	}

	q := fmt.Sprintf("SELECT id FROM assets WHERE is_stub = FALSE AND %s", condition)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("executing metadata query: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		assetIDs = append(assetIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating results: %w", err)
	}

	return assetIDs, nil
}

func (r *PostgresRepository) PreviewRule(ctx context.Context, rule *RuleInput, limit int) (*RulePreview, error) {
	start := time.Now()

	tempRule := &Rule{
		RuleType:        rule.RuleType,
		QueryExpression: rule.QueryExpression,
		MetadataField:   rule.MetadataField,
		PatternType:     rule.PatternType,
		PatternValue:    rule.PatternValue,
		IsEnabled:       true,
	}

	assetIDs, err := r.ExecuteRule(ctx, tempRule)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_preview_rule", time.Since(start), false)
		return &RulePreview{
			AssetIDs:   []string{},
			AssetCount: 0,
			Errors:     []string{err.Error()},
		}, nil
	}

	total := len(assetIDs)
	if limit > 0 && limit < len(assetIDs) {
		assetIDs = assetIDs[:limit]
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_preview_rule", time.Since(start), true)
	return &RulePreview{
		AssetIDs:   assetIDs,
		AssetCount: total,
	}, nil
}

func (r *PostgresRepository) GetDataProductsForAsset(ctx context.Context, assetID string) ([]*DataProduct, error) {
	start := time.Now()

	q := `
		SELECT DISTINCT dp.id, dp.name, dp.description, dp.metadata, dp.tags,
			   dp.created_by, dp.created_at, dp.updated_at
		FROM data_products dp
		JOIN data_product_memberships dpm ON dp.id = dpm.data_product_id
		WHERE dpm.asset_id = $1
		ORDER BY dp.name ASC`

	rows, err := r.db.Query(ctx, q, assetID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_for_asset", time.Since(start), false)
		return nil, fmt.Errorf("querying data products: %w", err)
	}
	defer rows.Close()

	var products []*DataProduct
	for rows.Next() {
		var dp DataProduct
		var metadataJSON []byte

		if err := rows.Scan(
			&dp.ID, &dp.Name, &dp.Description, &metadataJSON,
			&dp.Tags, &dp.CreatedBy, &dp.CreatedAt, &dp.UpdatedAt,
		); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_get_for_asset", time.Since(start), false)
			return nil, fmt.Errorf("scanning data product: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &dp.Metadata); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_get_for_asset", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		dp.Owners, err = r.loadOwners(ctx, dp.ID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_get_for_asset", time.Since(start), false)
			return nil, fmt.Errorf("loading owners for %s: %w", dp.ID, err)
		}

		products = append(products, &dp)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_for_asset", time.Since(start), false)
		return nil, fmt.Errorf("iterating data products: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_get_for_asset", time.Since(start), true)
	return products, nil
}

type ImagePurpose string

const (
	ImagePurposeIcon   ImagePurpose = "icon"
	ImagePurposeHeader ImagePurpose = "header"
)

const (
	MaxImageSizeBytes = 5 * 1024 * 1024 // 5MB per image
)

var ValidImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

var ErrImageNotFound = errors.New("image not found")
var ErrImageTooLarge = errors.New("image exceeds maximum size")
var ErrInvalidImageType = errors.New("invalid image type")

type ProductImage struct {
	ID            string       `json:"id"`
	DataProductID string       `json:"data_product_id"`
	Purpose       ImagePurpose `json:"purpose"`
	Filename      string       `json:"filename"`
	ContentType   string       `json:"content_type"`
	SizeBytes     int          `json:"size_bytes"`
	Data          []byte       `json:"-"`
	CreatedAt     time.Time    `json:"created_at"`
	CreatedBy     *string      `json:"created_by,omitempty"`
}

type ProductImageMeta struct {
	ID            string       `json:"id"`
	DataProductID string       `json:"data_product_id"`
	Purpose       ImagePurpose `json:"purpose"`
	Filename      string       `json:"filename"`
	ContentType   string       `json:"content_type"`
	SizeBytes     int          `json:"size_bytes"`
	URL           string       `json:"url"`
	CreatedAt     time.Time    `json:"created_at"`
}

type UploadImageInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

func (r *PostgresRepository) UploadProductImage(ctx context.Context, dataProductID string, purpose ImagePurpose, input UploadImageInput, createdBy *string) (*ProductImage, error) {
	start := time.Now()

	query := `
		INSERT INTO product_images (data_product_id, purpose, filename, content_type, size_bytes, data, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (data_product_id, purpose)
		DO UPDATE SET filename = EXCLUDED.filename, content_type = EXCLUDED.content_type,
		              size_bytes = EXCLUDED.size_bytes, data = EXCLUDED.data,
		              created_at = NOW(), created_by = EXCLUDED.created_by
		RETURNING id, data_product_id, purpose, filename, content_type, size_bytes, created_at, created_by`

	var image ProductImage
	err := r.db.QueryRow(ctx, query,
		dataProductID, purpose, input.Filename, input.ContentType, len(input.Data), input.Data, createdBy,
	).Scan(
		&image.ID, &image.DataProductID, &image.Purpose,
		&image.Filename, &image.ContentType, &image.SizeBytes, &image.CreatedAt, &image.CreatedBy,
	)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_upload_image", duration, false)
		return nil, fmt.Errorf("uploading image: %w", err)
	}

	image.Data = input.Data
	r.recorder.RecordDBQuery(ctx, "dataproduct_upload_image", duration, true)
	return &image, nil
}

func (r *PostgresRepository) GetProductImage(ctx context.Context, imageID string) (*ProductImage, error) {
	start := time.Now()

	query := `
		SELECT id, data_product_id, purpose, filename, content_type, size_bytes, data, created_at, created_by
		FROM product_images
		WHERE id = $1`

	var image ProductImage
	err := r.db.QueryRow(ctx, query, imageID).Scan(
		&image.ID, &image.DataProductID, &image.Purpose,
		&image.Filename, &image.ContentType, &image.SizeBytes, &image.Data, &image.CreatedAt, &image.CreatedBy,
	)

	duration := time.Since(start)

	if errors.Is(err, pgx.ErrNoRows) {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_image", duration, true)
		return nil, ErrImageNotFound
	}
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_image", duration, false)
		return nil, fmt.Errorf("getting image: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_get_image", duration, true)
	return &image, nil
}

func (r *PostgresRepository) GetProductImageByPurpose(ctx context.Context, dataProductID string, purpose ImagePurpose) (*ProductImage, error) {
	start := time.Now()

	query := `
		SELECT id, data_product_id, purpose, filename, content_type, size_bytes, data, created_at, created_by
		FROM product_images
		WHERE data_product_id = $1 AND purpose = $2`

	var image ProductImage
	err := r.db.QueryRow(ctx, query, dataProductID, purpose).Scan(
		&image.ID, &image.DataProductID, &image.Purpose,
		&image.Filename, &image.ContentType, &image.SizeBytes, &image.Data, &image.CreatedAt, &image.CreatedBy,
	)

	duration := time.Since(start)

	if errors.Is(err, pgx.ErrNoRows) {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_image_by_purpose", duration, true)
		return nil, ErrImageNotFound
	}
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_image_by_purpose", duration, false)
		return nil, fmt.Errorf("getting image by purpose: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_get_image_by_purpose", duration, true)
	return &image, nil
}

func (r *PostgresRepository) GetProductImageMeta(ctx context.Context, dataProductID string, purpose ImagePurpose) (*ProductImageMeta, error) {
	start := time.Now()

	query := `
		SELECT id, data_product_id, purpose, filename, content_type, size_bytes, created_at
		FROM product_images
		WHERE data_product_id = $1 AND purpose = $2`

	var meta ProductImageMeta
	err := r.db.QueryRow(ctx, query, dataProductID, purpose).Scan(
		&meta.ID, &meta.DataProductID, &meta.Purpose,
		&meta.Filename, &meta.ContentType, &meta.SizeBytes, &meta.CreatedAt,
	)

	duration := time.Since(start)

	if errors.Is(err, pgx.ErrNoRows) {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_image_meta", duration, true)
		return nil, ErrImageNotFound
	}
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_get_image_meta", duration, false)
		return nil, fmt.Errorf("getting image metadata: %w", err)
	}

	meta.URL = fmt.Sprintf("/api/v1/products/images/%s/%s", meta.DataProductID, meta.Purpose)
	r.recorder.RecordDBQuery(ctx, "dataproduct_get_image_meta", duration, true)
	return &meta, nil
}

func (r *PostgresRepository) DeleteProductImage(ctx context.Context, dataProductID string, purpose ImagePurpose) error {
	start := time.Now()

	result, err := r.db.Exec(ctx,
		"DELETE FROM product_images WHERE data_product_id = $1 AND purpose = $2",
		dataProductID, purpose)

	duration := time.Since(start)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_delete_image", duration, false)
		return fmt.Errorf("deleting image: %w", err)
	}
	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "dataproduct_delete_image", duration, true)
		return ErrImageNotFound
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_delete_image", duration, true)
	return nil
}

func (r *PostgresRepository) ListProductImages(ctx context.Context, dataProductID string) ([]*ProductImageMeta, error) {
	start := time.Now()

	query := `
		SELECT id, data_product_id, purpose, filename, content_type, size_bytes, created_at
		FROM product_images
		WHERE data_product_id = $1
		ORDER BY purpose, created_at`

	rows, err := r.db.Query(ctx, query, dataProductID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list_images", time.Since(start), false)
		return nil, fmt.Errorf("listing product images: %w", err)
	}
	defer rows.Close()

	var images []*ProductImageMeta
	for rows.Next() {
		var meta ProductImageMeta
		err := rows.Scan(
			&meta.ID, &meta.DataProductID, &meta.Purpose,
			&meta.Filename, &meta.ContentType, &meta.SizeBytes, &meta.CreatedAt,
		)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_list_images", time.Since(start), false)
			return nil, fmt.Errorf("scanning image: %w", err)
		}
		meta.URL = fmt.Sprintf("/api/v1/products/images/%s/%s", meta.DataProductID, meta.Purpose)
		images = append(images, &meta)
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_list_images", time.Since(start), true)
	return images, rows.Err()
}
