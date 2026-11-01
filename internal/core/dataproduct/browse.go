package dataproduct

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/assetsignal"
)

// Sort orders for ListFiltered and GetMembers.
const (
	SortName    = "name"
	SortUpdated = "updated"
	SortPopular = "popular"
)

// Values for ListFilter.Mine.
const (
	MineOnly    = "only"
	MineExclude = "exclude"
)

// ListFilter narrows and orders the data product list on the server, so a client never has to read the whole catalog to search or filter it.
type ListFilter struct {
	Query string
	// OwnerIDs keeps products owned by any of these users or teams.
	OwnerIDs []string
	// Tags keeps products carrying any of these tags.
	Tags []string
	// Mine is MineOnly for the products UserID or one of their teams owns, MineExclude for the rest, or empty for all.
	Mine   string
	UserID string
	Sort   string
	Limit  int
	Offset int
}

// TypeCount is how many of a product's assets are of one type.
type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
} // @name DataProductTypeCount

// OwnerFacet is an owner that owns at least one data product.
type OwnerFacet struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
} // @name DataProductOwnerFacet

// TagFacet is a tag carried by at least one data product.
type TagFacet struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
} // @name DataProductTagFacet

// Facets lists the values the product list can be filtered by.
type Facets struct {
	Owners []OwnerFacet `json:"owners"`
	Tags   []TagFacet   `json:"tags"`
} // @name DataProductFacets

// MemberFilter narrows a product's assets.
type MemberFilter struct {
	// Source is SourceManual or SourceRule, or empty for both.
	Source string
	RuleID string
	// Query matches the asset name or MRN.
	Query  string
	Type   string
	Sort   string
	Limit  int
	Offset int
}

// Member is one asset in a data product with how it got there, so a page can group assets by rule without asking each rule for a preview.
type Member struct {
	AssetID     string   `json:"asset_id"`
	Name        string   `json:"name"`
	MRN         string   `json:"mrn"`
	Type        string   `json:"type"`
	Providers   []string `json:"providers"`
	Description *string  `json:"description,omitempty"`
	OwnerNames  []string `json:"owner_names"`
	Source      string   `json:"source"`
	RuleID      *string  `json:"rule_id,omitempty"`
	RuleName    *string  `json:"rule_name,omitempty"`
	// Popularity is the asset's level from 0 to 4, as assetsignal defines it.
	Popularity int       `json:"popularity"`
	AddedAt    time.Time `json:"added_at"`
} // @name DataProductMember

// MembersResult is a page of a product's assets.
type MembersResult struct {
	Members []Member `json:"members"`
	Total   int      `json:"total"`
} // @name DataProductMembersResult

// productViewsSQL sums each product's asset views over the popularity window, so products rank and score the way their assets do.
const productViewsSQL = `SELECT m.data_product_id, SUM(v.views)::bigint AS views
	FROM data_product_memberships m
	JOIN (` + assetsignal.ViewsSQL + `) v ON v.asset_id = m.asset_id
	GROUP BY m.data_product_id`

const productColumns = `id, name, description, metadata, tags, created_by, created_at, updated_at`

