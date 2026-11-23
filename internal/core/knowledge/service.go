package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrUnsaved is returned when marking an entry reviewed that has never been saved, since there is nothing yet to vouch for.
var ErrUnsaved = errors.New("knowledge entry has not been saved")

// SaveInput is everything a person sets on an entry. It replaces what was there, so a field left out is cleared.
type SaveInput struct {
	Status        string         `json:"status"`
	Fields        map[string]any `json:"fields"`
	EffectiveFrom *string        `json:"effective_from,omitempty"`
	ReviewBy      *string        `json:"review_by,omitempty"`
	// Supersedes is the asset ID of the entry this one replaces. It must be the same kind, and is marked superseded once this entry is current.
	Supersedes *string `json:"supersedes,omitempty"`
	// Links and Rules replace the entry's links and rules; leave both out to keep what is there. Only changes are written, so resending thousands of unchanged links is cheap.
	Links *[]LinkInput `json:"links,omitempty"`
	Rules *[]RuleInput `json:"rules,omitempty"`
} // @name KnowledgeSaveInput

// LinkInput links the entry to an asset. Relation defaults to applies_to.
type LinkInput struct {
	Relation string `json:"relation,omitempty"`
	TargetID string `json:"target_id"`
} // @name KnowledgeLinkInput

// RuleInput links the entry to every asset with a tag, of a type, or from a provider. Relation defaults to applies_to.
type RuleInput struct {
	Relation      string `json:"relation,omitempty"`
	SelectorType  string `json:"selector_type"`
	SelectorValue string `json:"selector_value"`
} // @name KnowledgeRuleInput

const (
	maxLinksPerSave = 5000
	maxRules        = 50
)

