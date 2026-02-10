package assetrule

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/enrichment"
	"github.com/marmotdata/marmot/internal/metrics"
)

var (
	ErrNotFound     = errors.New("asset rule not found")
	ErrConflict     = errors.New("asset rule with this name already exists")
	ErrInvalidInput = errors.New("invalid input")
)

// ExternalLink represents a link that a rule assigns to matching assets.
type ExternalLink struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
	URL  string `json:"url"`
}

// AssetRule represents a governance rule that assigns external links and/or glossary terms to assets.
type AssetRule struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Description     *string             `json:"description,omitempty"`
	Links           []ExternalLink      `json:"links"`
	TermIDs         []string            `json:"term_ids"`
	RuleType        enrichment.RuleType `json:"rule_type"`
	QueryExpression *string             `json:"query_expression,omitempty"`
	MetadataField   *string             `json:"metadata_field,omitempty"`
	PatternType     *string             `json:"pattern_type,omitempty"`
	PatternValue    *string             `json:"pattern_value,omitempty"`
	Priority        int                 `json:"priority"`
	IsEnabled       bool                `json:"is_enabled"`
	CreatedBy       *string             `json:"created_by,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`

	MembershipCount    int        `json:"membership_count"`
	LastReconciledAt   *time.Time `json:"last_reconciled_at,omitempty"`
	ReconciliationHash *string    `json:"reconciliation_hash,omitempty"`
}

// Implement enrichment.EnrichmentRule interface.
func (r *AssetRule) GetID() string                    { return r.ID }
func (r *AssetRule) GetRuleType() enrichment.RuleType { return r.RuleType }
func (r *AssetRule) GetQueryExpression() *string       { return r.QueryExpression }
func (r *AssetRule) GetMetadataField() *string         { return r.MetadataField }
func (r *AssetRule) GetPatternType() *string           { return r.PatternType }
func (r *AssetRule) GetPatternValue() *string          { return r.PatternValue }
func (r *AssetRule) GetIsEnabled() bool                { return r.IsEnabled }

