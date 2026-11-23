package knowledge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned for an asset that does not exist or is not a knowledge entry.
var ErrNotFound = errors.New("knowledge entry not found")

// AssetRef names another asset an entry points at.
type AssetRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	MRN  string `json:"mrn"`
	// Provider is the asset's first provider, for its logo.
	Provider string `json:"provider,omitempty"`
	Status   string `json:"status,omitempty"`
} // @name KnowledgeAssetRef

// Owner is one owner of an entry, for list views that show who to ask.
type Owner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
} // @name KnowledgeOwner

// Link is one asset an entry is linked to, and how.
type Link struct {
	Relation string   `json:"relation"`
	Target   AssetRef `json:"target"`
} // @name KnowledgeLink

// Rule links an entry to every asset matching a selector, evaluated when the asset is read. Matches counts the assets it covers today, up to MatchLimit.
type Rule struct {
	ID            string `json:"id"`
	Relation      string `json:"relation"`
	SelectorType  string `json:"selector_type"`
	SelectorValue string `json:"selector_value"`
	Matches       int    `json:"matches"`
} // @name KnowledgeRule

// MatchLimit caps how far a rule's coverage is counted, so a rule over a whole catalog stays cheap to show; more reads as "MatchLimit or more".
const MatchLimit = 10000

