package dataproduct

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/query"
	"github.com/rs/zerolog/log"
)

// Membership represents a precomputed relationship between a data product and an asset.
type Membership struct {
	DataProductID string
	AssetID       string
	Source        string  // SourceManual or SourceRule
	RuleID        *string // nil for manual memberships
	CreatedAt     time.Time
}

// RuleTarget represents what a rule is targeting for fast candidate lookup.
type RuleTarget struct {
	RuleID        string
	DataProductID string
	TargetType    string // TargetTypeAssetType, TargetTypeProvider, TargetTypeTag, TargetTypeMetadataKey, or TargetTypeQuery
	TargetValue   string // empty string for complex queries
}

// AssetSignature contains the key fields used for rule matching.
type AssetSignature struct {
	ID           string
	Type         string
	Providers    []string
	Tags         []string
	MetadataKeys []string
}

// CandidateRule represents a rule that might match an asset.
type CandidateRule struct {
	RuleID        string
	DataProductID string
}

// MembershipRepository handles database operations for memberships.
type MembershipRepository interface {
	// Membership operations
	CreateMemberships(ctx context.Context, memberships []Membership) error
	DeleteMembershipsByAsset(ctx context.Context, assetID string) error
	DeleteMembershipsByRule(ctx context.Context, ruleID string) error
	DeleteMembershipsByDataProduct(ctx context.Context, dataProductID string) error
	GetMemberships(ctx context.Context, dataProductID string, limit, offset int) ([]Membership, int, error)
	GetDataProductsForAsset(ctx context.Context, assetID string) ([]string, error)

	// Rule target operations
	SaveRuleTargets(ctx context.Context, ruleID, dataProductID string, targets []RuleTarget) error
	DeleteRuleTargets(ctx context.Context, ruleID string) error
	FindCandidateRules(ctx context.Context, sig AssetSignature) ([]CandidateRule, error)

	// Rule evaluation
	EvaluateRuleForAsset(ctx context.Context, rule *Rule, assetID string) (bool, error)

	// Stats
	UpdateMembershipStats(ctx context.Context, dataProductID string) error
}

// PostgresMembershipRepository implements MembershipRepository for PostgreSQL.
type PostgresMembershipRepository struct {
	db       *pgxpool.Pool
	recorder metrics.Recorder
}

// NewPostgresMembershipRepository creates a new PostgreSQL membership repository.
func NewPostgresMembershipRepository(db *pgxpool.Pool, recorder metrics.Recorder) *PostgresMembershipRepository {
	return &PostgresMembershipRepository{
		db:       db,
		recorder: recorder,
	}
}

// CreateMemberships inserts multiple memberships, ignoring duplicates.
func (r *PostgresMembershipRepository) CreateMemberships(ctx context.Context, memberships []Membership) error {
	if len(memberships) == 0 {
		return nil
	}

	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "membership_create_batch", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Batch insert with ON CONFLICT DO NOTHING
	for _, m := range memberships {
		_, err := tx.Exec(ctx, `
			INSERT INTO data_product_memberships (data_product_id, asset_id, source, rule_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (data_product_id, asset_id) DO NOTHING`,
			m.DataProductID, m.AssetID, m.Source, m.RuleID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "membership_create_batch", time.Since(start), false)
			return fmt.Errorf("inserting membership: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "membership_create_batch", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "membership_create_batch", time.Since(start), true)

	// Update stats for affected data products
	productIDs := make(map[string]struct{})
	for _, m := range memberships {
		productIDs[m.DataProductID] = struct{}{}
	}
	for productID := range productIDs {
		if err := r.UpdateMembershipStats(ctx, productID); err != nil {
			log.Warn().Err(err).Str("product_id", productID).Msg("Failed to update membership stats")
		}
	}

	return nil
}

// DeleteMembershipsByAsset removes all memberships for a given asset.
func (r *PostgresMembershipRepository) DeleteMembershipsByAsset(ctx context.Context, assetID string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `
		DELETE FROM data_product_memberships WHERE asset_id = $1`, assetID)

	r.recorder.RecordDBQuery(ctx, "membership_delete_by_asset", time.Since(start), err == nil)

	if err != nil {
		return fmt.Errorf("deleting memberships by asset: %w", err)
	}

	return nil
}

// DeleteMembershipsByRule removes all memberships created by a specific rule.
func (r *PostgresMembershipRepository) DeleteMembershipsByRule(ctx context.Context, ruleID string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `
		DELETE FROM data_product_memberships WHERE rule_id = $1`, ruleID)

	r.recorder.RecordDBQuery(ctx, "membership_delete_by_rule", time.Since(start), err == nil)

	if err != nil {
		return fmt.Errorf("deleting memberships by rule: %w", err)
	}

	return nil
}

