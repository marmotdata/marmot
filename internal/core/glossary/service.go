package glossary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/marmotdata/marmot/internal/core/limits"
)

type Owner struct {
	ID             string  `json:"id"`
	Username       *string `json:"username,omitempty"` // Only for user owners
	Name           string  `json:"name"`
	Type           string  `json:"type"` // "user" or "team"
	Email          *string `json:"email,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
} // @name GlossaryOwner

type GlossaryTerm struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Definition is what the last run wrote. On a term a person has
	// worded themselves the read path serves UserDefinition here instead,
	// so a caller always gets the wording the catalog stands behind.
	Definition string `json:"definition"`
	// UserDefinition is the wording a person gave the term. Ingestion
	// reads it and never writes it, so it survives every run.
	UserDefinition *string                `json:"user_definition,omitempty"`
	Description    *string                `json:"description,omitempty"`
	ParentTermID   *string                `json:"parent_term_id,omitempty"`
	Owners         []Owner                `json:"owners"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	// CreatedBy is the person who wrote the term, absent for terms an ingestion brought in and for those made before authors were recorded.
	CreatedBy     *string    `json:"created_by,omitempty"`
	CreatedByName *string    `json:"created_by_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	// LinkedAssetCount counts every asset linked to the term, by hand or by an asset rule, so a tree can show it without a request per term.
	LinkedAssetCount int `json:"linked_asset_count"`
	// RelatedTerms is only filled in when a single term is read.
	RelatedTerms []RelatedTerm `json:"related_terms,omitempty"`
} // @name GlossaryTerm

// RelatedTerm is a term that sits beside another rather than above or below it.
type RelatedTerm struct {
	ID   string `json:"id"`
	Name string `json:"name"`
} // @name GlossaryRelatedTerm

// SuggestedAsset is an asset that looks like it belongs to a term but is not linked to it yet.
type SuggestedAsset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MRN         string   `json:"mrn"`
	Type        string   `json:"type"`
	Providers   []string `json:"providers"`
	Description *string  `json:"description,omitempty"`
} // @name GlossarySuggestedAsset

type OwnerInput struct {
	ID   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required,oneof=user team"`
}

type CreateTermInput struct {
	Name         string                 `json:"name" validate:"required,min=1,max=255"`
	Definition   string                 `json:"definition" validate:"required,min=1"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerInput           `json:"owners" validate:"required,min=1,dive"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	// RelatedTermIDs links the new term to existing ones.
	RelatedTermIDs []string `json:"related_term_ids,omitempty"`
	// CreatedBy is the user writing the term, left empty for machine identities since the column refers to users.
	CreatedBy *string `json:"-"`
}

