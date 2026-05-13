package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/search"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// SearchResults is the response from SearchService.Query.
type SearchResults = models.SearchResponse

// SearchOptions filters SearchService.Query.
type SearchOptions struct {
	Types  []string
	Limit  int64
	Offset int64
}

// SearchService performs unified search across assets, glossary terms, teams, and users.
type SearchService struct {
	gen *apiclient.Marmot
}

// Query runs a unified search.
func (s *SearchService) Query(ctx context.Context, q string, opts SearchOptions) (*SearchResults, error) {
	p := search.NewGetSearchParams().WithContext(ctx).WithQ(q)
	if len(opts.Types) > 0 {
		p = p.WithTypes(opts.Types)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Search.GetSearch(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
