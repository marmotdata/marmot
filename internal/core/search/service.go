package search

import (
	"context"
	"fmt"

	validator "github.com/go-playground/validator/v10"
)

type Service interface {
	Search(ctx context.Context, filter Filter) (*Response, error)
}

type service struct {
	repo      Repository
	validator *validator.Validate
}

func NewService(repo Repository) Service {
	return &service{
		repo:      repo,
		validator: validator.New(),
	}
}

func (s *service) Search(ctx context.Context, filter Filter) (*Response, error) {
	// Set defaults
	if filter.Limit <= 0 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// Validate filter
	if err := s.validator.Struct(filter); err != nil {
		return nil, fmt.Errorf("invalid search filter: %w", err)
	}

	// Execute search
	results, total, facets, err := s.repo.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}

	return &Response{
		Results: results,
		Total:   total,
		Facets:  facets,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}
