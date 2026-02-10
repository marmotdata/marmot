package assetrule

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/enrichment"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/rs/zerolog/log"
)

type MembershipRepository interface {
	CreateMemberships(ctx context.Context, ruleID string, assetIDs []string) error
	DeleteMembershipsByAsset(ctx context.Context, assetID string) error
	DeleteMembershipsByRule(ctx context.Context, ruleID string) error
	DeleteMembershipsBatch(ctx context.Context, ruleID string, assetIDs []string) error
	GetMembershipAssetIDs(ctx context.Context, ruleID string, limit, offset int) ([]string, int, error)
	GetExistingMembershipAssetIDs(ctx context.Context, ruleID string) (map[string]struct{}, error)

	// Writes to asset_terms with source='rule:<id>'
	CreateTermMemberships(ctx context.Context, ruleID string, termIDs []string, assetIDs []string) error
	DeleteTermMembershipsByAsset(ctx context.Context, assetID string) error
	DeleteTermMembershipsByRule(ctx context.Context, ruleID string) error
	DeleteTermMembershipsBatch(ctx context.Context, ruleID string, assetIDs []string) error

	SaveRuleTargets(ctx context.Context, ruleID string, targets []enrichment.RuleTarget) error
	DeleteRuleTargets(ctx context.Context, ruleID string) error
	FindCandidateRules(ctx context.Context, sig enrichment.AssetSignature) ([]enrichment.CandidateRule, error)

	UpdateMembershipStats(ctx context.Context, ruleID string) error
}

// PostgresMembershipRepository implements MembershipRepository for PostgreSQL.
type PostgresMembershipRepository struct {
	db       *pgxpool.Pool
	recorder metrics.Recorder
}

// NewPostgresMembershipRepository creates a new membership repository.
func NewPostgresMembershipRepository(db *pgxpool.Pool, recorder metrics.Recorder) *PostgresMembershipRepository {
	return &PostgresMembershipRepository{db: db, recorder: recorder}
}

func ruleSource(ruleID string) string {
	return "rule:" + ruleID
}

func (r *PostgresMembershipRepository) CreateMemberships(ctx context.Context, ruleID string, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}

	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_create_batch", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, assetID := range assetIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO asset_rule_memberships (asset_rule_id, asset_id)
			VALUES ($1, $2)
			ON CONFLICT (asset_rule_id, asset_id) DO NOTHING`,
			ruleID, assetID)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "arm_create_batch", time.Since(start), false)
			return fmt.Errorf("inserting membership: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_create_batch", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "arm_create_batch", time.Since(start), true)

	if err := r.UpdateMembershipStats(ctx, ruleID); err != nil {
		log.Warn().Err(err).Str("rule_id", ruleID).Msg("Failed to update asset rule membership stats")
	}

	return nil
}

func (r *PostgresMembershipRepository) DeleteMembershipsByAsset(ctx context.Context, assetID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx, `DELETE FROM asset_rule_memberships WHERE asset_id = $1`, assetID)
	r.recorder.RecordDBQuery(ctx, "arm_delete_by_asset", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("deleting memberships by asset: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) DeleteMembershipsByRule(ctx context.Context, ruleID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx, `DELETE FROM asset_rule_memberships WHERE asset_rule_id = $1`, ruleID)
	r.recorder.RecordDBQuery(ctx, "arm_delete_by_rule", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("deleting memberships by rule: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) GetMembershipAssetIDs(ctx context.Context, ruleID string, limit, offset int) ([]string, int, error) {
	start := time.Now()

	var total int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM asset_rule_memberships WHERE asset_rule_id = $1`, ruleID).Scan(&total)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_get_assets_count", time.Since(start), false)
		return nil, 0, fmt.Errorf("counting memberships: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT asset_id FROM asset_rule_memberships
		WHERE asset_rule_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, ruleID, limit, offset)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_get_assets", time.Since(start), false)
		return nil, 0, fmt.Errorf("querying memberships: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.recorder.RecordDBQuery(ctx, "arm_get_assets", time.Since(start), false)
			return nil, 0, fmt.Errorf("scanning asset ID: %w", err)
		}
		assetIDs = append(assetIDs, id)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_get_assets", time.Since(start), false)
		return nil, 0, fmt.Errorf("iterating memberships: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "arm_get_assets", time.Since(start), true)
	return assetIDs, total, nil
}

func (r *PostgresMembershipRepository) GetExistingMembershipAssetIDs(ctx context.Context, ruleID string) (map[string]struct{}, error) {
	start := time.Now()

	rows, err := r.db.Query(ctx,
		`SELECT asset_id FROM asset_rule_memberships WHERE asset_rule_id = $1`, ruleID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_get_existing", time.Since(start), false)
		return nil, fmt.Errorf("querying existing memberships: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.recorder.RecordDBQuery(ctx, "arm_get_existing", time.Since(start), false)
			return nil, fmt.Errorf("scanning asset ID: %w", err)
		}
		existing[id] = struct{}{}
	}

	r.recorder.RecordDBQuery(ctx, "arm_get_existing", time.Since(start), true)
	return existing, rows.Err()
}

