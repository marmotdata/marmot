package asset

import (
	"context"
	"fmt"
	"strings"
)

// TermLinkManual names every link to a term that no asset rule made, whether a person, a plugin or a service account added it.
const TermLinkManual = "manual"

// TermLink says how one asset came to be linked to a term.
type TermLink struct {
	AssetID string `json:"asset_id"`
	// Source is "manual" or "rule".
	Source   string  `json:"source"`
	RuleID   *string `json:"rule_id,omitempty"`
	RuleName *string `json:"rule_name,omitempty"`
} // @name AssetTermLink

// TermLinkGroup counts a term's linked assets by how they were linked, so a page can show each group without fetching every asset.
type TermLinkGroup struct {
	Source   string  `json:"source"`
	RuleID   *string `json:"rule_id,omitempty"`
	RuleName *string `json:"rule_name,omitempty"`
	Count    int     `json:"count"`
} // @name AssetTermLinkGroup

// termLinkCondition turns a link filter into SQL over asset_terms aliased as at. An empty filter matches every link; "manual" matches links no rule made; "rule:<id>" matches one rule's links.
func termLinkCondition(link string, arg int) (string, []interface{}, error) {
	switch {
	case link == "":
		return "", nil, nil
	case link == TermLinkManual:
		return " AND at.source NOT LIKE 'rule:%'", nil, nil
	case strings.HasPrefix(link, "rule:") && len(link) > len("rule:"):
		return fmt.Sprintf(" AND at.source = $%d", arg), []interface{}{link}, nil
	default:
		return "", nil, fmt.Errorf("unknown link filter %q", link)
	}
}

// GetAssetsByTermLink pages through a term's linked assets, optionally only those linked one way (see termLinkCondition).
func (r *PostgresRepository) GetAssetsByTermLink(ctx context.Context, termID, link string, limit, offset int) ([]*Asset, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	cond, extra, err := termLinkCondition(link, 2)
	if err != nil {
		return nil, 0, err
	}

	countArgs := append([]interface{}{termID}, extra...)
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM asset_terms at WHERE at.glossary_term_id = $1`+cond, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting assets by term: %w", err)
	}

	limitArg := len(countArgs) + 1
	query := baseSelectAsset + `
		WHERE assets.id IN (SELECT at.asset_id FROM asset_terms at WHERE at.glossary_term_id = $1` + cond + `)
		ORDER BY assets.name ASC
		LIMIT $` + fmt.Sprint(limitArg) + ` OFFSET $` + fmt.Sprint(limitArg+1)

	rows, err := r.db.Query(ctx, query, append(countArgs, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying assets by term: %w", err)
	}
	defer rows.Close()

	assets := []*Asset{}
	for rows.Next() {
		a, err := r.scanAsset(ctx, rows)
		if err != nil {
			return nil, 0, err
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating assets: %w", err)
	}

	return assets, total, nil
}

// GetTermLinks says how each of the given assets is linked to a term.
func (r *PostgresRepository) GetTermLinks(ctx context.Context, termID string, assetIDs []string) ([]TermLink, error) {
	if len(assetIDs) == 0 {
		return []TermLink{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT at.asset_id,
			CASE WHEN at.source LIKE 'rule:%' THEN 'rule' ELSE 'manual' END,
			ar.id::text, ar.name
		FROM asset_terms at
		LEFT JOIN asset_rules ar ON at.source LIKE 'rule:%' AND ar.id::text = substring(at.source FROM 6)
		WHERE at.glossary_term_id = $1 AND at.asset_id = ANY($2)`, termID, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("querying term links: %w", err)
	}
	defer rows.Close()

	links := []TermLink{}
	for rows.Next() {
		var l TermLink
		if err := rows.Scan(&l.AssetID, &l.Source, &l.RuleID, &l.RuleName); err != nil {
			return nil, fmt.Errorf("scanning term link: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetTermLinkGroups counts a term's linked assets by how they were linked: links no rule made first, then each rule by name. A rule that has since been deleted keeps its group under its id until reconciliation removes its links.
func (r *PostgresRepository) GetTermLinkGroups(ctx context.Context, termID string) ([]TermLinkGroup, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			CASE WHEN at.source LIKE 'rule:%' THEN 'rule' ELSE 'manual' END AS kind,
			CASE WHEN at.source LIKE 'rule:%' THEN substring(at.source FROM 6) END AS rule_id,
			MIN(ar.name) AS rule_name,
			COUNT(*)
		FROM asset_terms at
		LEFT JOIN asset_rules ar ON at.source LIKE 'rule:%' AND ar.id::text = substring(at.source FROM 6)
		WHERE at.glossary_term_id = $1
		GROUP BY 1, 2
		ORDER BY 1, 3 NULLS LAST, 2`, termID)
	if err != nil {
		return nil, fmt.Errorf("grouping term links: %w", err)
	}
	defer rows.Close()

	groups := []TermLinkGroup{}
	for rows.Next() {
		var g TermLinkGroup
		if err := rows.Scan(&g.Source, &g.RuleID, &g.RuleName, &g.Count); err != nil {
			return nil, fmt.Errorf("scanning term link group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *service) GetAssetsByTermLink(ctx context.Context, termID, link string, limit, offset int) ([]*Asset, int, error) {
	assets, total, err := s.repo.GetAssetsByTermLink(ctx, termID, link, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("getting assets by term link: %w", err)
	}
	return assets, total, nil
}

func (s *service) GetTermLinks(ctx context.Context, termID string, assetIDs []string) ([]TermLink, error) {
	return s.repo.GetTermLinks(ctx, termID, assetIDs)
}

func (s *service) GetTermLinkGroups(ctx context.Context, termID string) ([]TermLinkGroup, error) {
	return s.repo.GetTermLinkGroups(ctx, termID)
}