// DeleteMembershipsByDataProduct removes all memberships for a data product.
func (r *PostgresMembershipRepository) DeleteMembershipsByDataProduct(ctx context.Context, dataProductID string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `
		DELETE FROM data_product_memberships WHERE data_product_id = $1`, dataProductID)

	r.recorder.RecordDBQuery(ctx, "membership_delete_by_product", time.Since(start), err == nil)

	if err != nil {
		return fmt.Errorf("deleting memberships by product: %w", err)
	}

	return nil
}

// GetMemberships returns all memberships for a data product with pagination.
func (r *PostgresMembershipRepository) GetMemberships(ctx context.Context, dataProductID string, limit, offset int) ([]Membership, int, error) {
	start := time.Now()

	// Get total count
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM data_product_memberships WHERE data_product_id = $1`,
		dataProductID).Scan(&total)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "membership_get_count", time.Since(start), false)
		return nil, 0, fmt.Errorf("counting memberships: %w", err)
	}

	// Get memberships
	rows, err := r.db.Query(ctx, `
		SELECT data_product_id, asset_id, source, rule_id, created_at
		FROM data_product_memberships
		WHERE data_product_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		dataProductID, limit, offset)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "membership_get", time.Since(start), false)
		return nil, 0, fmt.Errorf("querying memberships: %w", err)
	}
	defer rows.Close()

	var memberships []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.DataProductID, &m.AssetID, &m.Source, &m.RuleID, &m.CreatedAt); err != nil {
			r.recorder.RecordDBQuery(ctx, "membership_get", time.Since(start), false)
			return nil, 0, fmt.Errorf("scanning membership: %w", err)
		}
		memberships = append(memberships, m)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "membership_get", time.Since(start), false)
		return nil, 0, fmt.Errorf("iterating memberships: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "membership_get", time.Since(start), true)
	return memberships, total, nil
}

