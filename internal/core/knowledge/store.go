package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned for an asset that does not exist or is not a knowledge entry.
var ErrNotFound = errors.New("knowledge entry not found")

// AssetRef names another asset an entry points at.
type AssetRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	MRN    string `json:"mrn"`
	Status string `json:"status,omitempty"`
} // @name KnowledgeAssetRef

// Owner is one owner of an entry, for list views that show who to ask.
type Owner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
} // @name KnowledgeOwner

// Entry is an asset's knowledge record. An asset of a knowledge type that has never been saved here still has an entry, with no status and no fields.
type Entry struct {
	AssetID string `json:"asset_id"`
	Kind    string `json:"kind"`
	// Status is one of the kind's statuses, or empty for an entry nobody has set one on.
	Status string `json:"status"`
	// State is what Status means today: pending, current or historical. A current entry whose effective date is still ahead is pending.
	State  State          `json:"state,omitempty"`
	Fields map[string]any `json:"fields"`
	// EffectiveFrom and ReviewBy are dates written as YYYY-MM-DD.
	EffectiveFrom  *string    `json:"effective_from,omitempty"`
	ReviewBy       *string    `json:"review_by,omitempty"`
	ReviewDue      bool       `json:"review_due"`
	LastReviewedAt *time.Time `json:"last_reviewed_at,omitempty"`
	LastReviewedBy string     `json:"last_reviewed_by,omitempty"`
	// Supersedes is the entry this one replaces, and SupersededBy the newer entries that replace this one.
	Supersedes   *AssetRef  `json:"supersedes,omitempty"`
	SupersededBy []AssetRef `json:"superseded_by"`
	AppliesTo    []AssetRef `json:"applies_to"`
	UpdatedBy    string     `json:"updated_by,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
} // @name KnowledgeEntry

// Summary is one entry in a list.
type Summary struct {
	AssetID       string    `json:"asset_id"`
	Name          string    `json:"name"`
	Summary       string    `json:"summary,omitempty"`
	MRN           string    `json:"mrn"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	State         State     `json:"state,omitempty"`
	EffectiveFrom *string   `json:"effective_from,omitempty"`
	ReviewBy      *string   `json:"review_by,omitempty"`
	ReviewDue     bool      `json:"review_due"`
	Owners        []Owner   `json:"owners"`
	UpdatedAt     time.Time `json:"updated_at"`
} // @name KnowledgeSummary

// Counts sums every entry by kind and by state, for an overview of what has been written down and what needs attention.
type Counts struct {
	Total     int            `json:"total"`
	ByKind    map[string]int `json:"by_kind"`
	ByState   map[string]int `json:"by_state"`
	ReviewDue int            `json:"review_due"`
	// Unset counts entries nobody has given a status yet.
	Unset int `json:"unset"`
} // @name KnowledgeCounts

// ListFilter narrows a list of entries. Kinds and Statuses are ORed within themselves and ANDed together; ReviewDue keeps entries whose review date has passed and OwnerID those owned by a user or by a team the user is in.
type ListFilter struct {
	Kinds     []string
	Statuses  []string
	Query     string
	ReviewDue bool
	OwnerID   string
	Sort      string
	Limit     int
	Offset    int
}

type record struct {
	kind          string
	status        string
	fields        map[string]any
	effectiveFrom *string
	reviewBy      *string
	supersedes    *string
	appliesTo     []string
	updatedBy     string
	// retire is the entry Supersedes points at when this one is current, which is then marked superseded.
	retire *string
}

