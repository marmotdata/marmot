package search

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// entityOwnersSQL lists every owner of an asset, glossary term or data product as (entity_type, entity_id, owner_id, owner_kind, owner_name), with entity_type matching search_index.type.
const entityOwnersSQL = `
	SELECT 'asset' AS entity_type, ao.asset_id AS entity_id,
	       COALESCE(ao.user_id, ao.team_id)::text AS owner_id,
	       CASE WHEN ao.user_id IS NOT NULL THEN 'user' ELSE 'team' END AS owner_kind,
	       COALESCE(NULLIF(u.name, ''), u.username, t.name, '') AS owner_name
	FROM asset_owners ao
	LEFT JOIN users u ON u.id = ao.user_id
	LEFT JOIN teams t ON t.id = ao.team_id
	UNION ALL
	SELECT 'glossary', o.glossary_term_id::text,
	       COALESCE(o.user_id, o.team_id)::text,
	       CASE WHEN o.user_id IS NOT NULL THEN 'user' ELSE 'team' END,
	       COALESCE(NULLIF(u.name, ''), u.username, t.name, '')
	FROM glossary_term_owners o
	LEFT JOIN users u ON u.id = o.user_id
	LEFT JOIN teams t ON t.id = o.team_id
	UNION ALL
	SELECT 'data_product', o.data_product_id::text,
	       COALESCE(o.user_id, o.team_id)::text,
	       CASE WHEN o.user_id IS NOT NULL THEN 'user' ELSE 'team' END,
	       COALESCE(NULLIF(u.name, ''), u.username, t.name, '')
	FROM data_product_owners o
	LEFT JOIN users u ON u.id = o.user_id
	LEFT JOIN teams t ON t.id = o.team_id`

// ownerFilterCondition matches rows owned by any of the user or team IDs in the given parameter. Each kind checks its own owners table so the lookups stay on their indexes.
func ownerFilterCondition(param int) string {
	return fmt.Sprintf(`(
		(type = 'asset' AND EXISTS (SELECT 1 FROM asset_owners ao WHERE ao.asset_id = search_index.entity_id AND COALESCE(ao.user_id, ao.team_id)::text = ANY($%[1]d)))
		OR (type = 'glossary' AND EXISTS (SELECT 1 FROM glossary_term_owners o WHERE o.glossary_term_id::text = search_index.entity_id AND COALESCE(o.user_id, o.team_id)::text = ANY($%[1]d)))
		OR (type = 'data_product' AND EXISTS (SELECT 1 FROM data_product_owners o WHERE o.data_product_id::text = search_index.entity_id AND COALESCE(o.user_id, o.team_id)::text = ANY($%[1]d)))
	)`, param)
}

// EnrichResults adds owners, counts and asset signals to a page of results. Each detail is best effort: a failure is logged and leaves that detail out rather than failing the search.
func (r *PostgresRepository) EnrichResults(ctx context.Context, results []*Result) {
	if len(results) == 0 || r.db == nil {
		return
	}
	ids := map[ResultType][]string{}
	for _, res := range results {
		ids[res.Type] = append(ids[res.Type], res.ID)
	}

	if err := r.enrichOwners(ctx, results, ids); err != nil {
		log.Warn().Err(err).Msg("Failed to add owners to search results")
	}
	if assetIDs := ids[ResultTypeAsset]; len(assetIDs) > 0 {
		r.enrichAssets(ctx, results, assetIDs)
	}
	if err := r.enrichCounts(ctx, results, ids); err != nil {
		log.Warn().Err(err).Msg("Failed to add counts to search results")
	}
}