// Entry is an asset's knowledge record. An asset of a knowledge type that has never been saved here still has an entry, with no status and no fields.
type Entry struct {
	AssetID string `json:"asset_id"`
	Kind    string `json:"kind"`
	// Status is one of the kind's statuses, or empty for an entry nobody has set one on.
	Status string `json:"status"`
	// State is what Status means today: pending, current or historical. A current entry whose effective date is still ahead is pending.
	State  State          `json:"state,omitempty" swaggertype:"string" enums:"pending,current,historical"`
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
	// Links holds the first EntryLinkLimit links, and LinkCount how many there are; page through the rest with the entry's links endpoint.
	Links     []Link     `json:"links"`
	LinkCount int        `json:"link_count"`
	Rules     []Rule     `json:"rules"`
	UpdatedBy string     `json:"updated_by,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
} // @name KnowledgeEntry

// EntryLinkLimit is how many links an entry carries inline.
const EntryLinkLimit = 20

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

// AboutItem is one entry about an asset, with where it came from: Relation for every source, Via for an inherited entry (the containing asset it is linked to) and Rule for a matched one.
type AboutItem struct {
	Summary
	Source   string    `json:"source"`
	Relation string    `json:"relation"`
	Via      *AssetRef `json:"via,omitempty"`
	Rule     *Rule     `json:"rule,omitempty"`
} // @name KnowledgeAboutItem

// AboutPage is a page of entries about an asset, with totals over every page. Next is the cursor for the following page, empty on the last.
type AboutPage struct {
	Entries  []AboutItem    `json:"entries"`
	Next     string         `json:"next,omitempty"`
	Total    int            `json:"total"`
	ByKind   map[string]int `json:"by_kind"`
	BySource map[string]int `json:"by_source"`
} // @name KnowledgeAboutPage

// AboutFilter narrows the entries about an asset. Empty fields match everything.
type AboutFilter struct {
	Kinds    []string
	States   []State
	Sources  []string
	Relation string
	Limit    int
	Cursor   string
}

// LinkPage is a page of an entry's links.
type LinkPage struct {
	Links []Link `json:"links"`
	Next  string `json:"next,omitempty"`
	Total int    `json:"total"`
} // @name KnowledgeLinkPage

// Counts sums every entry by kind, state and owning team, for an overview of what has been written down and what needs attention.
type Counts struct {
	Total     int            `json:"total"`
	ByKind    map[string]int `json:"by_kind"`
	ByState   map[string]int `json:"by_state"`
	ByTeam    []TeamCount    `json:"by_team"`
	ReviewDue int            `json:"review_due"`
	// Unset counts entries nobody has given a status yet.
	Unset int `json:"unset"`
} // @name KnowledgeCounts

// TeamCount is how many entries a team owns.
type TeamCount struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
} // @name KnowledgeTeamCount

// ListFilter narrows a list of entries. Kinds and Statuses are ORed within themselves and ANDed together; ReviewDue keeps entries whose review date has passed, OwnerID those owned by a user or a team they are in, and TeamIDs those owned by any of the teams.
type ListFilter struct {
	Kinds    []string
	Statuses []string
	Query    string
	// AnyWord matches entries holding any word of Query rather than every word, ranked by how many they hold, for callers that fall back when nothing matches every word.
	AnyWord   bool
	ReviewDue bool
	OwnerID   string
	TeamIDs   []string
	Sort      string
	Limit     int
	Offset    int
}

type linkKey struct {
	relation, target string
}

type record struct {
	kind          string
	status        string
	fields        map[string]any
	effectiveFrom *string
	reviewBy      *string
	supersedes    *string
	// links and rules replace the entry's when set and are left alone when nil.
	links     *[]linkKey
	rules     *[]RuleInput
	updatedBy string
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
	Links(ctx context.Context, assetID, relation, cursor string, limit int) (*LinkPage, error)
	About(ctx context.Context, assetID string, f AboutFilter) (*AboutPage, error)
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

const refSQL = `json_build_object('id', x.id, 'name', COALESCE(x.name, ''), 'type', x.type, 'mrn', COALESCE(x.mrn, ''), 'provider', COALESCE(x.providers[1], ''), 'status', COALESCE(xk.status, ''))`

// ruleMatchesSQL counts the assets a rule r covers, stopping at MatchLimit.
var ruleMatchesSQL = fmt.Sprintf(`(SELECT COUNT(*) FROM (SELECT 1 FROM assets m WHERE NOT m.is_stub AND m.id <> r.entry_id AND CASE r.selector_type
	WHEN 'tag' THEN m.tags @> ARRAY[r.selector_value]
	WHEN 'type' THEN lower(m.type) = r.selector_value
	ELSE EXISTS (SELECT 1 FROM unnest(m.providers) p WHERE lower(p) = r.selector_value) END LIMIT %d) c)`, MatchLimit)

func (r *PostgresRepository) Get(ctx context.Context, assetID string) (*Entry, error) {
	var (
		e                                 Entry
		fields, supersedes                []byte
		supersededBy, links, rules        []byte
		status, updatedBy, lastReviewedBy *string
	)
	err := r.db.QueryRow(ctx, `
		SELECT k.status, k.fields, k.effective_from::text, k.review_by::text, COALESCE(k.review_by <= CURRENT_DATE, false),
		       k.last_reviewed_at, k.last_reviewed_by,
		       (SELECT `+refSQL+` FROM assets x LEFT JOIN knowledge_entries xk ON xk.asset_id = x.id WHERE x.id = k.supersedes),
		       COALESCE((SELECT json_agg(`+refSQL+` ORDER BY x.name) FROM knowledge_entries xk JOIN assets x ON x.id = xk.asset_id WHERE xk.supersedes = a.id), '[]'),
		       COALESCE((SELECT json_agg(json_build_object('relation', l.relation, 'target', `+refSQL+`) ORDER BY l.relation, lower(x.name), x.id)
		                 FROM (SELECT * FROM knowledge_links WHERE entry_id = a.id ORDER BY relation, target_id LIMIT `+strconv.Itoa(EntryLinkLimit)+`) l
		                 JOIN assets x ON x.id = l.target_id LEFT JOIN knowledge_entries xk ON xk.asset_id = x.id), '[]'),
		       (SELECT COUNT(*) FROM knowledge_links WHERE entry_id = a.id),
		       COALESCE((SELECT json_agg(json_build_object('id', r.id, 'relation', r.relation, 'selector_type', r.selector_type,
		                    'selector_value', r.selector_value, 'matches', `+ruleMatchesSQL+`) ORDER BY r.created_at)
		                 FROM knowledge_rules r WHERE r.entry_id = a.id), '[]'),
		       k.updated_by, k.updated_at
		FROM assets a
		LEFT JOIN knowledge_entries k ON k.asset_id = a.id
		WHERE a.id = $1`, assetID).Scan(&status, &fields, &e.EffectiveFrom, &e.ReviewBy, &e.ReviewDue,
		&e.LastReviewedAt, &lastReviewedBy, &supersedes, &supersededBy, &links, &e.LinkCount, &rules, &updatedBy, &e.UpdatedAt)
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
	for _, d := range []struct {
		raw  []byte
		into any
	}{{supersededBy, &e.SupersededBy}, {links, &e.Links}, {rules, &e.Rules}} {
		if err := json.Unmarshal(d.raw, d.into); err != nil {
			return nil, fmt.Errorf("decoding entry: %w", err)
		}
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
		if rec.links != nil {
			if err := saveLinks(ctx, tx, assetID, *rec.links, rec.updatedBy); err != nil {
				return err
			}
		}
		if rec.rules != nil {
			if err := saveRules(ctx, tx, assetID, *rec.rules, rec.updatedBy); err != nil {
				return err
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

// saveLinks writes only the links that changed, so saving an entry linked to thousands of assets costs what was edited.
func saveLinks(ctx context.Context, tx pgx.Tx, assetID string, links []linkKey, by string) error {
	relations := make([]string, len(links))
	targets := make([]string, len(links))
	for i, l := range links {
		relations[i], targets[i] = l.relation, l.target
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM knowledge_links l WHERE l.entry_id = $1
		AND NOT EXISTS (SELECT 1 FROM unnest($2::text[], $3::text[]) AS w(relation, target_id) WHERE w.relation = l.relation AND w.target_id = l.target_id)`,
		assetID, relations, targets); err != nil {
		return fmt.Errorf("removing links: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_links (entry_id, relation, target_id, kind, state_rank, name_key, created_by)
		SELECT $1, w.relation, w.target_id, k.kind, k.state_rank, lower(COALESCE(a.name, '')), $4
		FROM unnest($2::text[], $3::text[]) AS w(relation, target_id)
		JOIN knowledge_entries k ON k.asset_id = $1
		JOIN assets a ON a.id = $1
		ON CONFLICT DO NOTHING`, assetID, relations, targets, by); err != nil {
		return fmt.Errorf("adding links: %w", err)
	}
	return nil
}

func saveRules(ctx context.Context, tx pgx.Tx, assetID string, rules []RuleInput, by string) error {
	ruleRel := make([]string, len(rules))
	ruleType := make([]string, len(rules))
	ruleValue := make([]string, len(rules))
	for i, ru := range rules {
		ruleRel[i], ruleType[i], ruleValue[i] = ru.Relation, ru.SelectorType, ru.SelectorValue
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM knowledge_rules r WHERE r.entry_id = $1
		AND NOT EXISTS (SELECT 1 FROM unnest($2::text[], $3::text[], $4::text[]) AS w(relation, selector_type, selector_value)
			WHERE w.relation = r.relation AND w.selector_type = r.selector_type AND w.selector_value = r.selector_value)`,
		assetID, ruleRel, ruleType, ruleValue); err != nil {
		return fmt.Errorf("removing rules: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_rules (entry_id, relation, selector_type, selector_value, created_by)
		SELECT $1, w.relation, w.selector_type, w.selector_value, $5
		FROM unnest($2::text[], $3::text[], $4::text[]) AS w(relation, selector_type, selector_value)
		ON CONFLICT DO NOTHING`, assetID, ruleRel, ruleType, ruleValue, by); err != nil {
		return fmt.Errorf("adding rules: %w", err)
	}
	return nil
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

// summaryColumns selects one list row from assets a left-joined to knowledge_entries k.
const summaryColumns = `
	a.id, COALESCE(a.name, ''), COALESCE(NULLIF(a.user_description, ''), a.description, ''), COALESCE(a.mrn, ''), a.type,
	COALESCE(k.status, ''), k.effective_from::text, k.review_by::text, COALESCE(k.review_by <= CURRENT_DATE, false),
	COALESCE((SELECT json_agg(json_build_object(
	             'id', COALESCE(u.id::text, t.id::text),
	             'name', COALESCE(u.name, t.name, ''),
	             'type', CASE WHEN ao.user_id IS NOT NULL THEN 'user' ELSE 'team' END) ORDER BY COALESCE(u.name, t.name))
	          FROM asset_owners ao
	          LEFT JOIN users u ON u.id = ao.user_id
	          LEFT JOIN teams t ON t.id = ao.team_id
	          WHERE ao.asset_id = a.id), '[]'),
	GREATEST(a.updated_at, COALESCE(k.updated_at, a.updated_at))`

// summaryScanner scans summaryColumns followed by any extra columns.
type summaryScanner struct {
	s      Summary
	typ    string
	owners []byte
}

func (sc *summaryScanner) targets(extra ...any) []any {
	return append([]any{&sc.s.AssetID, &sc.s.Name, &sc.s.Summary, &sc.s.MRN, &sc.typ, &sc.s.Status, &sc.s.EffectiveFrom,
		&sc.s.ReviewBy, &sc.s.ReviewDue, &sc.owners, &sc.s.UpdatedAt}, extra...)
}

func (sc *summaryScanner) summary(now time.Time) (Summary, error) {
	s := sc.s
	if err := json.Unmarshal(sc.owners, &s.Owners); err != nil {
		return s, fmt.Errorf("decoding owners: %w", err)
	}
	if k, ok := KindOfType(sc.typ); ok {
		s.Kind = k.ID
		s.State = stateOn(k, s.Status, s.EffectiveFrom, now)
	}
	return s, nil
}

var sorts = map[string]string{
	"updated":   "COALESCE(k.state_rank, 3) = 2, changed_at DESC, a.id",
	"name":      "COALESCE(k.state_rank, 3) = 2, lower(a.name), a.id",
	"review":    "k.review_by ASC NULLS LAST, a.id",
	"relevance": "COALESCE(k.state_rank, 3) = 2, ts_rank(k.search_text, query) DESC, changed_at DESC, a.id",
}

// prefixQuery turns what someone typed into a tsquery that matches each word as a prefix, so "expen pol" finds the expenses policy. It returns "" when there are no words.
func prefixQuery(q string, anyWord bool) string {
	words := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for i, w := range words {
		words[i] = w + ":*"
	}
	if anyWord {
		return strings.Join(words, " | ")
	}
	return strings.Join(words, " & ")
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
	queryExpr := "NULL::tsquery"
	if len(f.Statuses) > 0 {
		args = append(args, f.Statuses)
		where = append(where, fmt.Sprintf("k.status = ANY($%d)", len(args)))
	}
	if q := prefixQuery(f.Query, f.AnyWord); q != "" {
		args = append(args, q, "%"+escapeLike(strings.TrimSpace(f.Query))+"%")
		queryExpr = fmt.Sprintf("to_tsquery('simple', $%d)", len(args)-1)
		// Entries never saved have no search text, so their title is matched directly.
		where = append(where, fmt.Sprintf(`(k.search_text @@ %s OR (k.asset_id IS NULL AND a.name ILIKE $%d))`, queryExpr, len(args)))
	}
	if f.ReviewDue {
		where = append(where, "k.review_by <= CURRENT_DATE")
	}
	if f.OwnerID != "" {
		args = append(args, f.OwnerID)
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM asset_owners ao WHERE ao.asset_id = a.id
			AND (ao.user_id::text = $%[1]d OR ao.team_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id::text = $%[1]d)))`, len(args)))
	}
	if len(f.TeamIDs) > 0 {
		args = append(args, f.TeamIDs)
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM asset_owners ao WHERE ao.asset_id = a.id AND ao.team_id::text = ANY($%d))`, len(args)))
	}
	from := `FROM assets a LEFT JOIN knowledge_entries k ON k.asset_id = a.id WHERE ` + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting knowledge entries: %w", err)
	}
	sort := f.Sort
	if sort == "" && f.Query != "" && queryExpr != "NULL::tsquery" {
		sort = "relevance"
	}
	order, ok := sorts[sort]
	if !ok || (sort == "relevance" && queryExpr == "NULL::tsquery") {
		order = sorts["updated"]
	}
	args = append(args, f.Limit, f.Offset)
	order = strings.Replace(order, "ts_rank(k.search_text, query)", "ts_rank(k.search_text, "+queryExpr+")", 1)
	sql := fmt.Sprintf(`SELECT %s AS changed_at %s ORDER BY %s LIMIT $%d OFFSET $%d`, summaryColumns, from, order, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing knowledge entries: %w", err)
	}
	defer rows.Close()
	out := []Summary{}
	now := time.Now()
	for rows.Next() {
		var sc summaryScanner
		if err := rows.Scan(sc.targets()...); err != nil {
			return nil, 0, fmt.Errorf("reading knowledge entry: %w", err)
		}
		s, err := sc.summary(now)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

type cursor struct {
	rank int16
	name string
	id   string
}

func encodeCursor(c cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d\x00%s\x00%s", c.rank, c.name, c.id)))
}

func decodeCursor(s string) (*cursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, invalid("the cursor is not one this server issued")
	}
	parts := strings.SplitN(string(raw), "\x00", 3)
	if len(parts) != 3 {
		return nil, invalid("the cursor is not one this server issued")
	}
	rank, err := strconv.ParseInt(parts[0], 10, 16)
	if err != nil {
		return nil, invalid("the cursor is not one this server issued")
	}
	return &cursor{rank: int16(rank), name: parts[1], id: parts[2]}, nil
}

func (r *PostgresRepository) Links(ctx context.Context, assetID, relation, after string, limit int) (*LinkPage, error) {
	c, err := decodeCursor(after)
	if err != nil {
		return nil, err
	}
	args := []any{assetID, relation, limit + 1}
	keyset := ""
	if c != nil {
		args = append(args, c.name, c.id)
		keyset = `AND (lower(COALESCE(x.name, '')), x.id) > ($4, $5)`
	}
	rows, err := r.db.Query(ctx, `
		SELECT l.relation, `+refSQL+`, lower(COALESCE(x.name, ''))
		FROM knowledge_links l JOIN assets x ON x.id = l.target_id LEFT JOIN knowledge_entries xk ON xk.asset_id = x.id
		WHERE l.entry_id = $1 AND ($2 = '' OR l.relation = $2) `+keyset+`
		ORDER BY lower(COALESCE(x.name, '')), x.id
		LIMIT $3`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing links: %w", err)
	}
	defer rows.Close()
	page := &LinkPage{Links: []Link{}}
	var lastName string
	for rows.Next() {
		var l Link
		var ref []byte
		if err := rows.Scan(&l.Relation, &ref, &lastName); err != nil {
			return nil, fmt.Errorf("reading link: %w", err)
		}
		if err := json.Unmarshal(ref, &l.Target); err != nil {
			return nil, fmt.Errorf("decoding link: %w", err)
		}
		page.Links = append(page.Links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(page.Links) > limit {
		last := page.Links[limit-1]
		page.Links = page.Links[:limit]
		page.Next = encodeCursor(cursor{name: strings.ToLower(last.Target.Name), id: last.Target.ID})
	}
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM knowledge_links WHERE entry_id = $1 AND ($2 = '' OR relation = $2)`, assetID, relation).Scan(&page.Total); err != nil {
		return nil, fmt.Errorf("counting links: %w", err)
	}
	return page, nil
}