type Repository interface {
	AssetType(ctx context.Context, assetID string) (string, error)
	AssetTypes(ctx context.Context, ids []string) (map[string]string, error)
	Get(ctx context.Context, assetID string) (*Entry, error)
	Save(ctx context.Context, assetID string, r record) error
	MarkReviewed(ctx context.Context, assetID string, reviewBy *string, by string) error
	List(ctx context.Context, f ListFilter) ([]Summary, int, error)
	ApplyingTo(ctx context.Context, assetID string) ([]Summary, error)
	Counts(ctx context.Context) (*Counts, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) AssetType(ctx context.Context, assetID string) (string, error) {
	var t string
	err := r.db.QueryRow(ctx, `SELECT type FROM assets WHERE id = $1 AND NOT is_stub`, assetID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading asset type: %w", err)
	}
	return t, nil
}

func (r *PostgresRepository) AssetTypes(ctx context.Context, ids []string) (map[string]string, error) {
	rows, err := r.db.Query(ctx, `SELECT id, type FROM assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("checking assets: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, t string
		if err := rows.Scan(&id, &t); err != nil {
			return nil, fmt.Errorf("reading asset: %w", err)
		}
		out[id] = t
	}
	return out, rows.Err()
}

const refSQL = `json_build_object('id', x.id, 'name', COALESCE(x.name, ''), 'type', x.type, 'mrn', COALESCE(x.mrn, ''), 'status', COALESCE(xk.status, ''))`

func (r *PostgresRepository) Get(ctx context.Context, assetID string) (*Entry, error) {
	var (
		e                                 Entry
		fields, supersedes                []byte
		supersededBy, appliesTo           []byte
		status, updatedBy, lastReviewedBy *string
	)
	err := r.db.QueryRow(ctx, `
		SELECT k.status, k.fields, k.effective_from::text, k.review_by::text, COALESCE(k.review_by <= CURRENT_DATE, false),
		       k.last_reviewed_at, k.last_reviewed_by,
		       (SELECT `+refSQL+` FROM assets x LEFT JOIN knowledge_entries xk ON xk.asset_id = x.id WHERE x.id = k.supersedes),
		       COALESCE((SELECT json_agg(`+refSQL+` ORDER BY x.name) FROM knowledge_entries xk JOIN assets x ON x.id = xk.asset_id WHERE xk.supersedes = a.id), '[]'),
		       COALESCE((SELECT json_agg(`+refSQL+` ORDER BY x.name) FROM knowledge_applies_to l JOIN assets x ON x.id = l.target_id LEFT JOIN knowledge_entries xk ON xk.asset_id = x.id WHERE l.entry_id = a.id), '[]'),
		       k.updated_by, k.updated_at
		FROM assets a
		LEFT JOIN knowledge_entries k ON k.asset_id = a.id
		WHERE a.id = $1`, assetID).Scan(&status, &fields, &e.EffectiveFrom, &e.ReviewBy, &e.ReviewDue,
		&e.LastReviewedAt, &lastReviewedBy, &supersedes, &supersededBy, &appliesTo, &updatedBy, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading knowledge entry: %w", err)
	}
	e.AssetID = assetID
	e.Fields = map[string]any{}
	if len(fields) > 0 {
		if err := json.Unmarshal(fields, &e.Fields); err != nil {
			return nil, fmt.Errorf("decoding fields: %w", err)
		}
	}
	if len(supersedes) > 0 {
		e.Supersedes = &AssetRef{}
		if err := json.Unmarshal(supersedes, e.Supersedes); err != nil {
			return nil, fmt.Errorf("decoding supersedes: %w", err)
		}
	}
	if err := json.Unmarshal(supersededBy, &e.SupersededBy); err != nil {
		return nil, fmt.Errorf("decoding superseded_by: %w", err)
	}
	if err := json.Unmarshal(appliesTo, &e.AppliesTo); err != nil {
		return nil, fmt.Errorf("decoding applies_to: %w", err)
	}
	e.Status = deref(status)
	e.UpdatedBy = deref(updatedBy)
	e.LastReviewedBy = deref(lastReviewedBy)
	return &e, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *PostgresRepository) Save(ctx context.Context, assetID string, rec record) error {
	fields, err := json.Marshal(rec.fields)
	if err != nil {
		return fmt.Errorf("encoding fields: %w", err)
	}
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_entries (asset_id, kind, status, fields, effective_from, review_by, supersedes, updated_by)
			VALUES ($1, $2, $3, $4, $5::date, $6::date, $7, $8)
			ON CONFLICT (asset_id) DO UPDATE SET
				kind = EXCLUDED.kind, status = EXCLUDED.status, fields = EXCLUDED.fields,
				effective_from = EXCLUDED.effective_from, review_by = EXCLUDED.review_by,
				supersedes = EXCLUDED.supersedes, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
			assetID, rec.kind, rec.status, fields, rec.effectiveFrom, rec.reviewBy, rec.supersedes, rec.updatedBy); err != nil {
			return fmt.Errorf("saving knowledge entry: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM knowledge_applies_to WHERE entry_id = $1`, assetID); err != nil {
			return fmt.Errorf("clearing applies-to links: %w", err)
		}
		if len(rec.appliesTo) > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO knowledge_applies_to (entry_id, target_id)
				SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING`, assetID, rec.appliesTo); err != nil {
				return fmt.Errorf("saving applies-to links: %w", err)
			}
		}
		if rec.retire != nil {
			// The replaced entry is checked to be the same kind before saving, so "superseded" is one of its statuses too.
			if _, err := tx.Exec(ctx, `
				INSERT INTO knowledge_entries (asset_id, kind, status, updated_by)
				VALUES ($1, $2, 'superseded', $3)
				ON CONFLICT (asset_id) DO UPDATE SET status = 'superseded', updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
				*rec.retire, rec.kind, rec.updatedBy); err != nil {
				return fmt.Errorf("marking the replaced entry superseded: %w", err)
			}
		}
		return nil
	})
}

