package domain

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const writeEnforcementKey = "write_enforcement"

func (r *PostgresRepository) WriteEnforced(ctx context.Context) (bool, error) {
	var on bool
	err := r.db.QueryRow(ctx, "SELECT value = 'true'::jsonb FROM domain_settings WHERE key = $1", writeEnforcementKey).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return on, err
}

func (r *PostgresRepository) SetWriteEnforced(ctx context.Context, on bool, by string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO domain_settings (key, value, updated_by) VALUES ($1, to_jsonb($2::boolean), NULLIF($3, ''))
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		writeEnforcementKey, on, by)
	return err
}

// Placements returns where each entity is. Entities without a membership row,
// including ones that do not exist, are in Unassigned.
func (r *PostgresRepository) Placements(ctx context.Context, kind Kind, ids []string) (map[string]Placement, error) {
	m, ok := memberships[kind]
	if !ok {
		return nil, ErrInvalidInput
	}
	out := make(map[string]Placement, len(ids))
	for _, id := range ids {
		out[id] = Placement{DomainID: UnassignedID, Path: unassignedPath}
	}
	rows, err := r.db.Query(ctx, `
		SELECT mm.`+m.column+`::text, d.id, d.path
		  FROM `+m.table+` mm JOIN domains d ON d.id = mm.domain_id
		 WHERE mm.`+m.column+`::text = ANY($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var p Placement
		if err := rows.Scan(&id, &p.DomainID, &p.Path); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

// TermPlacements returns where the live glossary terms with these names are.
// Names without a live term are absent from the result.
func (r *PostgresRepository) TermPlacements(ctx context.Context, names []string) (map[string]Placement, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.name, COALESCE(d.id::text, $2), COALESCE(d.path, $3)
		  FROM glossary_terms t
		  LEFT JOIN glossary_term_domains mm ON mm.glossary_term_id = t.id
		  LEFT JOIN domains d ON d.id = mm.domain_id
		 WHERE t.name = ANY($1::text[]) AND t.deleted_at IS NULL`, names, UnassignedID, unassignedPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Placement{}
	for rows.Next() {
		var name string
		var p Placement
		if err := rows.Scan(&name, &p.DomainID, &p.Path); err != nil {
			return nil, err
		}
		out[name] = p
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Audit(ctx context.Context, entries []AuditEntry) error {
	if len(entries) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(`
			INSERT INTO domain_audit_log (actor, action, entity_kind, entity_id, from_domain, to_domain)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			e.Actor, e.Action, e.EntityKind, e.EntityID, e.FromDomain, e.ToDomain)
	}
	return r.db.SendBatch(ctx, batch).Close()
}

func (r *PostgresRepository) AuditLog(ctx context.Context, entityKind, entityID string) ([]AuditEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT actor, action, entity_kind, entity_id, from_domain::text, to_domain::text, at
		  FROM domain_audit_log WHERE entity_kind = $1 AND entity_id = $2 ORDER BY at, id`, entityKind, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.Actor, &e.Action, &e.EntityKind, &e.EntityID, &e.FromDomain, &e.ToDomain, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) AssetIDsByMRN(ctx context.Context, mrns []string) (map[string]string, error) {
	rows, err := r.db.Query(ctx, "SELECT mrn, id FROM assets WHERE mrn = ANY($1::text[])", mrns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var mrn, id string
		if err := rows.Scan(&mrn, &id); err != nil {
			return nil, err
		}
		out[mrn] = id
	}
	return out, rows.Err()
}

// DocOwner returns the entity a documentation page, or the page an image is
// on, belongs to.
func (r *PostgresRepository) DocOwner(ctx context.Context, pageID, imageID string) (entityType, entityID string, found bool, err error) {
	q := "SELECT entity_type, entity_id FROM doc_pages WHERE id::text = $1"
	arg := pageID
	if imageID != "" {
		q = "SELECT p.entity_type, p.entity_id FROM doc_images i JOIN doc_pages p ON p.id = i.page_id WHERE i.id::text = $1"
		arg = imageID
	}
	err = r.db.QueryRow(ctx, q, arg).Scan(&entityType, &entityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	return entityType, entityID, err == nil, err
}