func (r *PostgresRepository) enrichOwners(ctx context.Context, results []*Result, ids map[ResultType][]string) error {
	var types, entityIDs []string
	for _, t := range []ResultType{ResultTypeAsset, ResultTypeGlossary, ResultTypeDataProduct} {
		for _, id := range ids[t] {
			types = append(types, string(t))
			entityIDs = append(entityIDs, id)
		}
	}
	if len(entityIDs) == 0 {
		return nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT o.entity_type, o.entity_id, o.owner_id, o.owner_kind, o.owner_name
		FROM (`+entityOwnersSQL+`) o
		JOIN unnest($1::text[], $2::text[]) AS want(entity_type, entity_id)
		  ON want.entity_type = o.entity_type AND want.entity_id = o.entity_id
		ORDER BY o.owner_kind DESC, o.owner_name`, types, entityIDs)
	if err != nil {
		return fmt.Errorf("querying owners: %w", err)
	}
	defer rows.Close()
	owners := map[string][]ResultOwner{}
	for rows.Next() {
		var entityType, entityID string
		var o ResultOwner
		if err := rows.Scan(&entityType, &entityID, &o.ID, &o.Kind, &o.Name); err != nil {
			return fmt.Errorf("scanning owner: %w", err)
		}
		key := entityType + "/" + entityID
		owners[key] = append(owners[key], o)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, res := range results {
		if o := owners[string(res.Type)+"/"+res.ID]; len(o) > 0 {
			res.Owners = o
		}
	}
	return nil
}

func (r *PostgresRepository) enrichAssets(ctx context.Context, results []*Result, assetIDs []string) {
	popularity, err := r.signals.Popularity(ctx, assetIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to add popularity to search results")
	}
	stale, err := r.signals.Stale(ctx, assetIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to add staleness to search results")
	}
	sizes, err := r.signals.Sizes(ctx, assetIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to add sizes to search results")
	}
	for _, res := range results {
		if res.Type != ResultTypeAsset {
			continue
		}
		if level, ok := popularity[res.ID]; ok {
			res.Popularity = &level
		}
		if s, ok := stale[res.ID]; ok {
			res.Stale = &s
		}
		if size, ok := sizes[res.ID]; ok {
			res.RowCount = size.Rows
			res.ColumnCount = size.Columns
		}
	}
}

// enrichCounts adds linked asset counts to glossary terms, asset counts to data products and member and asset counts to teams, in one round trip.
func (r *PostgresRepository) enrichCounts(ctx context.Context, results []*Result, ids map[ResultType][]string) error {
	terms, products, teams := ids[ResultTypeGlossary], ids[ResultTypeDataProduct], ids[ResultTypeTeam]
	if len(terms)+len(products)+len(teams) == 0 {
		return nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT 'glossary', at.glossary_term_id::text, COUNT(*), 0
		FROM asset_terms at
		WHERE at.glossary_term_id::text = ANY($1)
		GROUP BY at.glossary_term_id
		UNION ALL
		SELECT 'data_product', m.data_product_id::text, COUNT(*), 0
		FROM data_product_memberships m
		WHERE m.data_product_id::text = ANY($2)
		GROUP BY m.data_product_id
		UNION ALL
		SELECT 'team', t.id::text,
		       (SELECT COUNT(*) FROM asset_owners ao WHERE ao.team_id = t.id),
		       (SELECT COUNT(*) FROM team_members tm WHERE tm.team_id = t.id)
		FROM teams t
		WHERE t.id::text = ANY($3)`, terms, products, teams)
	if err != nil {
		return fmt.Errorf("querying counts: %w", err)
	}
	defer rows.Close()
	type counts struct{ assets, members int }
	found := map[string]counts{}
	for rows.Next() {
		var kind, id string
		var c counts
		if err := rows.Scan(&kind, &id, &c.assets, &c.members); err != nil {
			return fmt.Errorf("scanning counts: %w", err)
		}
		found[kind+"/"+id] = c
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, res := range results {
		c := found[string(res.Type)+"/"+res.ID]
		assets, members := c.assets, c.members
		switch res.Type {
		case ResultTypeGlossary:
			res.LinkedAssetCount = &assets
		case ResultTypeDataProduct:
			res.AssetCount = &assets
		case ResultTypeTeam:
			res.AssetCount = &assets
			res.MemberCount = &members
		}
	}
	return nil
}