type Service interface {
	Get(ctx context.Context, assetID string) (*Entry, error)
	Save(ctx context.Context, assetID string, in SaveInput, updatedBy string) (*Entry, error)
	MarkReviewed(ctx context.Context, assetID string, nextReviewBy *string, by string) (*Entry, error)
	List(ctx context.Context, f ListFilter) ([]Summary, int, error)
	Links(ctx context.Context, assetID, relation, cursor string, limit int) (*LinkPage, error)
	About(ctx context.Context, assetID string, f AboutFilter) (*AboutPage, error)
	Counts(ctx context.Context) (*Counts, error)
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

// stateOn is what a status means on a given day: a current entry whose effective date is still ahead is pending.
func stateOn(k Kind, status string, effectiveFrom *string, now time.Time) State {
	st := k.StateOf(status)
	if st == StateCurrent && effectiveFrom != nil && *effectiveFrom > now.Format(dateLayout) {
		return StatePending
	}
	return st
}

func (s *service) kindOf(ctx context.Context, assetID string) (Kind, error) {
	t, err := s.repo.AssetType(ctx, assetID)
	if err != nil {
		return Kind{}, err
	}
	k, ok := KindOfType(t)
	if !ok {
		return Kind{}, ErrNotFound
	}
	return k, nil
}

func (s *service) Get(ctx context.Context, assetID string) (*Entry, error) {
	k, err := s.kindOf(ctx, assetID)
	if err != nil {
		return nil, err
	}
	e, err := s.repo.Get(ctx, assetID)
	if err != nil {
		return nil, err
	}
	// The asset's type decides the kind, so an entry whose type was changed reads as its new kind.
	e.Kind = k.ID
	e.State = stateOn(k, e.Status, e.EffectiveFrom, s.now())
	return e, nil
}

func (s *service) Save(ctx context.Context, assetID string, in SaveInput, updatedBy string) (*Entry, error) {
	k, err := s.kindOf(ctx, assetID)
	if err != nil {
		return nil, err
	}
	status, ok := k.status(in.Status)
	if !ok {
		values := make([]string, len(k.Statuses))
		for i, st := range k.Statuses {
			values[i] = st.Value
		}
		return nil, invalid("status must be one of %s", strings.Join(values, ", "))
	}
	fields, err := cleanFields(k, status.Value, in.Fields)
	if err != nil {
		return nil, err
	}
	effectiveFrom, err := cleanDate("In force from", in.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	reviewBy, err := cleanDate("Review by", in.ReviewBy)
	if err != nil {
		return nil, err
	}

	var supersedes *string
	if in.Supersedes != nil && strings.TrimSpace(*in.Supersedes) != "" {
		id := strings.TrimSpace(*in.Supersedes)
		if id == assetID {
			return nil, invalid("an entry cannot replace itself")
		}
		supersedes = &id
	}

	links, err := s.cleanLinks(ctx, assetID, in.Links)
	if err != nil {
		return nil, err
	}
	rules, err := cleanRules(in.Rules)
	if err != nil {
		return nil, err
	}
	if supersedes != nil {
		found, err := s.repo.AssetTypes(ctx, []string{*supersedes})
		if err != nil {
			return nil, err
		}
		t, ok := found[*supersedes]
		if !ok {
			return nil, invalid("no asset has the id %q", *supersedes)
		}
		if other, _ := KindOfType(t); other.ID != k.ID {
			return nil, invalid("a %s can only replace another %s", strings.ToLower(k.Label), strings.ToLower(k.Label))
		}
	}

	rec := record{
		kind:          k.ID,
		status:        status.Value,
		fields:        fields,
		effectiveFrom: effectiveFrom,
		reviewBy:      reviewBy,
		supersedes:    supersedes,
		links:         links,
		rules:         rules,
		updatedBy:     updatedBy,
	}
	if supersedes != nil && stateOn(k, status.Value, effectiveFrom, s.now()) == StateCurrent {
		rec.retire = supersedes
	}
	if err := s.repo.Save(ctx, assetID, rec); err != nil {
		return nil, err
	}
	return s.Get(ctx, assetID)
}

func (s *service) MarkReviewed(ctx context.Context, assetID string, nextReviewBy *string, by string) (*Entry, error) {
	if _, err := s.kindOf(ctx, assetID); err != nil {
		return nil, err
	}
	next, err := cleanDate("Next review", nextReviewBy)
	if err != nil {
		return nil, err
	}
	if next != nil && *next <= s.now().Format(dateLayout) {
		return nil, invalid("the next review must be after today")
	}
	if err := s.repo.MarkReviewed(ctx, assetID, next, by); err != nil {
		if errors.Is(err, ErrUnsaved) {
			return nil, invalid("save the entry with a status before marking it reviewed")
		}
		return nil, err
	}
	return s.Get(ctx, assetID)
}

func (s *service) List(ctx context.Context, f ListFilter) ([]Summary, int, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.List(ctx, f)
}

func (s *service) Links(ctx context.Context, assetID, relation, cursor string, limit int) (*LinkPage, error) {
	if _, err := s.kindOf(ctx, assetID); err != nil {
		return nil, err
	}
	if relation != "" {
		if _, ok := relationByID(relation); !ok {
			return nil, invalid("unknown relation %q", relation)
		}
	}
	return s.repo.Links(ctx, assetID, relation, cursor, clampLimit(limit))
}

// About lists the entries about any asset, knowledge entries included: linked to it, matched to it by a rule, or linked to something that contains it.
func (s *service) About(ctx context.Context, assetID string, f AboutFilter) (*AboutPage, error) {
	if _, err := s.repo.AssetType(ctx, assetID); err != nil {
		return nil, err
	}
	for _, k := range f.Kinds {
		if _, ok := KindByID(k); !ok {
			return nil, invalid("unknown kind %q", k)
		}
	}
	for _, src := range f.Sources {
		if src != SourceDirect && src != SourceRule && src != SourceInherited {
			return nil, invalid("source must be direct, rule or inherited")
		}
	}
	if f.Relation != "" {
		if _, ok := relationByID(f.Relation); !ok {
			return nil, invalid("unknown relation %q", f.Relation)
		}
	}
	f.Limit = clampLimit(f.Limit)
	return s.repo.About(ctx, assetID, f)
}

func clampLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}

// cleanLinks checks and de-duplicates links. Nil leaves the entry's links as they are.
func (s *service) cleanLinks(ctx context.Context, assetID string, in *[]LinkInput) (*[]linkKey, error) {
	if in == nil {
		return nil, nil
	}
	if len(*in) > maxLinksPerSave {
		return nil, invalid("an entry can be saved with at most %d links; use a rule to cover more", maxLinksPerSave)
	}
	seen := map[linkKey]bool{}
	out := make([]linkKey, 0, len(*in))
	ids := make([]string, 0, len(*in))
	for _, l := range *in {
		rel := strings.TrimSpace(l.Relation)
		if rel == "" {
			rel = RelationAppliesTo
		}
		if _, ok := relationByID(rel); !ok {
			return nil, invalid("unknown relation %q", rel)
		}
		id := strings.TrimSpace(l.TargetID)
		if id == "" || id == assetID {
			continue
		}
		key := linkKey{relation: rel, target: id}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		found, err := s.repo.AssetTypes(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if _, ok := found[id]; !ok {
				return nil, invalid("no asset has the id %q", id)
			}
		}
	}
	return &out, nil
}

// cleanRules checks rules and stores type and provider values lowercase, as they are matched. Nil leaves the entry's rules as they are.
func cleanRules(in *[]RuleInput) (*[]RuleInput, error) {
	if in == nil {
		return nil, nil
	}
	if len(*in) > maxRules {
		return nil, invalid("an entry can have at most %d rules", maxRules)
	}
	seen := map[RuleInput]bool{}
	out := make([]RuleInput, 0, len(*in))
	for _, r := range *in {
		r.Relation = strings.TrimSpace(r.Relation)
		if r.Relation == "" {
			r.Relation = RelationAppliesTo
		}
		if _, ok := relationByID(r.Relation); !ok {
			return nil, invalid("unknown relation %q", r.Relation)
		}
		r.SelectorValue = strings.TrimSpace(r.SelectorValue)
		switch r.SelectorType {
		case SelectorTag:
		case SelectorType, SelectorProvider:
			r.SelectorValue = strings.ToLower(r.SelectorValue)
		default:
			return nil, invalid("a rule matches by tag, type or provider")
		}
		if r.SelectorValue == "" {
			return nil, invalid("a rule needs a %s to match", r.SelectorType)
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return &out, nil
}

func (s *service) Counts(ctx context.Context) (*Counts, error) {
	return s.repo.Counts(ctx)
}
