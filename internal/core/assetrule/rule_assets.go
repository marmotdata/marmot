package assetrule

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RuleAsset is an asset a rule applies to, with what a list of them needs to show.
type RuleAsset struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	MRN       string   `json:"mrn"`
	Type      string   `json:"type"`
	Providers []string `json:"providers"`
} // @name AssetRuleAsset

// SearchMembershipAssets pages through a rule's matched assets by name, optionally only those whose name or MRN contains query.
func (r *PostgresMembershipRepository) SearchMembershipAssets(ctx context.Context, ruleID, query string, limit, offset int) ([]RuleAsset, int, error) {
	start := time.Now()

	args := []interface{}{ruleID}
	cond := ""
	if q := strings.TrimSpace(query); q != "" {
		args = append(args, "%"+q+"%")
		cond = " AND (a.name ILIKE $2 OR a.mrn ILIKE $2)"
	}
	from := ` FROM asset_rule_memberships m JOIN assets a ON a.id = m.asset_id WHERE m.asset_rule_id = $1` + cond

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_search_assets", time.Since(start), false)
		return nil, 0, fmt.Errorf("counting matched assets: %w", err)
	}

	n := len(args)
	rows, err := r.db.Query(ctx,
		`SELECT a.id, a.name, a.mrn, a.type, a.providers`+from+fmt.Sprintf(` ORDER BY a.name ASC, a.id ASC LIMIT $%d OFFSET $%d`, n+1, n+2),
		append(args, limit, offset)...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_search_assets", time.Since(start), false)
		return nil, 0, fmt.Errorf("querying matched assets: %w", err)
	}
	defer rows.Close()

	assets := []RuleAsset{}
	for rows.Next() {
		var a RuleAsset
		if err := rows.Scan(&a.ID, &a.Name, &a.MRN, &a.Type, &a.Providers); err != nil {
			r.recorder.RecordDBQuery(ctx, "arm_search_assets", time.Since(start), false)
			return nil, 0, fmt.Errorf("scanning matched asset: %w", err)
		}
		if a.Providers == nil {
			a.Providers = []string{}
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "arm_search_assets", time.Since(start), false)
		return nil, 0, fmt.Errorf("iterating matched assets: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "arm_search_assets", time.Since(start), true)
	return assets, total, nil
}
