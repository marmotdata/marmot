// Package recent remembers what each person opened last so Home can offer it back to them.
package recent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Kinds of item a person can come back to.
const (
	KindAsset        = "asset"
	KindGlossaryTerm = "glossary_term"
	KindDataProduct  = "data_product"
	KindSavedQuery   = "saved_query"
)

var (
	ErrInvalidKind  = errors.New("unknown kind")
	ErrItemNotFound = errors.New("item not found")
)

const (
	// keepPerUser bounds each person's rows, well above what Home shows so items a person can no longer see do not empty the list.
	keepPerUser = 50
	// throttle is how long a repeat view of the same item leaves the stored time alone, so reloading or tabbing back and forth does not write every time.
	throttle     = time.Minute
	defaultLimit = 10
)

// ValidKind reports whether kind is one the store records.
func ValidKind(kind string) bool {
	switch kind {
	case KindAsset, KindGlossaryTerm, KindDataProduct, KindSavedQuery:
		return true
	}
	return false
}

// Item is one thing a person viewed, described as it is now rather than when it was viewed, so renames show through.
type Item struct {
	Kind     string    `json:"kind"`
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type,omitempty"`
	Provider string    `json:"provider,omitempty"`
	MRN      string    `json:"mrn,omitempty"`
	ViewedAt time.Time `json:"viewed_at"`
} // @name RecentItem

// Store keeps recently viewed items per person.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// ResolveAsset finds an asset's id from its MRN, which is all the asset page's address carries.
func (s *Store) ResolveAsset(ctx context.Context, mrn string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `SELECT id FROM assets WHERE mrn = $1`, mrn).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrItemNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolving asset: %w", err)
	}
	return id, nil
}

// exists checks the item is there before it is recorded, and for a saved query that it is the viewer's own.
func (s *Store) exists(ctx context.Context, userID, kind, id string) (bool, error) {
	var query string
	args := []any{id}
	switch kind {
	case KindAsset:
		query = `SELECT EXISTS (SELECT 1 FROM assets WHERE id = $1)`
	case KindGlossaryTerm:
		query = `SELECT EXISTS (SELECT 1 FROM glossary_terms WHERE id::text = $1 AND deleted_at IS NULL)`
	case KindDataProduct:
		query = `SELECT EXISTS (SELECT 1 FROM data_products WHERE id::text = $1)`
	case KindSavedQuery:
		query = `SELECT EXISTS (SELECT 1 FROM saved_queries WHERE id::text = $1 AND user_id = $2)`
		args = append(args, userID)
	default:
		return false, ErrInvalidKind
	}
	var ok bool
	if err := s.db.QueryRow(ctx, query, args...).Scan(&ok); err != nil {
		return false, fmt.Errorf("checking %s: %w", kind, err)
	}
	return ok, nil
}

// Record notes that a person viewed an item. A repeat view within the throttle window changes nothing, and each person keeps only their latest items.
func (s *Store) Record(ctx context.Context, userID, kind, id string) error {
	if !ValidKind(kind) {
		return ErrInvalidKind
	}
	ok, err := s.exists(ctx, userID, kind, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotFound
	}

	tag, err := s.db.Exec(ctx, `
		INSERT INTO recent_views (user_id, kind, item_id, viewed_at) VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, kind, item_id) DO UPDATE SET viewed_at = NOW()
		WHERE recent_views.viewed_at < NOW() - make_interval(secs => $4)`,
		userID, kind, id, throttle.Seconds())
	if err != nil {
		return fmt.Errorf("recording view: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := s.db.Exec(ctx, `
		DELETE FROM recent_views
		WHERE user_id = $1 AND viewed_at < (
			SELECT viewed_at FROM recent_views WHERE user_id = $1
			ORDER BY viewed_at DESC OFFSET $2 LIMIT 1
		)`, userID, keepPerUser-1); err != nil {
		return fmt.Errorf("trimming views: %w", err)
	}
	return nil
}

// List returns the person's latest items of the given kinds, newest first. Items that have since been deleted are left out.
func (s *Store) List(ctx context.Context, userID string, kinds []string, limit int) ([]Item, error) {
	if limit <= 0 || limit > keepPerUser {
		limit = defaultLimit
	}
	if len(kinds) == 0 {
		return []Item{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT r.kind, r.item_id, r.viewed_at,
		       COALESCE(a.name, a.mrn, g.name, p.name, q.name),
		       CASE r.kind WHEN 'asset' THEN COALESCE(a.type, '') ELSE '' END,
		       CASE r.kind WHEN 'asset' THEN COALESCE(a.providers[1], '') WHEN 'saved_query' THEN COALESCE(sc.plugin_id, '') ELSE '' END,
		       COALESCE(a.mrn, '')
		FROM recent_views r
		LEFT JOIN assets a ON r.kind = 'asset' AND a.id = r.item_id
		LEFT JOIN glossary_terms g ON r.kind = 'glossary_term' AND g.id::text = r.item_id AND g.deleted_at IS NULL
		LEFT JOIN data_products p ON r.kind = 'data_product' AND p.id::text = r.item_id
		LEFT JOIN saved_queries q ON r.kind = 'saved_query' AND q.id::text = r.item_id AND q.user_id = r.user_id
		LEFT JOIN ingestion_schedules sc ON r.kind = 'saved_query' AND sc.name = q.source_name
		WHERE r.user_id = $1 AND r.kind = ANY($2)
		  AND COALESCE(a.name, a.mrn, g.name, p.name, q.name) IS NOT NULL
		ORDER BY r.viewed_at DESC
		LIMIT $3`, userID, kinds, limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent views: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.Kind, &it.ID, &it.ViewedAt, &it.Name, &it.Type, &it.Provider, &it.MRN); err != nil {
			return nil, fmt.Errorf("scanning recent view: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