// aboutSources is the candidate set of entries about asset $1: its own links, rules matching its tags, type or providers, and the links of what contains it, up to five levels up the CONTAINS edges plugins write. Each source is cut to its first $2 rows after the keyset when it is paged, so a page reads a few index pages per source whatever the totals. Filters are applied inside each source so the cut keeps only rows that can be shown.
func aboutSources(filters, keyset, limit string) string {
	return `
	target AS (SELECT id, mrn, type, tags, providers FROM assets WHERE id = $1),
	up AS (
		SELECT t.mrn, 0 AS depth FROM target t
		UNION ALL
		SELECT e.source_mrn, up.depth + 1 FROM lineage_edges e JOIN up ON e.target_mrn = up.mrn
		WHERE e.type = 'CONTAINS' AND up.depth < 5
	) CYCLE mrn SET is_cycle USING path,
	anc AS (
		SELECT a.id, min(up.depth) AS depth FROM up JOIN assets a ON a.mrn = up.mrn
		WHERE NOT up.is_cycle AND up.depth > 0 GROUP BY a.id
	),
	direct AS (
		SELECT l.entry_id, l.state_rank, l.name_key, l.relation, 'direct'::text AS source, NULL::text AS via, NULL::uuid AS rule_id, 0 AS depth
		FROM knowledge_links l WHERE l.target_id = $1 ` + filters + keyset + `
		ORDER BY l.state_rank, l.name_key, l.entry_id ` + limit + `
	),
	inherited AS (
		SELECT p.* FROM anc CROSS JOIN LATERAL (
			SELECT l.entry_id, l.state_rank, l.name_key, l.relation, 'inherited'::text AS source, anc.id::text AS via, NULL::uuid AS rule_id, anc.depth
			FROM knowledge_links l WHERE l.target_id = anc.id ` + filters + keyset + `
			ORDER BY l.state_rank, l.name_key, l.entry_id ` + limit + `
		) p
	),
	probes AS (
		SELECT 'tag'::text AS t, unnest(target.tags) AS v FROM target
		UNION ALL SELECT 'type', lower(target.type) FROM target
		UNION ALL SELECT 'provider', lower(unnest(target.providers)) FROM target
	),
	ruled AS (
		SELECT * FROM (
			SELECT DISTINCT ON (r.entry_id) r.entry_id, k.state_rank, lower(COALESCE(e.name, '')) AS name_key, r.relation, 'rule'::text AS source, NULL::text AS via, r.id AS rule_id, 0 AS depth, k.kind
			FROM probes s JOIN knowledge_rules r ON r.selector_type = s.t AND r.selector_value = s.v
			JOIN knowledge_entries k ON k.asset_id = r.entry_id JOIN assets e ON e.id = r.entry_id
			WHERE r.entry_id <> $1
			ORDER BY r.entry_id, r.created_at
		) l WHERE true ` + filters + keyset + `
	),
	candidates AS (
		SELECT DISTINCT ON (entry_id) * FROM (
			SELECT entry_id, state_rank, name_key, relation, source, via, rule_id, depth FROM direct
			UNION ALL SELECT entry_id, state_rank, name_key, relation, source, via, rule_id, depth FROM ruled
			UNION ALL SELECT entry_id, state_rank, name_key, relation, source, via, rule_id, depth FROM inherited
		) u
		ORDER BY entry_id, CASE source WHEN 'direct' THEN 0 WHEN 'rule' THEN 1 ELSE 2 END, depth
	)`
}