func (r *PostgresRepository) MarkReviewed(ctx context.Context, assetID string, reviewBy *string, by string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE knowledge_entries
		SET last_reviewed_at = NOW(), last_reviewed_by = $2, review_by = COALESCE($3::date, review_by), updated_at = NOW()
		WHERE asset_id = $1`, assetID, by, reviewBy)
	if err != nil {
		return fmt.Errorf("marking entry reviewed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUnsaved
	}
	return nil
}

// summarySQL selects one list row from assets a left-joined to knowledge_entries k.
const summarySQL = `
	SELECT a.id, COALESCE(a.name, ''), COALESCE(NULLIF(a.user_description, ''), a.description, ''), COALESCE(a.mrn, ''), a.type,
	       COALESCE(k.status, ''), k.effective_from::text, k.review_by::text, COALESCE(k.review_by <= CURRENT_DATE, false),
	       COALESCE((SELECT json_agg(json_build_object(
	                    'id', COALESCE(u.id::text, t.id::text),
	                    'name', COALESCE(u.name, t.name, ''),
	                    'type', CASE WHEN ao.user_id IS NOT NULL THEN 'user' ELSE 'team' END) ORDER BY COALESCE(u.name, t.name))
	                 FROM asset_owners ao
	                 LEFT JOIN users u ON u.id = ao.user_id
	                 LEFT JOIN teams t ON t.id = ao.team_id
	                 WHERE ao.asset_id = a.id), '[]'),
	       GREATEST(a.updated_at, COALESCE(k.updated_at, a.updated_at)) AS changed_at`

func scanSummary(row pgx.Row) (Summary, error) {
	var (
		s      Summary
		typ    string
		owners []byte
	)
	if err := row.Scan(&s.AssetID, &s.Name, &s.Summary, &s.MRN, &typ, &s.Status, &s.EffectiveFrom, &s.ReviewBy, &s.ReviewDue, &owners, &s.UpdatedAt); err != nil {
		return s, err
	}
	if err := json.Unmarshal(owners, &s.Owners); err != nil {
		return s, fmt.Errorf("decoding owners: %w", err)
	}
	if k, ok := KindOfType(typ); ok {
		s.Kind = k.ID
		s.State = stateOn(k, s.Status, s.EffectiveFrom, time.Now())
	}
	return s, nil
}

func collect(rows pgx.Rows) ([]Summary, error) {
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		s, err := scanSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("reading knowledge entry: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Entries no longer in use sink below the rest whatever the sort, so the current answer comes first but the old one is still findable.
const historicalLast = `COALESCE(k.status IN ('superseded', 'retired', 'rejected', 'deprecated'), false), `

var sorts = map[string]string{
	"updated": historicalLast + "changed_at DESC, a.id",
	"name":    historicalLast + "lower(a.name), a.id",
	"review":  "k.review_by ASC NULLS LAST, a.id",
}

func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]Summary, int, error) {
	types := Types()
	if len(f.Kinds) > 0 {
		types = types[:0:0]
		for _, id := range f.Kinds {
			if k, ok := KindByID(id); ok {
				types = append(types, strings.ToLower(k.Type))
			}
		}
	}
	where := []string{"lower(a.type) = ANY($1)", "NOT a.is_stub"}
	args := []any{types}
	if len(f.Statuses) > 0 {
		args = append(args, f.Statuses)
		where = append(where, fmt.Sprintf("k.status = ANY($%d)", len(args)))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+escapeLike(q)+"%")
		where = append(where, fmt.Sprintf(`(a.name ILIKE $%[1]d OR a.user_description ILIKE $%[1]d OR a.description ILIKE $%[1]d
			OR k.fields::text ILIKE $%[1]d
			OR EXISTS (SELECT 1 FROM doc_pages dp WHERE dp.entity_type = 'asset' AND dp.entity_id = a.mrn AND (dp.title ILIKE $%[1]d OR dp.content ILIKE $%[1]d)))`, len(args)))
	}
	if f.ReviewDue {
		where = append(where, "k.review_by <= CURRENT_DATE")
	}
	if f.OwnerID != "" {
		args = append(args, f.OwnerID)
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM asset_owners ao WHERE ao.asset_id = a.id
			AND (ao.user_id::text = $%[1]d OR ao.team_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id::text = $%[1]d)))`, len(args)))
	}
	from := `FROM assets a LEFT JOIN knowledge_entries k ON k.asset_id = a.id WHERE ` + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting knowledge entries: %w", err)
	}
	order, ok := sorts[f.Sort]
	if !ok {
		order = sorts["updated"]
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, summarySQL+` `+from+fmt.Sprintf(` ORDER BY %s LIMIT $%d OFFSET $%d`, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing knowledge entries: %w", err)
	}
	out, err := collect(rows)
	return out, total, err
}

