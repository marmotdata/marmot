package tag

import (
	"context"
)

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetTag(ctx context.Context, id string) (*Tag, error) {
	return s.repo.GetTag(ctx, id)
}

func (s *service) ListTags(ctx context.Context) ([]Tag, error) {
	return s.repo.ListTags(ctx)
}

func (s *service) CreateTag(ctx context.Context, input CreateTagInput) (*Tag, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateTag(ctx, input)
}

func (s *service) UpdateTag(ctx context.Context, id string, input UpdateTagInput) (*Tag, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.UpdateTag(ctx, id, input)
}

func (s *service) DeleteTag(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}
	return s.repo.DeleteTag(ctx, id)
}

func (s *service) ResolveNames(ctx context.Context, names []string) ([]string, error) {
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" {
			filtered = append(filtered, name)
		}
	}
	if len(filtered) == 0 {
		return []string{}, nil
	}
	return s.repo.ResolveNames(ctx, filtered)
}