// ComputeHash computes a hash of the rule's config for differential reconciliation.
func (r *AssetRule) ComputeHash() string {
	h := sha256.New()
	h.Write([]byte(string(r.RuleType)))
	if r.QueryExpression != nil {
		h.Write([]byte(*r.QueryExpression))
	}
	if r.MetadataField != nil {
		h.Write([]byte(*r.MetadataField))
	}
	if r.PatternType != nil {
		h.Write([]byte(*r.PatternType))
	}
	if r.PatternValue != nil {
		h.Write([]byte(*r.PatternValue))
	}
	linksJSON, _ := json.Marshal(r.Links)
	h.Write(linksJSON)
	for _, id := range r.TermIDs {
		h.Write([]byte(id))
	}
	h.Write([]byte(fmt.Sprintf("%t", r.IsEnabled)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// RuleSource returns the source string for asset_terms rows.
func (r *AssetRule) RuleSource() string {
	return "rule:" + r.ID
}

// EnrichedExternalLink is an external link with source information.
type EnrichedExternalLink struct {
	asset.ExternalLink
	Source   string  `json:"source"`
	RuleID   *string `json:"rule_id,omitempty"`
	RuleName *string `json:"rule_name,omitempty"`
}

// SearchFilter for searching asset rules.
type SearchFilter struct {
	Query  string `json:"query,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// ListResult is the result of listing asset rules.
type ListResult struct {
	AssetRules []*AssetRule `json:"asset_rules"`
	Total      int          `json:"total"`
}

// RulePreview is the result of previewing a rule.
type RulePreview struct {
	AssetIDs   []string `json:"asset_ids"`
	AssetCount int      `json:"asset_count"`
	Errors     []string `json:"errors,omitempty"`
}

// Repository handles database operations for asset rules.
type Repository interface {
	Create(ctx context.Context, rule *AssetRule) error
	Get(ctx context.Context, id string) (*AssetRule, error)
	Update(ctx context.Context, rule *AssetRule) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter) (*ListResult, error)
	GetAllEnabled(ctx context.Context) ([]*AssetRule, error)
	UpdateReconciliationState(ctx context.Context, ruleID string, hash string) error
	SetTerms(ctx context.Context, ruleID string, termIDs []string) error
	GetTermIDs(ctx context.Context, ruleID string) ([]string, error)

	// For asset detail page: get rule-managed links for an asset
	GetRuleManagedLinks(ctx context.Context, assetID string) ([]EnrichedExternalLink, error)
}

// PostgresRepository implements Repository for PostgreSQL.
type PostgresRepository struct {
	db       *pgxpool.Pool
	recorder metrics.Recorder
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(db *pgxpool.Pool, recorder metrics.Recorder) *PostgresRepository {
	return &PostgresRepository{db: db, recorder: recorder}
}

func (r *PostgresRepository) Create(ctx context.Context, rule *AssetRule) error {
	start := time.Now()

	linksJSON, err := json.Marshal(rule.Links)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_create", time.Since(start), false)
		return fmt.Errorf("marshaling links: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_create", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
		INSERT INTO asset_rules (name, description, links, rule_type, query_expression,
			metadata_field, pattern_type, pattern_value, priority, is_enabled,
			created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	err = tx.QueryRow(ctx, q,
		rule.Name, rule.Description, linksJSON, rule.RuleType, rule.QueryExpression,
		rule.MetadataField, rule.PatternType, rule.PatternValue, rule.Priority, rule.IsEnabled,
		rule.CreatedBy, rule.CreatedAt, rule.UpdatedAt,
	).Scan(&rule.ID)

	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_create", time.Since(start), false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("creating asset rule: %w", err)
	}

	// Insert term associations
	for _, termID := range rule.TermIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO asset_rule_terms (asset_rule_id, glossary_term_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, rule.ID, termID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_create", time.Since(start), false)
			return fmt.Errorf("inserting term association: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_create", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_create", time.Since(start), true)
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*AssetRule, error) {
	start := time.Now()

	q := `
		SELECT id, name, description, links, rule_type, query_expression,
			metadata_field, pattern_type, pattern_value, priority, is_enabled,
			created_by, created_at, updated_at, membership_count,
			last_reconciled_at, reconciliation_hash
		FROM asset_rules
		WHERE id = $1`

	rule, err := r.scanRule(r.db.QueryRow(ctx, q, id))

	duration := time.Since(start)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.recorder.RecordDBQuery(ctx, "assetrule_get", duration, true)
			return nil, ErrNotFound
		}
		r.recorder.RecordDBQuery(ctx, "assetrule_get", duration, false)
		return nil, fmt.Errorf("getting asset rule: %w", err)
	}

	// Load term IDs
	rule.TermIDs, err = r.GetTermIDs(ctx, id)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_get", duration, false)
		return nil, fmt.Errorf("loading term IDs: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_get", duration, true)
	return rule, nil
}

func (r *PostgresRepository) Update(ctx context.Context, rule *AssetRule) error {
	start := time.Now()

	linksJSON, err := json.Marshal(rule.Links)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_update", time.Since(start), false)
		return fmt.Errorf("marshaling links: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_update", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
		UPDATE asset_rules
		SET name = $1, description = $2, links = $3, rule_type = $4, query_expression = $5,
			metadata_field = $6, pattern_type = $7, pattern_value = $8,
			priority = $9, is_enabled = $10, updated_at = $11
		WHERE id = $12`

	result, err := tx.Exec(ctx, q,
		rule.Name, rule.Description, linksJSON, rule.RuleType, rule.QueryExpression,
		rule.MetadataField, rule.PatternType, rule.PatternValue,
		rule.Priority, rule.IsEnabled, rule.UpdatedAt, rule.ID,
	)

	duration := time.Since(start)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_update", duration, false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("updating asset rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "assetrule_update", duration, true)
		return ErrNotFound
	}

	// Update term associations
	_, err = tx.Exec(ctx, `DELETE FROM asset_rule_terms WHERE asset_rule_id = $1`, rule.ID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_update", duration, false)
		return fmt.Errorf("clearing term associations: %w", err)
	}

	for _, termID := range rule.TermIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO asset_rule_terms (asset_rule_id, glossary_term_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, rule.ID, termID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_update", duration, false)
			return fmt.Errorf("inserting term association: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_update", duration, false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_update", duration, true)
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	result, err := r.db.Exec(ctx, "DELETE FROM asset_rules WHERE id = $1", id)
	duration := time.Since(start)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_delete", duration, false)
		return fmt.Errorf("deleting asset rule: %w", err)
	}
	if result.RowsAffected() == 0 {
		r.recorder.RecordDBQuery(ctx, "assetrule_delete", duration, true)
		return ErrNotFound
	}
	r.recorder.RecordDBQuery(ctx, "assetrule_delete", duration, true)
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	start := time.Now()

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM asset_rules").Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_list_count", time.Since(start), false)
		return nil, fmt.Errorf("counting asset rules: %w", err)
	}

	q := `
		SELECT id, name, description, links, rule_type, query_expression,
			metadata_field, pattern_type, pattern_value, priority, is_enabled,
			created_by, created_at, updated_at, membership_count,
			last_reconciled_at, reconciliation_hash
		FROM asset_rules
		ORDER BY name ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_list", time.Since(start), false)
		return nil, fmt.Errorf("listing asset rules: %w", err)
	}
	defer rows.Close()

	var rules []*AssetRule
	for rows.Next() {
		rule, err := r.scanRuleFromRows(rows)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_list", time.Since(start), false)
			return nil, fmt.Errorf("scanning asset rule: %w", err)
		}
		rule.TermIDs, _ = r.GetTermIDs(ctx, rule.ID)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_list", time.Since(start), false)
		return nil, fmt.Errorf("iterating asset rules: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_list", time.Since(start), true)
	return &ListResult{AssetRules: rules, Total: total}, nil
}

func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (*ListResult, error) {
	start := time.Now()

	args := []interface{}{}
	argCount := 1
	conditions := []string{}

	if filter.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(search_text @@ plainto_tsquery('english', $%d) OR name ILIKE $%d)", argCount, argCount+1))
		args = append(args, filter.Query, "%"+filter.Query+"%")
		argCount += 2
	}

	where := "WHERE 1=1"
	if len(conditions) > 0 {
		where = fmt.Sprintf("WHERE 1=1 AND %s", strings.Join(conditions, " AND "))
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM asset_rules %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_search_count", time.Since(start), false)
		return nil, fmt.Errorf("counting search results: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT id, name, description, links, rule_type, query_expression,
			metadata_field, pattern_type, pattern_value, priority, is_enabled,
			created_by, created_at, updated_at, membership_count,
			last_reconciled_at, reconciliation_hash
		FROM asset_rules
		%s
		ORDER BY name ASC
		LIMIT $%d OFFSET $%d`, where, argCount, argCount+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_search", time.Since(start), false)
		return nil, fmt.Errorf("searching asset rules: %w", err)
	}
	defer rows.Close()

	var rules []*AssetRule
	for rows.Next() {
		rule, err := r.scanRuleFromRows(rows)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_search", time.Since(start), false)
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		rule.TermIDs, _ = r.GetTermIDs(ctx, rule.ID)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_search", time.Since(start), false)
		return nil, fmt.Errorf("iterating search results: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_search", time.Since(start), true)
	return &ListResult{AssetRules: rules, Total: total}, nil
}

