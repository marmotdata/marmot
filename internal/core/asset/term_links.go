package asset

import (
	"context"
	"fmt"
	"strings"
)

// TermLinkManual names every link to a term that no asset rule made, whether a person, a plugin or a service account added it.
const TermLinkManual = "manual"

// TermLinkField names the links an asset has to a term through the fields of its schema rather than on the asset itself.
const TermLinkField = "field"

// TermLink says how one asset came to be linked to a term. An asset linked more than one way, such as manually and through a field, has one link for each way.
type TermLink struct {
	AssetID string `json:"asset_id"`
	// Source is "manual", "rule" or "field".
	Source   string  `json:"source"`
	RuleID   *string `json:"rule_id,omitempty"`
	RuleName *string `json:"rule_name,omitempty"`
	// Fields are the schema fields that carry the term, set when Source is "field".
	Fields []FieldTermLink `json:"fields,omitempty"`
} // @name AssetTermLink

// FieldTermLink is one schema field that carries a term.
type FieldTermLink struct {
	// Schema is the key of the schema the field belongs to, as in the asset's schema map.
	Schema string `json:"schema"`
	// Path is the field's dotted path, such as customer.email.
	Path string `json:"path"`
} // @name AssetTermLinkField

// TermLinkGroup counts a term's linked assets by how they were linked, so a page can show each group without fetching every asset.
type TermLinkGroup struct {
	Source   string  `json:"source"`
	RuleID   *string `json:"rule_id,omitempty"`
	RuleName *string `json:"rule_name,omitempty"`
	Count    int     `json:"count"`
} // @name AssetTermLinkGroup

// ValidTermLinkFilter reports whether link is a filter GetAssetsByTermLink accepts.
func ValidTermLinkFilter(link string) bool {
	_, _, err := termLinkAssets(link, 2)
	return err == nil
}

// termLinkAssets returns a query for the ids of the assets linked to the term in $1 one way. An empty filter matches every link, each asset once; "manual" matches links on the asset that no rule made; "rule:<id>" matches one rule's links; "field" matches assets with a field that carries the term.
func termLinkAssets(link string, arg int) (string, []interface{}, error) {
	switch {
	case link == "":
		return `SELECT at.asset_id FROM asset_terms at WHERE at.glossary_term_id = $1
			UNION SELECT ft.asset_id FROM asset_field_terms ft WHERE ft.glossary_term_id = $1`, nil, nil
	case link == TermLinkManual:
		return `SELECT at.asset_id FROM asset_terms at WHERE at.glossary_term_id = $1 AND at.source NOT LIKE 'rule:%'`, nil, nil
	case link == TermLinkField:
		return `SELECT DISTINCT ft.asset_id FROM asset_field_terms ft WHERE ft.glossary_term_id = $1`, nil, nil
	case strings.HasPrefix(link, "rule:") && len(link) > len("rule:"):
		return fmt.Sprintf(`SELECT at.asset_id FROM asset_terms at WHERE at.glossary_term_id = $1 AND at.source = $%d`, arg), []interface{}{link}, nil
	default:
		return "", nil, fmt.Errorf("unknown link filter %q", link)
	}
}

// GetAssetsByTermLink pages through a term's linked assets, optionally only those linked one way (see termLinkAssets).
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

	linked, extra, err := termLinkAssets(link, 2)
	if err != nil {
		return nil, 0, err
	}

	countArgs := append([]interface{}{termID}, extra...)
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM (`+linked+`) linked`, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting assets by term: %w", err)
	}

	limitArg := len(countArgs) + 1
	query := baseSelectAsset + `
		WHERE assets.id IN (` + linked + `)
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

// GetTermLinks says how each of the given assets is linked to a term: links on the asset first, then the fields that carry the term.
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating term links: %w", err)
	}

	fieldRows, err := r.db.Query(ctx, `
		SELECT asset_id, schema_name, field_path
		FROM asset_field_terms
		WHERE glossary_term_id = $1 AND asset_id = ANY($2)
		ORDER BY asset_id, schema_name, field_path`, termID, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("querying field term links: %w", err)
	}
	defer fieldRows.Close()

	byAsset := map[string]int{}
	for fieldRows.Next() {
		var assetID string
		var f FieldTermLink
		if err := fieldRows.Scan(&assetID, &f.Schema, &f.Path); err != nil {
			return nil, fmt.Errorf("scanning field term link: %w", err)
		}
		i, ok := byAsset[assetID]
		if !ok {
			i = len(links)
			byAsset[assetID] = i
			links = append(links, TermLink{AssetID: assetID, Source: TermLinkField})
		}
		links[i].Fields = append(links[i].Fields, f)
	}
	return links, fieldRows.Err()
}

// GetTermLinkGroups counts a term's linked assets by how they were linked: links no rule made first, then assets whose fields carry the term, then each rule by name. A rule that has since been deleted keeps its group under its id until reconciliation removes its links.
func (r *PostgresRepository) GetTermLinkGroups(ctx context.Context, termID string) ([]TermLinkGroup, error) {
	rows, err := r.db.Query(ctx, `
		SELECT kind, rule_id, rule_name, n FROM (
			SELECT
				CASE WHEN at.source LIKE 'rule:%' THEN 'rule' ELSE 'manual' END AS kind,
				CASE WHEN at.source LIKE 'rule:%' THEN substring(at.source FROM 6) END AS rule_id,
				MIN(ar.name) AS rule_name,
				COUNT(*) AS n
			FROM asset_terms at
			LEFT JOIN asset_rules ar ON at.source LIKE 'rule:%' AND ar.id::text = substring(at.source FROM 6)
			WHERE at.glossary_term_id = $1
			GROUP BY 1, 2
			UNION ALL
			SELECT 'field', NULL, NULL, COUNT(DISTINCT ft.asset_id)
			FROM asset_field_terms ft
			WHERE ft.glossary_term_id = $1
			HAVING COUNT(*) > 0
		) g
		ORDER BY CASE kind WHEN 'manual' THEN 0 WHEN 'field' THEN 1 ELSE 2 END, rule_name NULLS LAST, rule_id`, termID)
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