// GetDataProductsForAsset returns all data product IDs that contain an asset.
func (r *PostgresMembershipRepository) GetDataProductsForAsset(ctx context.Context, assetID string) ([]string, error) {
	start := time.Now()

	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT data_product_id
		FROM data_product_memberships
		WHERE asset_id = $1`,
		assetID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "membership_get_products_for_asset", time.Since(start), false)
		return nil, fmt.Errorf("querying data products for asset: %w", err)
	}
	defer rows.Close()

	var productIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.recorder.RecordDBQuery(ctx, "membership_get_products_for_asset", time.Since(start), false)
			return nil, fmt.Errorf("scanning product ID: %w", err)
		}
		productIDs = append(productIDs, id)
	}

	r.recorder.RecordDBQuery(ctx, "membership_get_products_for_asset", time.Since(start), true)
	return productIDs, nil
}

// SaveRuleTargets replaces the targets for a rule.
func (r *PostgresMembershipRepository) SaveRuleTargets(ctx context.Context, ruleID, dataProductID string, targets []RuleTarget) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_targets_save", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete existing targets
	_, err = tx.Exec(ctx, `DELETE FROM data_product_rule_targets WHERE rule_id = $1`, ruleID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_targets_save", time.Since(start), false)
		return fmt.Errorf("deleting existing targets: %w", err)
	}

	// Insert new targets
	for _, t := range targets {
		_, err := tx.Exec(ctx, `
			INSERT INTO data_product_rule_targets (rule_id, data_product_id, target_type, target_value)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING`,
			ruleID, dataProductID, t.TargetType, t.TargetValue)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "rule_targets_save", time.Since(start), false)
			return fmt.Errorf("inserting target: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_targets_save", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "rule_targets_save", time.Since(start), true)
	return nil
}

// DeleteRuleTargets removes all targets for a rule.
func (r *PostgresMembershipRepository) DeleteRuleTargets(ctx context.Context, ruleID string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `DELETE FROM data_product_rule_targets WHERE rule_id = $1`, ruleID)

	r.recorder.RecordDBQuery(ctx, "rule_targets_delete", time.Since(start), err == nil)

	if err != nil {
		return fmt.Errorf("deleting rule targets: %w", err)
	}

	return nil
}

// FindCandidateRules finds rules that might match an asset based on its signature.
func (r *PostgresMembershipRepository) FindCandidateRules(ctx context.Context, sig AssetSignature) ([]CandidateRule, error) {
	start := time.Now()

	// This query finds rules where ANY target matches the asset
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT t.rule_id, t.data_product_id
		FROM data_product_rule_targets t
		JOIN data_product_rules r ON t.rule_id = r.id
		WHERE r.is_enabled = TRUE
		AND (
			(t.target_type = 'asset_type' AND t.target_value = $1)
			OR (t.target_type = 'provider' AND t.target_value = ANY($2))
			OR (t.target_type = 'tag' AND t.target_value = ANY($3))
			OR (t.target_type = 'metadata_key' AND t.target_value = ANY($4))
			OR (t.target_type = 'query')
		)`,
		sig.Type, sig.Providers, sig.Tags, sig.MetadataKeys)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_targets_find_candidates", time.Since(start), false)
		return nil, fmt.Errorf("querying candidate rules: %w", err)
	}
	defer rows.Close()

	var candidates []CandidateRule
	for rows.Next() {
		var c CandidateRule
		if err := rows.Scan(&c.RuleID, &c.DataProductID); err != nil {
			r.recorder.RecordDBQuery(ctx, "rule_targets_find_candidates", time.Since(start), false)
			return nil, fmt.Errorf("scanning candidate: %w", err)
		}
		candidates = append(candidates, c)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_targets_find_candidates", time.Since(start), false)
		return nil, fmt.Errorf("iterating candidates: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "rule_targets_find_candidates", time.Since(start), true)

	log.Debug().
		Str("asset_id", sig.ID).
		Int("candidates", len(candidates)).
		Msg("Found candidate rules for asset")

	return candidates, nil
}

// EvaluateRuleForAsset checks if a specific asset matches a rule.
func (r *PostgresMembershipRepository) EvaluateRuleForAsset(ctx context.Context, rule *Rule, assetID string) (bool, error) {
	start := time.Now()

	if rule.RuleType == RuleTypeQuery && rule.QueryExpression != nil {
		return r.evaluateQueryRuleForAsset(ctx, *rule.QueryExpression, assetID)
	}

	if rule.RuleType == RuleTypeMetadataMatch {
		return r.evaluateMetadataRuleForAsset(ctx, rule, assetID)
	}

	r.recorder.RecordDBQuery(ctx, "rule_evaluate_for_asset", time.Since(start), false)
	return false, fmt.Errorf("unsupported rule type: %s", rule.RuleType)
}

func (r *PostgresMembershipRepository) evaluateQueryRuleForAsset(ctx context.Context, queryExpression, assetID string) (bool, error) {
	start := time.Now()

	parser := query.NewParser()
	builder := query.NewBuilder()

	parsedQuery, err := parser.Parse(queryExpression)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_evaluate_query", time.Since(start), false)
		return false, fmt.Errorf("parsing query: %w", err)
	}

	// Base query without WHERE - BuildSQL will add WHERE clause
	baseQuery := `WITH search_results AS (SELECT id, 1.0 as search_rank FROM assets`
	sqlQuery, queryParams, err := builder.BuildSQL(parsedQuery, baseQuery)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "rule_evaluate_query", time.Since(start), false)
		return false, fmt.Errorf("building SQL: %w", err)
	}

	// Add is_stub filter after BuildSQL constructs the query
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

	// Add asset ID filter - the param number is now len(params) + 1
	nextParam := len(params) + 1
	checkQuery := fmt.Sprintf(
		"SELECT EXISTS(SELECT 1 FROM (%s) AS results WHERE id = $%d)",
		sqlQuery, nextParam,
	)
	params = append(params, assetID)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var exists bool
	err = r.db.QueryRow(ctx, checkQuery, params...).Scan(&exists)

	r.recorder.RecordDBQuery(ctx, "rule_evaluate_query", time.Since(start), err == nil)

	if err != nil {
		return false, fmt.Errorf("executing query: %w", err)
	}

	return exists, nil
}