type UpdateTermInput struct {
	Name         *string                `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Definition   *string                `json:"definition,omitempty" validate:"omitempty,min=1"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerInput           `json:"owners,omitempty" validate:"omitempty,min=1,dive"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	// RelatedTermIDs replaces the term's related terms when set; nil leaves them as they are and an empty list clears them.
	RelatedTermIDs []string `json:"related_term_ids,omitempty"`
}

type SearchFilter struct {
	Query        string   `json:"query,omitempty"`
	ParentTermID *string  `json:"parent_term_id,omitempty"`
	OwnerIDs     []string `json:"owner_ids,omitempty"`
	Limit        int      `json:"limit,omitempty" validate:"omitempty,gte=0,lte=100"`
	Offset       int      `json:"offset,omitempty" validate:"omitempty,gte=0"`
}

type ListResult struct {
	Terms []*GlossaryTerm `json:"terms"`
	Total int             `json:"total"`
} // @name GlossaryListResult

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrTermNotFound = errors.New("glossary term not found")
	ErrTermExists   = errors.New("glossary term already exists")
	ErrCircularRef  = errors.New("circular reference detected in term hierarchy")
)

type Service interface {
	Create(ctx context.Context, input CreateTermInput) (*GlossaryTerm, error)
	Get(ctx context.Context, id string) (*GlossaryTerm, error)
	Update(ctx context.Context, id string, input UpdateTermInput) (*GlossaryTerm, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter) (*ListResult, error)
	GetChildren(ctx context.Context, parentID string) ([]*GlossaryTerm, error)
	GetAncestors(ctx context.Context, termID string) ([]*GlossaryTerm, error)
	GetRelatedTerms(ctx context.Context, termID string) ([]RelatedTerm, error)
	AddRelatedTerm(ctx context.Context, termID, relatedTermID string) ([]RelatedTerm, error)
	RemoveRelatedTerm(ctx context.Context, termID, relatedTermID string) ([]RelatedTerm, error)
	SuggestAssets(ctx context.Context, termID string, limit int) ([]SuggestedAsset, error)
	SyncTerms(ctx context.Context, source string, inputs []TermInput) (*SyncResult, error)
	SetSearchObserver(observer SearchObserver)
}

// SearchObserver is notified when glossary terms change.
type SearchObserver interface {
	OnEntityChanged(ctx context.Context, entityType, entityID string)
	OnEntityDeleted(ctx context.Context, entityType, entityID string)
}

type service struct {
	repo           Repository
	validator      *validator.Validate
	metrics        MetricsClient
	guard          limits.Guard
	searchObserver SearchObserver
}

type MetricsClient interface {
	Count(name string, value int64, tags ...string)
	Timing(name string, value time.Duration, tags ...string)
}

type ServiceOption func(*service)

func NewService(repo Repository, opts ...ServiceOption) Service {
	s := &service{
		repo:      repo,
		validator: validator.New(),
		guard:     limits.NoopGuard{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func WithMetrics(metrics MetricsClient) ServiceOption {
	return func(s *service) {
		s.metrics = metrics
	}
}

// WithGuard installs a resource-count Guard consulted before each Create.
// Passing NoopGuard (the default) disables enforcement.
func WithGuard(g limits.Guard) ServiceOption {
	return func(s *service) {
		if g != nil {
			s.guard = g
		}
	}
}

// SetSearchObserver registers an observer for search index sync.
func (s *service) SetSearchObserver(observer SearchObserver) {
	s.searchObserver = observer
}

func (s *service) Create(ctx context.Context, input CreateTermInput) (*GlossaryTerm, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if err := s.guard.CheckCreate(ctx, limits.ResourceGlossaryTerms); err != nil {
		return nil, err
	}

	if input.ParentTermID != nil && *input.ParentTermID != "" {
		parent, err := s.repo.Get(ctx, *input.ParentTermID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("%w: parent term not found", ErrInvalidInput)
			}
			return nil, fmt.Errorf("getting parent term: %w", err)
		}
		if parent.DeletedAt != nil {
			return nil, fmt.Errorf("%w: parent term is deleted", ErrInvalidInput)
		}
	}

	term := &GlossaryTerm{
		Name:         input.Name,
		Definition:   input.Definition,
		Description:  input.Description,
		ParentTermID: input.ParentTermID,
		Metadata:     input.Metadata,
		Tags:         input.Tags,
		CreatedBy:    input.CreatedBy,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	related, err := s.checkRelated(ctx, "", input.RelatedTermIDs)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, term, input.Owners); err != nil {
		return nil, err
	}

	if len(related) > 0 {
		if err := s.repo.SetRelatedTerms(ctx, term.ID, related); err != nil {
			return nil, err
		}
	}

	if s.searchObserver != nil {
		s.searchObserver.OnEntityChanged(ctx, "glossary", term.ID)
	}

	return s.Get(ctx, term.ID)
}

func (s *service) Get(ctx context.Context, id string) (*GlossaryTerm, error) {
	term, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}

	related, err := s.repo.GetRelatedTerms(ctx, id)
	if err != nil {
		return nil, err
	}

	term.RelatedTerms = related
	return asRead(term), nil
}

// load fetches a term as it is stored, for the paths that write it back.
// Serving a term goes through Get instead, which resolves the wording.
func (s *service) load(ctx context.Context, id string) (*GlossaryTerm, error) {
	term, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if term.DeletedAt != nil {
		return nil, ErrTermNotFound
	}

	return term, nil
}

// asRead returns the term a reader should see: a definition a person
// wrote wins over the one the last run brought in.
//
// This is the one place the preference lives, and it sits between the
// repository and every caller that serves a term, so the columns stay
// what they are on the way in. Doing it in SQL instead would feed the
// resolved wording back to the sync, which compares what it read against
// what the source says, and to Update and Delete, which write the row
// back whole: a person's wording would end up in the definition column
// and be lost on the next run.
func asRead(term *GlossaryTerm) *GlossaryTerm {
	if term == nil || term.UserDefinition == nil || strings.TrimSpace(*term.UserDefinition) == "" {
		return term
	}

	resolved := *term
	resolved.Definition = *term.UserDefinition
	return &resolved
}

// allAsRead resolves a page of terms in place, since the slice was built
// for this caller.
func allAsRead(terms []*GlossaryTerm) []*GlossaryTerm {
	for i, term := range terms {
		terms[i] = asRead(term)
	}
	return terms
}

func (s *service) Update(ctx context.Context, id string, input UpdateTermInput) (*GlossaryTerm, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	existing, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Definition != nil {
		existing.Definition = *input.Definition
	}
	if input.Description != nil {
		existing.Description = input.Description
	}
	if input.Metadata != nil {
		existing.Metadata = input.Metadata
	}
	if input.Tags != nil {
		existing.Tags = input.Tags
	}

	if input.ParentTermID != nil {
		if *input.ParentTermID == "" {
			existing.ParentTermID = nil
		} else {
			if *input.ParentTermID == id {
				return nil, fmt.Errorf("%w: term cannot be its own parent", ErrCircularRef)
			}

			parent, err := s.repo.Get(ctx, *input.ParentTermID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return nil, fmt.Errorf("%w: parent term not found", ErrInvalidInput)
				}
				return nil, fmt.Errorf("getting parent term: %w", err)
			}
			if parent.DeletedAt != nil {
				return nil, fmt.Errorf("%w: parent term is deleted", ErrInvalidInput)
			}

			if err := s.checkCircularReference(ctx, id, *input.ParentTermID); err != nil {
				return nil, err
			}

			existing.ParentTermID = input.ParentTermID
		}
	}

	var related []string
	if input.RelatedTermIDs != nil {
		related, err = s.checkRelated(ctx, id, input.RelatedTermIDs)
		if err != nil {
			return nil, err
		}
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, existing, input.Owners); err != nil {
		return nil, err
	}

	if input.RelatedTermIDs != nil {
		if err := s.repo.SetRelatedTerms(ctx, id, related); err != nil {
			return nil, err
		}
	}

	if s.searchObserver != nil {
		s.searchObserver.OnEntityChanged(ctx, "glossary", id)
	}

	return s.Get(ctx, id)
}

func (s *service) checkCircularReference(ctx context.Context, termID, newParentID string) error {
	ancestors, err := s.GetAncestors(ctx, newParentID)
	if err != nil {
		return fmt.Errorf("getting ancestors: %w", err)
	}

	for _, ancestor := range ancestors {
		if ancestor.ID == termID {
			return ErrCircularRef
		}
	}

	return nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	term, err := s.load(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	term.DeletedAt = &now
	term.UpdatedAt = now

	if err := s.repo.Update(ctx, term, nil); err != nil {
		return err
	}

	if s.searchObserver != nil {
		s.searchObserver.OnEntityDeleted(ctx, "glossary", id)
	}

	return nil
}

func (s *service) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	result, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	result.Terms = allAsRead(result.Terms)
	return result, nil
}

func (s *service) Search(ctx context.Context, filter SearchFilter) (*ListResult, error) {
	if err := s.validator.Struct(filter); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	result, err := s.repo.Search(ctx, filter)
	if err != nil {
		return nil, err
	}

	result.Terms = allAsRead(result.Terms)
	return result, nil
}

func (s *service) GetChildren(ctx context.Context, parentID string) ([]*GlossaryTerm, error) {
	if _, err := s.load(ctx, parentID); err != nil {
		return nil, err
	}

	children, err := s.repo.GetChildren(ctx, parentID)
	if err != nil {
		return nil, err
	}

	return allAsRead(children), nil
}

func (s *service) GetAncestors(ctx context.Context, termID string) ([]*GlossaryTerm, error) {
	term, err := s.load(ctx, termID)
	if err != nil {
		return nil, err
	}

	ancestors := []*GlossaryTerm{}
	current := term

	for current.ParentTermID != nil && *current.ParentTermID != "" {
		parent, err := s.repo.Get(ctx, *current.ParentTermID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				break
			}
			return nil, fmt.Errorf("getting parent term: %w", err)
		}

		if parent.DeletedAt != nil {
			break
		}

		ancestors = append([]*GlossaryTerm{parent}, ancestors...)
		current = parent

		if len(ancestors) > 100 {
			return nil, fmt.Errorf("hierarchy depth exceeded")
		}
	}

	return allAsRead(ancestors), nil
}

// checkRelated validates the terms a term is to be related to and drops duplicates, so a bad id is a 400 rather than a foreign key failure.
func (s *service) checkRelated(ctx context.Context, termID string, ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if id == termID {
			return nil, fmt.Errorf("%w: a term cannot be related to itself", ErrInvalidInput)
		}
		related, err := s.repo.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("%w: related term not found", ErrInvalidInput)
			}
			return nil, fmt.Errorf("getting related term: %w", err)
		}
		if related.DeletedAt != nil {
			return nil, fmt.Errorf("%w: related term is deleted", ErrInvalidInput)
		}
		out = append(out, id)
	}
	return out, nil
}

func (s *service) GetRelatedTerms(ctx context.Context, termID string) ([]RelatedTerm, error) {
	if _, err := s.load(ctx, termID); err != nil {
		return nil, err
	}
	return s.repo.GetRelatedTerms(ctx, termID)
}

func (s *service) AddRelatedTerm(ctx context.Context, termID, relatedTermID string) ([]RelatedTerm, error) {
	if _, err := s.load(ctx, termID); err != nil {
		return nil, err
	}
	ids, err := s.checkRelated(ctx, termID, []string{relatedTermID})
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: related term id required", ErrInvalidInput)
	}
	if err := s.repo.AddRelatedTerm(ctx, termID, ids[0]); err != nil {
		return nil, err
	}
	return s.repo.GetRelatedTerms(ctx, termID)
}

func (s *service) RemoveRelatedTerm(ctx context.Context, termID, relatedTermID string) ([]RelatedTerm, error) {
	if _, err := s.load(ctx, termID); err != nil {
		return nil, err
	}
	if err := s.repo.RemoveRelatedTerm(ctx, termID, relatedTermID); err != nil {
		return nil, err
	}
	return s.repo.GetRelatedTerms(ctx, termID)
}

// SuggestAssets offers assets that mention the term but are not linked to it, for the link drawer to show before anyone searches.
func (s *service) SuggestAssets(ctx context.Context, termID string, limit int) ([]SuggestedAsset, error) {
	term, err := s.load(ctx, termID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	words := suggestionWords(term.Name)
	if len(words) == 0 {
		return []SuggestedAsset{}, nil
	}
	return s.repo.SuggestAssets(ctx, termID, words, limit)
}

// suggestionWords splits a term name into the words worth matching on, trimming a plural s so "Refunds" still finds a refund table.
func suggestionWords(name string) []string {
	fields := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	seen := map[string]struct{}{}
	words := []string{}
	for _, f := range fields {
		if len(f) < 3 || stopWords[f] {
			continue
		}
		if len(f) > 4 && strings.HasSuffix(f, "s") && !strings.HasSuffix(f, "ss") {
			f = strings.TrimSuffix(f, "s")
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		words = append(words, f)
	}
	return words
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "per": true, "with": true, "from": true, "all": true, "any": true,
}
