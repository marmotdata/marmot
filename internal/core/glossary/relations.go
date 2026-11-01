package glossary

import (
	"context"
	"fmt"
	"time"
)

// GetRelatedTerms returns the live terms related to a term, by name.
func (r *PostgresRepository) GetRelatedTerms(ctx context.Context, termID string) ([]RelatedTerm, error) {
	start := time.Now()

	rows, err := r.db.Query(ctx, `
		SELECT gt.id, gt.name
		FROM glossary_term_relations rel
		JOIN glossary_terms gt ON gt.id = rel.related_term_id
		WHERE rel.term_id = $1 AND gt.deleted_at IS NULL
		ORDER BY gt.name ASC`, termID)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_get_related", time.Since(start), false)
		return nil, fmt.Errorf("getting related terms: %w", err)
	}
	defer rows.Close()

	related := []RelatedTerm{}
	for rows.Next() {
		var t RelatedTerm
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_get_related", time.Since(start), false)
			return nil, fmt.Errorf("scanning related term: %w", err)
		}
		related = append(related, t)
	}
	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_get_related", time.Since(start), false)
		return nil, fmt.Errorf("iterating related terms: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_get_related", time.Since(start), true)
	return related, nil
}

// SetRelatedTerms replaces a term's related terms. Links are stored in both directions, so the other side of each link changes too.
func (r *PostgresRepository) SetRelatedTerms(ctx context.Context, termID string, relatedIDs []string) error {
	start := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_set_related", time.Since(start), false)
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM glossary_term_relations WHERE term_id = $1 OR related_term_id = $1`, termID); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_set_related", time.Since(start), false)
		return fmt.Errorf("clearing related terms: %w", err)
	}

	if len(relatedIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO glossary_term_relations (term_id, related_term_id)
			SELECT $1::uuid, rid FROM unnest($2::uuid[]) AS rid
			UNION ALL
			SELECT rid, $1::uuid FROM unnest($2::uuid[]) AS rid
			ON CONFLICT DO NOTHING`, termID, relatedIDs); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_set_related", time.Since(start), false)
			return fmt.Errorf("inserting related terms: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_set_related", time.Since(start), false)
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_set_related", time.Since(start), true)
	return nil
}

// AddRelatedTerm links two terms both ways. Linking an already related pair is a no-op.
func (r *PostgresRepository) AddRelatedTerm(ctx context.Context, termID, relatedTermID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx, `
		INSERT INTO glossary_term_relations (term_id, related_term_id)
		VALUES ($1, $2), ($2, $1)
		ON CONFLICT DO NOTHING`, termID, relatedTermID)
	r.recorder.RecordDBQuery(ctx, "glossary_add_related", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("adding related term: %w", err)
	}
	return nil
}

// RemoveRelatedTerm unlinks two terms both ways. Unlinking a pair that is not related is a no-op.
func (r *PostgresRepository) RemoveRelatedTerm(ctx context.Context, termID, relatedTermID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx, `
		DELETE FROM glossary_term_relations
		WHERE (term_id = $1 AND related_term_id = $2) OR (term_id = $2 AND related_term_id = $1)`,
		termID, relatedTermID)
	r.recorder.RecordDBQuery(ctx, "glossary_remove_related", time.Since(start), err == nil)
	if err != nil {
		return fmt.Errorf("removing related term: %w", err)
	}
	return nil
}

// SuggestAssets ranks assets by how many of the words match their name, then their tags, then their description. A name match counts most because that is where a table says what it holds. Stubs and assets already linked to the term are left out.
func (r *PostgresRepository) SuggestAssets(ctx context.Context, termID string, words []string, limit int) ([]SuggestedAsset, error) {
	start := time.Now()

	rows, err := r.db.Query(ctx, `
		WITH words AS (SELECT '%' || w || '%' AS pattern FROM unnest($2::text[]) AS w),
		scored AS (
			SELECT a.id, a.name, a.mrn, a.type, a.providers,
				COALESCE(NULLIF(a.user_description, ''), a.description) AS description,
				(SELECT COUNT(*) FROM words WHERE a.name ILIKE words.pattern) * 3
				+ (SELECT COUNT(*) FROM words WHERE EXISTS (SELECT 1 FROM unnest(a.tags) t WHERE t ILIKE words.pattern)) * 2
				+ (SELECT COUNT(*) FROM words WHERE COALESCE(a.user_description, '') || ' ' || COALESCE(a.description, '') ILIKE words.pattern) AS score
			FROM assets a
			WHERE a.is_stub = FALSE
			  AND NOT EXISTS (SELECT 1 FROM asset_terms at WHERE at.asset_id = a.id AND at.glossary_term_id = $1)
			  AND EXISTS (
				SELECT 1 FROM words
				WHERE a.name ILIKE words.pattern
				   OR COALESCE(a.description, '') ILIKE words.pattern
				   OR COALESCE(a.user_description, '') ILIKE words.pattern
				   OR EXISTS (SELECT 1 FROM unnest(a.tags) t WHERE t ILIKE words.pattern)
			  )
		)
		SELECT id, name, mrn, type, providers, description
		FROM scored
		ORDER BY score DESC, name ASC
		LIMIT $3`, termID, words, limit)
	if err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_suggest_assets", time.Since(start), false)
		return nil, fmt.Errorf("suggesting assets: %w", err)
	}
	defer rows.Close()

	out := []SuggestedAsset{}
	for rows.Next() {
		var a SuggestedAsset
		if err := rows.Scan(&a.ID, &a.Name, &a.MRN, &a.Type, &a.Providers, &a.Description); err != nil {
			r.recorder.RecordDBQuery(ctx, "glossary_suggest_assets", time.Since(start), false)
			return nil, fmt.Errorf("scanning suggested asset: %w", err)
		}
		if a.Providers == nil {
			a.Providers = []string{}
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		r.recorder.RecordDBQuery(ctx, "glossary_suggest_assets", time.Since(start), false)
		return nil, fmt.Errorf("iterating suggested assets: %w", err)
	}

	r.recorder.RecordDBQuery(ctx, "glossary_suggest_assets", time.Since(start), true)
	return out, nil
}