func (r *PostgresRepository) GetAllEnabled(ctx context.Context) ([]*AssetRule, error) {
	start := time.Now()

	q := `
		SELECT id, name, description, links, rule_type, query_expression,
			metadata_field, pattern_type, pattern_value, priority, is_enabled,
			created_by, created_at, updated_at, membership_count,
			last_reconciled_at, reconciliation_hash
		FROM asset_rules
		WHERE is_enabled = TRUE
		ORDER BY priority ASC, created_at ASC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_get_all_enabled", time.Since(start), false)
		return nil, fmt.Errorf("querying enabled asset rules: %w", err)
	}
	defer rows.Close()

	var rules []*AssetRule
	for rows.Next() {
		rule, err := r.scanRuleFromRows(rows)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_get_all_enabled", time.Since(start), false)
			return nil, fmt.Errorf("scanning asset rule: %w", err)
		}
		rule.TermIDs, _ = r.GetTermIDs(ctx, rule.ID)
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_get_all_enabled", time.Since(start), false)
		return nil, fmt.Errorf("iterating asset rules: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_get_all_enabled", time.Since(start), true)
	return rules, nil
}

func (r *PostgresRepository) UpdateReconciliationState(ctx context.Context, ruleID string, hash string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `
		UPDATE asset_rules
		SET last_reconciled_at = NOW(),
			reconciliation_hash = $2,
			membership_count = (SELECT COUNT(*) FROM asset_rule_memberships WHERE asset_rule_id = $1),
			memberships_updated_at = NOW()
		WHERE id = $1`, ruleID, hash)

	r.recorder.RecordDBQuery(ctx, "assetrule_update_reconciliation", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("updating reconciliation state: %w", err)
	}
	return nil
}

func (r *PostgresRepository) SetTerms(ctx context.Context, ruleID string, termIDs []string) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_set_terms", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM asset_rule_terms WHERE asset_rule_id = $1`, ruleID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_set_terms", time.Since(start), false)
		return fmt.Errorf("clearing terms: %w", err)
	}

	for _, termID := range termIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO asset_rule_terms (asset_rule_id, glossary_term_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, ruleID, termID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_set_terms", time.Since(start), false)
			return fmt.Errorf("inserting term: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_set_terms", time.Since(start), false)
		return fmt.Errorf("committing: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_set_terms", time.Since(start), true)
	return nil
}

