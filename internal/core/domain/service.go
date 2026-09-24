package domain

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Service interface {
	Create(ctx context.Context, in CreateInput) (*Domain, error)
	Get(ctx context.Context, id string) (*Domain, error)
	Children(ctx context.Context, parentID *string) ([]*Domain, error)
	Subtree(ctx context.Context, id string) ([]*Domain, error)
	Update(ctx context.Context, id string, in UpdateInput) (*Domain, error)
	Delete(ctx context.Context, id string) error
	Move(ctx context.Context, id string, parentID *string) (*Domain, error)
	Assign(ctx context.Context, kind Kind, entityID, domainID string) error
	DomainOf(ctx context.Context, kind Kind, entityID string) (string, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", fmt.Errorf("%w: name exceeds %d characters", ErrInvalidInput, maxNameLength)
	}
	return name, nil
}

// Unassigned is a holding bucket, not a branch of the tree.
func checkParent(parentID *string) error {
	if parentID != nil && *parentID == UnassignedID {
		return fmt.Errorf("%w: it cannot have subdomains", ErrProtected)
	}
	return nil
}

func (s *service) Create(ctx context.Context, in CreateInput) (*Domain, error) {
	name, err := normalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	if err := checkParent(in.ParentID); err != nil {
		return nil, err
	}
	in.Name = name
	return s.repo.Create(ctx, in)
}

func (s *service) Get(ctx context.Context, id string) (*Domain, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) Children(ctx context.Context, parentID *string) ([]*Domain, error) {
	return s.repo.Children(ctx, parentID)
}

func (s *service) Subtree(ctx context.Context, id string) ([]*Domain, error) {
	return s.repo.Subtree(ctx, id)
}

func (s *service) Update(ctx context.Context, id string, in UpdateInput) (*Domain, error) {
	if in.Name != nil {
		if id == UnassignedID {
			return nil, ErrProtected
		}
		name, err := normalizeName(*in.Name)
		if err != nil {
			return nil, err
		}
		in.Name = &name
	}
	return s.repo.Update(ctx, id, in)
}

func (s *service) Delete(ctx context.Context, id string) error {
	if id == UnassignedID {
		return ErrProtected
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) Move(ctx context.Context, id string, parentID *string) (*Domain, error) {
	if id == UnassignedID {
		return nil, ErrProtected
	}
	if err := checkParent(parentID); err != nil {
		return nil, err
	}
	return s.repo.Move(ctx, id, parentID)
}

func (s *service) Assign(ctx context.Context, kind Kind, entityID, domainID string) error {
	if !kind.valid() {
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidInput, kind)
	}
	if strings.TrimSpace(entityID) == "" {
		return fmt.Errorf("%w: entity id is required", ErrInvalidInput)
	}
	if domainID != UnassignedID {
		if _, err := s.repo.Get(ctx, domainID); err != nil {
			return err
		}
	}
	return s.repo.Assign(ctx, kind, entityID, domainID)
}

func (s *service) DomainOf(ctx context.Context, kind Kind, entityID string) (string, error) {
	if !kind.valid() {
		return "", fmt.Errorf("%w: unknown kind %q", ErrInvalidInput, kind)
	}
	return s.repo.DomainOf(ctx, kind, entityID)
}
