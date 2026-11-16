package knowledge

import (
	"context"
	"errors"
	"slices"
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
	Supersedes *string  `json:"supersedes,omitempty"`
	AppliesTo  []string `json:"applies_to,omitempty"`
} // @name KnowledgeSaveInput

type Service interface {
	Get(ctx context.Context, assetID string) (*Entry, error)
	Save(ctx context.Context, assetID string, in SaveInput, updatedBy string) (*Entry, error)
	MarkReviewed(ctx context.Context, assetID string, nextReviewBy *string, by string) (*Entry, error)
	List(ctx context.Context, f ListFilter) ([]Summary, int, error)
	ApplyingTo(ctx context.Context, assetID string) ([]Summary, error)
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

	appliesTo := make([]string, 0, len(in.AppliesTo))
	for _, id := range in.AppliesTo {
		id = strings.TrimSpace(id)
		if id == "" || id == assetID || slices.Contains(appliesTo, id) {
			continue
		}
		appliesTo = append(appliesTo, id)
	}
	if len(appliesTo) > maxLinks {
		return nil, invalid("an entry can apply to at most %d assets", maxLinks)
	}

	var supersedes *string
	if in.Supersedes != nil && strings.TrimSpace(*in.Supersedes) != "" {
		id := strings.TrimSpace(*in.Supersedes)
		if id == assetID {
			return nil, invalid("an entry cannot replace itself")
		}
		supersedes = &id
	}

	check := slices.Clone(appliesTo)
	if supersedes != nil {
		check = append(check, *supersedes)
	}
	if len(check) > 0 {
		found, err := s.repo.AssetTypes(ctx, check)
		if err != nil {
			return nil, err
		}
		for _, id := range check {
			if _, ok := found[id]; !ok {
				return nil, invalid("no asset has the id %q", id)
			}
		}
		if supersedes != nil {
			if other, _ := KindOfType(found[*supersedes]); other.ID != k.ID {
				return nil, invalid("a %s can only replace another %s", strings.ToLower(k.Label), strings.ToLower(k.Label))
			}
		}
	}

	rec := record{
		kind:          k.ID,
		status:        status.Value,
		fields:        fields,
		effectiveFrom: effectiveFrom,
		reviewBy:      reviewBy,
		supersedes:    supersedes,
		appliesTo:     appliesTo,
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

func (s *service) ApplyingTo(ctx context.Context, assetID string) ([]Summary, error) {
	return s.repo.ApplyingTo(ctx, assetID)
}

func (s *service) Counts(ctx context.Context) (*Counts, error) {
	return s.repo.Counts(ctx)
}
