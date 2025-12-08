package glossary

import (
	"context"
	"errors"
	"fmt"
	"time"

	validator "github.com/go-playground/validator/v10"
)

type Owner struct {
	ID             string  `json:"id"`
	Username       *string `json:"username,omitempty"` // Only for user owners
	Name           string  `json:"name"`
	Type           string  `json:"type"` // "user" or "team"
	Email          *string `json:"email,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
}

type GlossaryTerm struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Definition   string                 `json:"definition"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []Owner                `json:"owners"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	DeletedAt    *time.Time             `json:"deleted_at,omitempty"`
}

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
}

type UpdateTermInput struct {
	Name         *string                `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Definition   *string                `json:"definition,omitempty" validate:"omitempty,min=1"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerInput           `json:"owners,omitempty" validate:"omitempty,min=1,dive"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
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
}

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
}

type service struct {
	repo      Repository
	validator *validator.Validate
	metrics   MetricsClient
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

func (s *service) Create(ctx context.Context, input CreateTermInput) (*GlossaryTerm, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
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
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, term, input.Owners); err != nil {
		return nil, err
	}

	return s.Get(ctx, term.ID)
}

func (s *service) Get(ctx context.Context, id string) (*GlossaryTerm, error) {
	term, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if term.DeletedAt != nil {
		return nil, ErrTermNotFound
	}

	return term, nil
}

func (s *service) Update(ctx context.Context, id string, input UpdateTermInput) (*GlossaryTerm, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	existing, err := s.Get(ctx, id)
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

	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, existing, input.Owners); err != nil {
		return nil, err
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
	term, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	term.DeletedAt = &now
	term.UpdatedAt = now

	return s.repo.Update(ctx, term, nil)
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

	return s.repo.List(ctx, offset, limit)
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

	return s.repo.Search(ctx, filter)
}

func (s *service) GetChildren(ctx context.Context, parentID string) ([]*GlossaryTerm, error) {
	if _, err := s.Get(ctx, parentID); err != nil {
		return nil, err
	}

	return s.repo.GetChildren(ctx, parentID)
}

func (s *service) GetAncestors(ctx context.Context, termID string) ([]*GlossaryTerm, error) {
	term, err := s.Get(ctx, termID)
	if err != nil {
		return nil, err
	}

	var ancestors []*GlossaryTerm
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

	return ancestors, nil
}