func (r *PostgresMembershipRepository) CreateTermMemberships(ctx context.Context, ruleID string, termIDs []string, assetIDs []string) error {
	if len(assetIDs) == 0 || len(termIDs) == 0 {
		return nil
	}

	start := time.Now()
	source := ruleSource(ruleID)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_create_terms_batch", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, assetID := range assetIDs {
		for _, termID := range termIDs {
			_, err := tx.Exec(ctx, `
				INSERT INTO asset_terms (asset_id, glossary_term_id, source)
				VALUES ($1, $2, $3)
				ON CONFLICT (asset_id, glossary_term_id) DO NOTHING`,
				assetID, termID, source)
			if err != nil {
				r.recorder.RecordDBQuery(ctx, "arm_create_terms_batch", time.Since(start), false)
				return fmt.Errorf("inserting asset term: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_create_terms_batch", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "arm_create_terms_batch", time.Since(start), true)
	return nil
}

func (r *PostgresMembershipRepository) DeleteTermMembershipsByAsset(ctx context.Context, assetID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx,
		`DELETE FROM asset_terms WHERE asset_id = $1 AND source LIKE 'rule:%'`, assetID)
	r.recorder.RecordDBQuery(ctx, "arm_delete_terms_by_asset", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("deleting term memberships by asset: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) DeleteTermMembershipsByRule(ctx context.Context, ruleID string) error {
	start := time.Now()
	source := ruleSource(ruleID)
	_, err := r.db.Exec(ctx,
		`DELETE FROM asset_terms WHERE source = $1`, source)
	r.recorder.RecordDBQuery(ctx, "arm_delete_terms_by_rule", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("deleting term memberships by rule: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) SaveRuleTargets(ctx context.Context, ruleID string, targets []enrichment.RuleTarget) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "argt_save", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM asset_rule_targets WHERE rule_id = $1`, ruleID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "argt_save", time.Since(start), false)
		return fmt.Errorf("deleting existing targets: %w", err)
	}

	for _, t := range targets {
		_, err := tx.Exec(ctx, `
			INSERT INTO asset_rule_targets (rule_id, target_type, target_value)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`,
			ruleID, t.TargetType, t.TargetValue)
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "argt_save", time.Since(start), false)
			return fmt.Errorf("inserting target: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "argt_save", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "argt_save", time.Since(start), true)
	return nil
}

func (r *PostgresMembershipRepository) DeleteRuleTargets(ctx context.Context, ruleID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx, `DELETE FROM asset_rule_targets WHERE rule_id = $1`, ruleID)
	r.recorder.RecordDBQuery(ctx, "argt_delete", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("deleting rule targets: %w", err)
	}
	return nil
}

func (r *PostgresMembershipRepository) FindCandidateRules(ctx context.Context, sig enrichment.AssetSignature) ([]enrichment.CandidateRule, error) {
	start := time.Now()

	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT t.rule_id
		FROM asset_rule_targets t
		JOIN asset_rules r ON t.rule_id = r.id
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
		r.recorder.RecordDBQuery(ctx, "argt_find_candidates", time.Since(start), false)
		return nil, fmt.Errorf("querying candidate rules: %w", err)
	}
	defer rows.Close()

	var candidates []enrichment.CandidateRule
	for rows.Next() {
		var c enrichment.CandidateRule
		if err := rows.Scan(&c.RuleID); err != nil {
			r.recorder.RecordDBQuery(ctx, "argt_find_candidates", time.Since(start), false)
			return nil, fmt.Errorf("scanning candidate: %w", err)
		}
		candidates = append(candidates, c)
	}

	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "argt_find_candidates", time.Since(start), false)
		return nil, fmt.Errorf("iterating candidates: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "argt_find_candidates", time.Since(start), true)
	return candidates, nil
}

func (r *PostgresMembershipRepository) DeleteMembershipsBatch(ctx context.Context, ruleID string, assetIDs []string) error {
	start := time.Now()
	const batchSize = 5000
	for i := 0; i < len(assetIDs); i += batchSize {
		end := min(i+batchSize, len(assetIDs))
		_, err := r.db.Exec(ctx, `
			DELETE FROM asset_rule_memberships
			WHERE asset_rule_id = $1 AND asset_id = ANY($2)`,
			ruleID, assetIDs[i:end])
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "arm_delete_batch", time.Since(start), false)
			return fmt.Errorf("deleting memberships batch: %w", err)
		}
	}
	r.recorder.RecordDBQuery(ctx, "arm_delete_batch", time.Since(start), true)
	return nil
}

func (r *PostgresMembershipRepository) DeleteTermMembershipsBatch(ctx context.Context, ruleID string, assetIDs []string) error {
	start := time.Now()
	source := ruleSource(ruleID)
	const batchSize = 5000
	for i := 0; i < len(assetIDs); i += batchSize {
		end := min(i+batchSize, len(assetIDs))
		_, err := r.db.Exec(ctx, `
			DELETE FROM asset_terms
			WHERE source = $1 AND asset_id = ANY($2)`,
			source, assetIDs[i:end])
		if err != nil {
			r.recorder.RecordDBQuery(ctx, "arm_delete_terms_batch", time.Since(start), false)
			return fmt.Errorf("deleting term memberships batch: %w", err)
		}
	}
	r.recorder.RecordDBQuery(ctx, "arm_delete_terms_batch", time.Since(start), true)
	return nil
}

func (r *PostgresMembershipRepository) UpdateMembershipStats(ctx context.Context, ruleID string) error {
	start := time.Now()

	_, err := r.db.Exec(ctx, `
		UPDATE asset_rules
		SET membership_count = (
			SELECT COUNT(*) FROM asset_rule_memberships WHERE asset_rule_id = $1
		),
		memberships_updated_at = NOW()
		WHERE id = $1`, ruleID)

	r.recorder.RecordDBQuery(ctx, "arm_update_stats", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("updating membership stats: %w", err)
	}
	return nil
}