// ListFiltered returns a filtered, sorted page of data products with their popularity.
func (r *PostgresRepository) ListFiltered(ctx context.Context, f ListFilter) (*ListResult, error) {
	start := time.Now()

	args := []interface{}{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	conditions := []string{"TRUE"}
	if q := strings.TrimSpace(f.Query); q != "" {
		conditions = append(conditions, fmt.Sprintf("(dp.search_text @@ plainto_tsquery('english', %s) OR dp.name ILIKE %s OR array_to_string(dp.tags, ' ') ILIKE %s)",
			arg(q), arg("%"+q+"%"), arg("%"+q+"%")))
	}
	if len(f.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("dp.tags && %s", arg(f.Tags)))
	}
	if len(f.OwnerIDs) > 0 {
		p := arg(f.OwnerIDs)
		conditions = append(conditions, fmt.Sprintf("dp.id IN (SELECT data_product_id FROM data_product_owners WHERE user_id::text = ANY(%s) OR team_id::text = ANY(%s))", p, p))
	}
	if f.Mine != "" && f.UserID != "" {
		u := arg(f.UserID)
		mine := fmt.Sprintf(`dp.id IN (SELECT data_product_id FROM data_product_owners
			WHERE user_id = %s OR team_id IN (SELECT team_id FROM team_members WHERE user_id = %s))`, u, u)
		if f.Mine == MineExclude {
			mine = "NOT " + mine
		}
		conditions = append(conditions, mine)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM data_products dp WHERE "+where, args...).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list_filtered_count", time.Since(start), false)
		return nil, fmt.Errorf("counting data products: %w", err)
	}

	join := ""
	order := "dp.name ASC"
	switch f.Sort {
	case SortUpdated:
		order = "dp.updated_at DESC, dp.name ASC"
	case SortPopular:
		join = " LEFT JOIN (" + productViewsSQL + ") pv ON pv.data_product_id = dp.id"
		order = "COALESCE(pv.views, 0) DESC, dp.name ASC"
	}

	cols := "dp." + strings.ReplaceAll(productColumns, ", ", ", dp.")
	q := fmt.Sprintf("SELECT %s FROM data_products dp%s WHERE %s ORDER BY %s LIMIT %s OFFSET %s",
		cols, join, where, order, arg(f.Limit), arg(f.Offset))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list_filtered", time.Since(start), false)
		return nil, fmt.Errorf("listing data products: %w", err)
	}
	products := []*DataProduct{}
	for rows.Next() {
		var dp DataProduct
		var metadataJSON []byte
		if err := rows.Scan(&dp.ID, &dp.Name, &dp.Description, &metadataJSON, &dp.Tags, &dp.CreatedBy, &dp.CreatedAt, &dp.UpdatedAt); err != nil {
			rows.Close()
			r.recorder.RecordDBQuery(ctx, "dataproduct_list_filtered", time.Since(start), false)
			return nil, fmt.Errorf("scanning data product: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &dp.Metadata); err != nil {
			rows.Close()
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
		products = append(products, &dp)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_list_filtered", time.Since(start), false)
		return nil, fmt.Errorf("iterating data products: %w", err)
	}

	for _, dp := range products {
		if dp.Owners, err = r.loadOwners(ctx, dp.ID); err != nil {
			return nil, fmt.Errorf("loading owners for %s: %w", dp.ID, err)
		}
		dp.ManualAssetCount, dp.RuleAssetCount, _ = r.getAssetCounts(ctx, dp.ID)
		dp.AssetCount = dp.ManualAssetCount + dp.RuleAssetCount
		if iconMeta, err := r.GetProductImageMeta(ctx, dp.ID, ImagePurposeIcon); err == nil {
			dp.IconURL = &iconMeta.URL
		}
	}

	if err := r.FillPopularity(ctx, products); err != nil {
		return nil, err
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_list_filtered", time.Since(start), true)
	return &ListResult{DataProducts: products, Total: total}, nil
}

// FillPopularity sets each product's popularity from its assets' views, scaled against the most viewed product the way an asset's is scaled against the most viewed asset.
func (r *PostgresRepository) FillPopularity(ctx context.Context, products []*DataProduct) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]string, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}

	var busiest int64
	views := map[string]int64{}
	rows, err := r.db.Query(ctx, `
		SELECT data_product_id::text, views, MAX(views) OVER ()
		FROM (`+productViewsSQL+`) pv`)
	if err != nil {
		return fmt.Errorf("counting product views: %w", err)
	}
	defer rows.Close()
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	for rows.Next() {
		var id string
		var n, max int64
		if err := rows.Scan(&id, &n, &max); err != nil {
			return fmt.Errorf("scanning product views: %w", err)
		}
		busiest = max
		if _, ok := want[id]; ok {
			views[id] = n
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating product views: %w", err)
	}

	for _, p := range products {
		p.Popularity = assetsignal.PopularityLevel(views[p.ID], busiest)
	}
	return nil
}

// GetAssetTypes counts a product's assets by type, most common first.
func (r *PostgresRepository) GetAssetTypes(ctx context.Context, dataProductID string) ([]TypeCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.type, COUNT(*)
		FROM data_product_memberships m
		JOIN assets a ON a.id = m.asset_id
		WHERE m.data_product_id = $1
		GROUP BY a.type
		ORDER BY 2 DESC, 1 ASC`, dataProductID)
	if err != nil {
		return nil, fmt.Errorf("counting asset types: %w", err)
	}
	defer rows.Close()
	out := []TypeCount{}
	for rows.Next() {
		var t TypeCount
		if err := rows.Scan(&t.Type, &t.Count); err != nil {
			return nil, fmt.Errorf("scanning asset type: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetFacets lists every owner and tag in use on data products, for the list's filter menus.
func (r *PostgresRepository) GetFacets(ctx context.Context) (*Facets, error) {
	facets := &Facets{Owners: []OwnerFacet{}, Tags: []TagFacet{}}

	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(u.id::text, t.id::text), COALESCE(u.name, t.name),
			CASE WHEN u.id IS NOT NULL THEN 'user' ELSE 'team' END, COUNT(DISTINCT dpo.data_product_id)
		FROM data_product_owners dpo
		LEFT JOIN users u ON dpo.user_id = u.id
		LEFT JOIN teams t ON dpo.team_id = t.id
		WHERE u.id IS NOT NULL OR t.id IS NOT NULL
		GROUP BY 1, 2, 3
		ORDER BY 2 ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing product owners: %w", err)
	}
	for rows.Next() {
		var o OwnerFacet
		if err := rows.Scan(&o.ID, &o.Name, &o.Type, &o.Count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning product owner: %w", err)
		}
		facets.Owners = append(facets.Owners, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating product owners: %w", err)
	}

	rows, err = r.db.Query(ctx, `
		SELECT tag, COUNT(*) FROM data_products, unnest(tags) AS tag
		GROUP BY tag ORDER BY tag ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing product tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t TagFacet
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, fmt.Errorf("scanning product tag: %w", err)
		}
		facets.Tags = append(facets.Tags, t)
	}
	return facets, rows.Err()
}