// renumberParameters renumbers SQL parameters from $2, $3, ... to $1, $2, ...
// Processes from highest to lowest to avoid conflicts during replacement.
func renumberParameters(sql string) string {
	for i := 20; i >= 2; i-- {
		old := fmt.Sprintf("$%d", i)
		new := fmt.Sprintf("$%d", i-1)
		sql = strings.ReplaceAll(sql, old, new)
	}
	return sql
}

func (r *PostgresMembershipRepository) evaluateMetadataRuleForAsset(ctx context.Context, rule *Rule, assetID string) (bool, error) {
	start := time.Now()

	if rule.MetadataField == nil || rule.PatternType == nil || rule.PatternValue == nil {
		r.recorder.RecordDBQuery(ctx, "rule_evaluate_metadata", time.Since(start), false)
		return false, fmt.Errorf("metadata match rule missing required fields")
	}

	// Build the JSON path expression
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

	var condition string
	var args []interface{}
	args = append(args, assetID)

	switch *rule.PatternType {
	case PatternTypeExact:
		condition = fmt.Sprintf("%s = $2", columnRef)
		args = append(args, *rule.PatternValue)
	case PatternTypeWildcard:
		pattern := strings.ReplaceAll(*rule.PatternValue, "*", "%")
		condition = fmt.Sprintf("%s ILIKE $2", columnRef)
		args = append(args, pattern)
	case PatternTypeRegex:
		condition = fmt.Sprintf("%s ~ $2", columnRef)
		args = append(args, *rule.PatternValue)
	case PatternTypePrefix:
		condition = fmt.Sprintf("%s LIKE $2", columnRef)
		args = append(args, *rule.PatternValue+"%")
	default:
		r.recorder.RecordDBQuery(ctx, "rule_evaluate_metadata", time.Since(start), false)
		return false, fmt.Errorf("unsupported pattern type: %s", *rule.PatternType)
	}

	q := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM assets WHERE id = $1 AND is_stub = FALSE AND %s)", condition)

	var exists bool
	err := r.db.QueryRow(ctx, q, args...).Scan(&exists)

	r.recorder.RecordDBQuery(ctx, "rule_evaluate_metadata", time.Since(start), err == nil)

	if err != nil {
		return false, fmt.Errorf("executing metadata query: %w", err)
	}

	return exists, nil
}

// UpdateMembershipStats updates the membership count on a data product.
func (r *PostgresMembershipRepository) UpdateMembershipStats(ctx context.Context, dataProductID string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `
		UPDATE data_products
		SET membership_count = (
			SELECT COUNT(*) FROM data_product_memberships WHERE data_product_id = $1
		),
		memberships_updated_at = NOW()
		WHERE id = $1`,
		dataProductID)

	r.recorder.RecordDBQuery(ctx, "membership_update_stats", time.Since(start), err == nil)

	if err != nil {
		return fmt.Errorf("updating membership stats: %w", err)
	}

	return nil
}
