package domain

import (
	"context"
	"errors"

	"github.com/marmotdata/marmot/internal/core/search"
)

// SearchResolver resolves the ids of an @domain search filter to subtree
// paths. Selecting Unassigned also matches entities with no membership row.
func SearchResolver(svc Service) search.DomainResolver {
	return func(ctx context.Context, ids []string) (*search.DomainFilter, error) {
		filter := &search.DomainFilter{Paths: []string{}}
		for _, id := range ids {
			if id == UnassignedID {
				filter.Unassigned = true
				continue
			}
			d, err := svc.Get(ctx, id)
			if errors.Is(err, ErrNotFound) {
				return nil, search.ErrUnknownDomain
			}
			if err != nil {
				return nil, err
			}
			filter.Paths = append(filter.Paths, d.Path)
		}
		return filter, nil
	}
}