func (r *PostgresRepository) GetTermIDs(ctx context.Context, ruleID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT glossary_term_id FROM asset_rule_terms WHERE asset_rule_id = $1 ORDER BY glossary_term_id`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("querying term IDs: %w", err)
	}
	defer rows.Close()

	var termIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning term ID: %w", err)
		}
		termIDs = append(termIDs, id)
	}
	return termIDs, rows.Err()
}

func (r *PostgresRepository) GetRuleManagedLinks(ctx context.Context, assetID string) ([]EnrichedExternalLink, error) {
	start := time.Now()

	q := `
		SELECT ar.id, ar.name, ar.links
		FROM asset_rules ar
		JOIN asset_rule_memberships arm ON ar.id = arm.asset_rule_id
		WHERE arm.asset_id = $1 AND ar.is_enabled = TRUE
		ORDER BY ar.priority ASC, ar.name ASC`

	rows, err := r.db.Query(ctx, q, assetID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_get_managed_links", time.Since(start), false)
		return nil, fmt.Errorf("querying rule-managed links: %w", err)
	}
	defer rows.Close()

	var result []EnrichedExternalLink
	for rows.Next() {
		var ruleID, ruleName string
		var linksJSON []byte

		if err := rows.Scan(&ruleID, &ruleName, &linksJSON); err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_get_managed_links", time.Since(start), false)
			return nil, fmt.Errorf("scanning rule links: %w", err)
		}

		var links []ExternalLink
		if err := json.Unmarshal(linksJSON, &links); err != nil {
			r.recorder.RecordDBQuery(ctx, "assetrule_get_managed_links", time.Since(start), false)
			return nil, fmt.Errorf("unmarshaling links: %w", err)
		}

		rID := ruleID
		rName := ruleName
		for _, link := range links {
			result = append(result, EnrichedExternalLink{
				ExternalLink: asset.ExternalLink{
					Name: link.Name,
					Icon: link.Icon,
					URL:  link.URL,
				},
				Source:   "rule",
				RuleID:   &rID,
				RuleName: &rName,
			})
		}
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "assetrule_get_managed_links", time.Since(start), false)
		return nil, fmt.Errorf("iterating rule links: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "assetrule_get_managed_links", time.Since(start), true)
	return result, nil
}

func (r *PostgresRepository) scanRule(row pgx.Row) (*AssetRule, error) {
	var rule AssetRule
	var linksJSON []byte

	err := row.Scan(
		&rule.ID, &rule.Name, &rule.Description, &linksJSON,
		&rule.RuleType, &rule.QueryExpression, &rule.MetadataField,
		&rule.PatternType, &rule.PatternValue, &rule.Priority, &rule.IsEnabled,
		&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt, &rule.MembershipCount,
		&rule.LastReconciledAt, &rule.ReconciliationHash,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(linksJSON, &rule.Links); err != nil {
		return nil, fmt.Errorf("unmarshaling links: %w", err)
	}

	return &rule, nil
}

func (r *PostgresRepository) scanRuleFromRows(rows pgx.Rows) (*AssetRule, error) {
	var rule AssetRule
	var linksJSON []byte

	err := rows.Scan(
		&rule.ID, &rule.Name, &rule.Description, &linksJSON,
		&rule.RuleType, &rule.QueryExpression, &rule.MetadataField,
		&rule.PatternType, &rule.PatternValue, &rule.Priority, &rule.IsEnabled,
		&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt, &rule.MembershipCount,
		&rule.LastReconciledAt, &rule.ReconciliationHash,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(linksJSON, &rule.Links); err != nil {
		return nil, fmt.Errorf("unmarshaling links: %w", err)
	}

	return &rule, nil
}