func (r *PostgresRepository) ApplyingTo(ctx context.Context, assetID string) ([]Summary, error) {
	rows, err := r.db.Query(ctx, summarySQL+`
		FROM knowledge_applies_to l
		JOIN assets a ON a.id = l.entry_id
		LEFT JOIN knowledge_entries k ON k.asset_id = a.id
		WHERE l.target_id = $1
		ORDER BY lower(a.name)`, assetID)
	if err != nil {
		return nil, fmt.Errorf("listing entries for asset: %w", err)
	}
	return collect(rows)
}

func (r *PostgresRepository) Counts(ctx context.Context) (*Counts, error) {
	rows, err := r.db.Query(ctx, `
		SELECT lower(a.type), COALESCE(k.status, ''), k.effective_from::text, COUNT(*), COUNT(*) FILTER (WHERE k.review_by <= CURRENT_DATE)
		FROM assets a LEFT JOIN knowledge_entries k ON k.asset_id = a.id
		WHERE lower(a.type) = ANY($1) AND NOT a.is_stub
		GROUP BY 1, 2, 3`, Types())
	if err != nil {
		return nil, fmt.Errorf("counting knowledge entries: %w", err)
	}
	defer rows.Close()
	c := &Counts{ByKind: map[string]int{}, ByState: map[string]int{}}
	now := time.Now()
	for rows.Next() {
		var (
			typ, status string
			effective   *string
			n, due      int
		)
		if err := rows.Scan(&typ, &status, &effective, &n, &due); err != nil {
			return nil, fmt.Errorf("reading counts: %w", err)
		}
		k, ok := KindOfType(typ)
		if !ok {
			continue
		}
		c.Total += n
		c.ReviewDue += due
		c.ByKind[k.ID] += n
		if state := stateOn(k, status, effective, now); state != "" {
			c.ByState[string(state)] += n
		} else {
			c.Unset += n
		}
	}
	return c, rows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
