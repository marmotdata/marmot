package role

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrSystemRoleProtected = errors.New("system role is protected")
	ErrRoleInUse           = errors.New("role is in use")
)

type Service interface {
	List(ctx context.Context) ([]*Role, error)
	Get(ctx context.Context, id string) (*Role, error)
	Create(ctx context.Context, input CreateInput) (*Role, error)
	Update(ctx context.Context, id string, input UpdateInput) (*Role, error)
	Delete(ctx context.Context, id string) error
	ReplacePermissions(ctx context.Context, roleID string, permIDs []string) error
	ListPermissions(ctx context.Context) ([]Permission, error)
}

type service struct {
	store Store
}

func NewService(store Store) Service {
	return &service{store: store}
}

func (s *service) List(ctx context.Context) ([]*Role, error) {
	return s.store.List(ctx, false)
}

func (s *service) Get(ctx context.Context, id string) (*Role, error) {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting role: %w", err)
	}
	return r, nil
}

func (s *service) Create(ctx context.Context, input CreateInput) (*Role, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	r, err := s.store.Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}
	return r, nil
}

func (s *service) Update(ctx context.Context, id string, input UpdateInput) (*Role, error) {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting role for update: %w", err)
	}

	if r.IsSystem {
		return nil, fmt.Errorf("%w: cannot modify system role %q", ErrSystemRoleProtected, r.Name)
	}

	updated, err := s.store.Update(ctx, id, input)
	if err != nil {
		return nil, fmt.Errorf("updating role: %w", err)
	}
	return updated, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting role for deletion: %w", err)
	}

	if r.IsSystem {
		return fmt.Errorf("%w: cannot delete system role %q", ErrSystemRoleProtected, r.Name)
	}

	hasUsers, err := s.store.HasUsers(ctx, id)
	if err != nil {
		return fmt.Errorf("checking role usage: %w", err)
	}
	if hasUsers {
		return fmt.Errorf("%w: role has active user assignments", ErrRoleInUse)
	}

	return s.store.SoftDelete(ctx, id)
}

func (s *service) ReplacePermissions(ctx context.Context, roleID string, permIDs []string) error {
	r, err := s.store.Get(ctx, roleID)
	if err != nil {
		return fmt.Errorf("getting role: %w", err)
	}

	if r.IsSystem {
		return fmt.Errorf("%w: cannot modify permissions of system role %q",
			ErrSystemRoleProtected, r.Name)
	}

	return s.store.ReplacePermissions(ctx, roleID, permIDs)
}

func (s *service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.store.ListPermissions(ctx)
}
