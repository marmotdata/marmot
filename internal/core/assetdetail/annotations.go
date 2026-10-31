// Package assetdetail holds what the asset page needs beyond the asset record itself: markers people set on schema fields and the counts shown on the page's tabs.
package assetdetail

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidInput = errors.New("invalid input")

// maxDescription bounds a field description so one field cannot bloat every asset response it is merged into.
const maxDescription = 4000

// FieldAnnotation is what people have marked on one field of an asset's schema.
type FieldAnnotation struct {
	// Schema is the key of the schema the field belongs to, as in the asset's schema map.
	Schema string `json:"schema"`
	// Path is the field's dotted path, such as customer.email.
	Path        string      `json:"path"`
	PII         bool        `json:"pii"`
	Certified   bool        `json:"certified"`
	Description *string     `json:"description,omitempty"`
	Terms       []FieldTerm `json:"glossary_terms"`
	UpdatedBy   string      `json:"updated_by,omitempty"`
	UpdatedAt   time.Time   `json:"updated_at"`
} // @name FieldAnnotation

// FieldTerm is a glossary term linked to a field.
type FieldTerm struct {
	ID   string `json:"id"`
	Name string `json:"name"`
} // @name FieldAnnotationTerm

// FieldAnnotationInput sets the markers on one field. Omitted markers keep their current value, so a client can flip one without knowing the rest.
type FieldAnnotationInput struct {
	Schema      string    `json:"schema"`
	Path        string    `json:"path"`
	PII         *bool     `json:"pii,omitempty"`
	Certified   *bool     `json:"certified,omitempty"`
	Description *string   `json:"description,omitempty"`
	TermIDs     *[]string `json:"glossary_term_ids,omitempty"`
} // @name FieldAnnotationInput

// Store reads and writes field annotations and the tab counts.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// ListFieldAnnotations returns every annotated field of an asset, ordered by schema and path.
func (s *Store) ListFieldAnnotations(ctx context.Context, assetID string) ([]FieldAnnotation, error) {
	rows, err := s.db.Query(ctx, `
		SELECT f.schema_name, f.field_path, f.pii, f.certified, f.description,
		       COALESCE(f.updated_by, ''), f.updated_at,
		       COALESCE(array_agg(t.id::text ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL), '{}'),
		       COALESCE(array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL), '{}')
		FROM asset_field_annotations f
		LEFT JOIN glossary_terms t ON t.id = ANY(f.term_ids) AND t.deleted_at IS NULL
		WHERE f.asset_id = $1
		GROUP BY f.schema_name, f.field_path, f.pii, f.certified, f.description, f.updated_by, f.updated_at
		ORDER BY f.schema_name, f.field_path`, assetID)
	if err != nil {
		return nil, fmt.Errorf("listing field annotations: %w", err)
	}
	defer rows.Close()

	out := []FieldAnnotation{}
	for rows.Next() {
		var a FieldAnnotation
		var ids, names []string
		if err := rows.Scan(&a.Schema, &a.Path, &a.PII, &a.Certified, &a.Description, &a.UpdatedBy, &a.UpdatedAt, &ids, &names); err != nil {
			return nil, fmt.Errorf("scanning field annotation: %w", err)
		}
		a.Terms = make([]FieldTerm, len(ids))
		for i := range ids {
			a.Terms[i] = FieldTerm{ID: ids[i], Name: names[i]}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetFieldAnnotation merges the input into the field's markers. A field left with no markers at all is removed rather than kept as an empty row.
func (s *Store) SetFieldAnnotation(ctx context.Context, assetID string, in FieldAnnotationInput, updatedBy string) error {
	in.Path = strings.TrimSpace(in.Path)
	in.Schema = strings.TrimSpace(in.Schema)
	if in.Path == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidInput)
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if len(d) > maxDescription {
			return fmt.Errorf("%w: description is longer than %d characters", ErrInvalidInput, maxDescription)
		}
		in.Description = &d
	}
	var termIDs any
	if in.TermIDs != nil {
		ids := make([]string, 0, len(*in.TermIDs))
		for _, id := range *in.TermIDs {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		termIDs = ids
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after commit

	if termIDs != nil {
		var known int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM glossary_terms WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, termIDs).Scan(&known); err != nil {
			return fmt.Errorf("%w: glossary_term_ids must be glossary term ids", ErrInvalidInput)
		}
		if known != len(termIDs.([]string)) {
			return fmt.Errorf("%w: unknown glossary term", ErrInvalidInput)
		}
	}

	// An empty description clears the edit, so the plugin's description shows again.
	if _, err := tx.Exec(ctx, `
		INSERT INTO asset_field_annotations (asset_id, schema_name, field_path, pii, certified, description, term_ids, updated_by, updated_at)
		VALUES ($1, $2, $3, COALESCE($4, false), COALESCE($5, false), NULLIF($6, ''), COALESCE($7::uuid[], '{}'), $8, NOW())
		ON CONFLICT (asset_id, schema_name, field_path) DO UPDATE SET
			pii         = COALESCE($4, asset_field_annotations.pii),
			certified   = COALESCE($5, asset_field_annotations.certified),
			description = CASE WHEN $6::text IS NULL THEN asset_field_annotations.description ELSE NULLIF($6, '') END,
			term_ids    = COALESCE($7::uuid[], asset_field_annotations.term_ids),
			updated_by  = $8,
			updated_at  = NOW()`,
		assetID, in.Schema, in.Path, in.PII, in.Certified, in.Description, termIDs, updatedBy,
	); err != nil {
		var fk interface{ SQLState() string }
		if errors.As(err, &fk) && fk.SQLState() == "23503" {
			return fmt.Errorf("%w: asset not found", ErrInvalidInput)
		}
		return fmt.Errorf("saving field annotation: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM asset_field_annotations
		WHERE asset_id = $1 AND schema_name = $2 AND field_path = $3
		  AND NOT pii AND NOT certified AND description IS NULL AND cardinality(term_ids) = 0`,
		assetID, in.Schema, in.Path); err != nil {
		return fmt.Errorf("pruning field annotation: %w", err)
	}
	return tx.Commit(ctx)
}

// DeleteFieldAnnotation removes every marker from a field. Removing one that is not there is not an error.
func (s *Store) DeleteFieldAnnotation(ctx context.Context, assetID, schema, path string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM asset_field_annotations WHERE asset_id = $1 AND schema_name = $2 AND field_path = $3`,
		assetID, strings.TrimSpace(schema), strings.TrimSpace(path)); err != nil {
		return fmt.Errorf("deleting field annotation: %w", err)
	}
	return nil
}