// GetMembers pages through a product's assets with what the asset page needs to list them, in one query rather than one request per asset.
func (r *PostgresRepository) GetMembers(ctx context.Context, dataProductID string, f MemberFilter) (*MembersResult, error) {
	start := time.Now()

	args := []interface{}{dataProductID}
	arg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	conditions := []string{"m.data_product_id = $1"}
	if f.Source != "" {
		conditions = append(conditions, "m.source = "+arg(f.Source))
	}
	if f.RuleID != "" {
		conditions = append(conditions, "m.rule_id::text = "+arg(f.RuleID))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		p := arg("%" + q + "%")
		conditions = append(conditions, fmt.Sprintf("(a.name ILIKE %s OR a.mrn ILIKE %s)", p, p))
	}
	if f.Type != "" {
		conditions = append(conditions, "a.type = "+arg(f.Type))
	}
	where := strings.Join(conditions, " AND ")

	from := `FROM data_product_memberships m
		JOIN assets a ON a.id = m.asset_id
		LEFT JOIN data_product_rules r ON r.id = m.rule_id`

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) "+from+" WHERE "+where, args...).Scan(&total); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_members_count", time.Since(start), false)
		return nil, fmt.Errorf("counting members: %w", err)
	}

	order := "a.name ASC"
	if f.Sort == SortPopular {
		from += " LEFT JOIN (" + assetsignal.ViewsSQL + ") v ON v.asset_id = a.id"
		order = "COALESCE(v.views, 0) DESC, a.name ASC"
	}

	q := `SELECT a.id, a.name, a.mrn, a.type, a.providers,
			COALESCE(NULLIF(a.user_description, ''), a.description),
			ARRAY(SELECT COALESCE(u.name, t.name) FROM asset_owners ao
				LEFT JOIN users u ON u.id = ao.user_id
				LEFT JOIN teams t ON t.id = ao.team_id
				WHERE ao.asset_id = a.id AND COALESCE(u.name, t.name) IS NOT NULL
				ORDER BY ao.created_at),
			m.source, m.rule_id::text, r.name, m.created_at
		` + from + ` WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ` + arg(f.Limit) + ` OFFSET ` + arg(f.Offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_members", time.Since(start), false)
		return nil, fmt.Errorf("listing members: %w", err)
	}
	defer rows.Close()

	members := []Member{}
	for rows.Next() {
		var mb Member
		if err := rows.Scan(&mb.AssetID, &mb.Name, &mb.MRN, &mb.Type, &mb.Providers, &mb.Description,
			&mb.OwnerNames, &mb.Source, &mb.RuleID, &mb.RuleName, &mb.AddedAt); err != nil {
			r.recorder.RecordDBQuery(ctx, "dataproduct_members", time.Since(start), false)
			return nil, fmt.Errorf("scanning member: %w", err)
		}
		if mb.Providers == nil {
			mb.Providers = []string{}
		}
		if mb.OwnerNames == nil {
			mb.OwnerNames = []string{}
		}
		members = append(members, mb)
	}
	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_members", time.Since(start), false)
		return nil, fmt.Errorf("iterating members: %w", err)
	}

	if len(members) > 0 {
		ids := make([]string, len(members))
		for i, mb := range members {
			ids[i] = mb.AssetID
		}
		levels, err := assetsignal.New(r.db).Popularity(ctx, ids)
		if err != nil {
			return nil, err
		}
		for i := range members {
			members[i].Popularity = levels[members[i].AssetID]
		}
	}

	r.recorder.RecordDBQuery(ctx, "dataproduct_members", time.Since(start), true)
	return &MembersResult{Members: members, Total: total}, nil
}

// ReorderRules stores the order people put a product's rules in. ruleIDs must name every rule of the product exactly once.
func (r *PostgresRepository) ReorderRules(ctx context.Context, dataProductID string, ruleIDs []string) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var existing []string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(id::text), '{}') FROM data_product_rules WHERE data_product_id = $1`, dataProductID).Scan(&existing); err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}
	if len(existing) != len(ruleIDs) {
		return fmt.Errorf("%w: rule_ids must list every rule of the data product once", ErrInvalidInput)
	}
	known := make(map[string]bool, len(existing))
	for _, id := range existing {
		known[id] = true
	}
	for _, id := range ruleIDs {
		if !known[id] {
			return fmt.Errorf("%w: rule_ids must list every rule of the data product once", ErrInvalidInput)
		}
		delete(known, id)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE data_product_rules r
		SET position = o.ord - 1
		FROM unnest($2::uuid[]) WITH ORDINALITY AS o(id, ord)
		WHERE r.id = o.id AND r.data_product_id = $1`, dataProductID, ruleIDs); err != nil {
		r.recorder.RecordDBQuery(ctx, "dataproduct_reorder_rules", time.Since(start), false)
		return fmt.Errorf("reordering rules: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	r.recorder.RecordDBQuery(ctx, "dataproduct_reorder_rules", time.Since(start), true)
	return nil
}
