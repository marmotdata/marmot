package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/marmotdata/marmot/internal/core/search"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Resolve finds the domains a reference names: an id, a name at any depth, or
// a slash-separated path of names from a root, ignoring case. Names may
// contain a slash themselves, so a path is also tried as a whole name.
func (s *service) Resolve(ctx context.Context, ref string) ([]*Domain, error) {
	ref = strings.TrimSpace(ref)
	if uuidPattern.MatchString(ref) {
		d, err := s.repo.Get(ctx, strings.ToLower(ref))
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []*Domain{d}, nil
	}
	found, err := s.repo.Named(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(ref, "/") {
		return found, nil
	}
	d, err := s.walk(ctx, strings.Split(ref, "/"))
	if err != nil || d == nil {
		return found, err
	}
	for _, f := range found {
		if f.ID == d.ID {
			return found, nil
		}
	}
	return append(found, d), nil
}

func (s *service) walk(ctx context.Context, names []string) (*Domain, error) {
	var current *Domain
	for _, name := range names {
		var parentID *string
		if current != nil {
			parentID = &current.ID
		}
		children, err := s.repo.Children(ctx, parentID)
		if err != nil {
			return nil, err
		}
		current = nil
		for _, c := range children {
			if strings.EqualFold(c.Name, strings.TrimSpace(name)) {
				current = c
				break
			}
		}
		if current == nil {
			return nil, nil
		}
	}
	return current, nil
}

// SearchResolver resolves the references of @domain search filters to subtree
// paths. References that name no domain are ignored unless all of them do.
// Unassigned also matches entities with no membership row.
func SearchResolver(svc Service) search.DomainResolver {
	return func(ctx context.Context, refs []string) (*search.DomainFilter, error) {
		filter := &search.DomainFilter{Paths: []string{}}
		seen := map[string]bool{}
		for _, ref := range refs {
			domains, err := svc.Resolve(ctx, ref)
			if err != nil {
				return nil, err
			}
			for _, d := range domains {
				switch {
				case d.ID == UnassignedID:
					filter.Unassigned = true
				case !seen[d.Path]:
					seen[d.Path] = true
					filter.Paths = append(filter.Paths, d.Path)
				}
			}
		}
		if len(filter.Paths) == 0 && !filter.Unassigned {
			return nil, search.ErrUnknownDomain
		}
		return filter, nil
	}
}