func (r *PostgresRepository) About(ctx context.Context, assetID string, f AboutFilter) (*AboutPage, error) {
	c, err := decodeCursor(f.Cursor)
	if err != nil {
		return nil, err
	}
	args := []any{assetID, f.Limit + 1}
	var filters []string
	if len(f.Kinds) > 0 {
		args = append(args, f.Kinds)
		filters = append(filters, fmt.Sprintf("l.kind = ANY($%d)", len(args)))
	}
	if len(f.States) > 0 {
		ranks := make([]int16, 0, len(f.States))
		for _, s := range f.States {
			if rk, ok := stateRanks[s]; ok {
				ranks = append(ranks, rk)
			}
		}
		args = append(args, ranks)
		filters = append(filters, fmt.Sprintf("l.state_rank = ANY($%d::smallint[])", len(args)))
	}
	if f.Relation != "" {
		args = append(args, f.Relation)
		filters = append(filters, fmt.Sprintf("l.relation = $%d", len(args)))
	}
	filterSQL := ""
	if len(filters) > 0 {
		filterSQL = "AND " + strings.Join(filters, " AND ") + " "
	}
	sourceSQL := "true"
	if len(f.Sources) > 0 {
		args = append(args, f.Sources)
		sourceSQL = fmt.Sprintf("c.source = ANY($%d)", len(args))
	}
	countArgs := append([]any{}, args...)

	keyset := ""
	if c != nil {
		args = append(args, c.rank, c.name, c.id)
		n := len(args)
		keyset = fmt.Sprintf("AND (l.state_rank, l.name_key, l.entry_id) > ($%d::smallint, $%d, $%d) ", n-2, n-1, n)
	}

	rows, err := r.db.Query(ctx, `WITH RECURSIVE `+aboutSources(filterSQL, keyset, "LIMIT $2")+`,
	page AS (SELECT * FROM candidates c WHERE `+sourceSQL+` ORDER BY state_rank, name_key, entry_id LIMIT $2)
	SELECT `+summaryColumns+`, page.source, page.relation, page.state_rank, page.name_key,
	       (SELECT `+refSQL+` FROM assets x LEFT JOIN knowledge_entries xk ON xk.asset_id = x.id WHERE x.id = page.via),
	       (SELECT json_build_object('id', r.id, 'relation', r.relation, 'selector_type', r.selector_type, 'selector_value', r.selector_value)
	        FROM knowledge_rules r WHERE r.id = page.rule_id)
	FROM page JOIN assets a ON a.id = page.entry_id LEFT JOIN knowledge_entries k ON k.asset_id = a.id
	ORDER BY page.state_rank, page.name_key, page.entry_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing entries about the asset: %w", err)
	}
	defer rows.Close()
	out := &AboutPage{Entries: []AboutItem{}, ByKind: map[string]int{}, BySource: map[string]int{}}
	now := time.Now()
	var last cursor
	for rows.Next() {
		var (
			sc        summaryScanner
			item      AboutItem
			via, rule []byte
			rank      int16
			nameKey   string
		)
		if err := rows.Scan(sc.targets(&item.Source, &item.Relation, &rank, &nameKey, &via, &rule)...); err != nil {
			return nil, fmt.Errorf("reading entry about the asset: %w", err)
		}
		s, err := sc.summary(now)
		if err != nil {
			return nil, err
		}
		item.Summary = s
		if len(via) > 0 {
			item.Via = &AssetRef{}
			if err := json.Unmarshal(via, item.Via); err != nil {
				return nil, fmt.Errorf("decoding via: %w", err)
			}
		}
		if len(rule) > 0 {
			item.Rule = &Rule{}
			if err := json.Unmarshal(rule, item.Rule); err != nil {
				return nil, fmt.Errorf("decoding rule: %w", err)
			}
		}
		out.Entries = append(out.Entries, item)
		if len(out.Entries) == f.Limit {
			last = cursor{rank: rank, name: nameKey, id: s.AssetID}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out.Entries) > f.Limit {
		out.Entries = out.Entries[:f.Limit]
		out.Next = encodeCursor(last)
	}

	// Totals cover every page, so the counts are taken over the whole candidate set rather than the cut one.
	countRows, err := r.db.Query(ctx, `WITH RECURSIVE `+aboutSources(filterSQL, "", "")+`
	SELECT ek.kind, c.source, COUNT(*) FROM candidates c JOIN knowledge_entries ek ON ek.asset_id = c.entry_id
	WHERE `+sourceSQL+` AND $2::int > 0 GROUP BY 1, 2`, countArgs...)
	if err != nil {
		return nil, fmt.Errorf("counting entries about the asset: %w", err)
	}
	defer countRows.Close()
	for countRows.Next() {
		var kind, source string
		var n int
		if err := countRows.Scan(&kind, &source, &n); err != nil {
			return nil, fmt.Errorf("reading counts: %w", err)
		}
		out.Total += n
		out.ByKind[kind] += n
		out.BySource[source] += n
	}
	return out, countRows.Err()
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
	c := &Counts{ByKind: map[string]int{}, ByState: map[string]int{}, ByTeam: []TeamCount{}}
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	teams, err := r.db.Query(ctx, `
		SELECT t.id::text, t.name, COUNT(*)
		FROM asset_owners ao JOIN teams t ON t.id = ao.team_id JOIN assets a ON a.id = ao.asset_id
		WHERE lower(a.type) = ANY($1) AND NOT a.is_stub
		GROUP BY 1, 2 ORDER BY 3 DESC, 2 LIMIT 50`, Types())
	if err != nil {
		return nil, fmt.Errorf("counting entries by team: %w", err)
	}
	defer teams.Close()
	for teams.Next() {
		var tc TeamCount
		if err := teams.Scan(&tc.ID, &tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("reading team counts: %w", err)
		}
		c.ByTeam = append(c.ByTeam, tc)
	}
	return c, teams.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
