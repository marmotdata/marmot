package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/owners"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// OwnerSearchResults is the response from OwnersService.Search.
type OwnerSearchResults = models.V1TeamsSearchOwnersResponse

// OwnerSearchOptions filters OwnersService.Search.
type OwnerSearchOptions struct {
	Limit int64
}

// OwnersService searches the catalog for asset owners (users and teams).
type OwnersService struct {
	gen *apiclient.Marmot
}

// Search returns owners matching q.
func (s *OwnersService) Search(ctx context.Context, q string, opts OwnerSearchOptions) (*OwnerSearchResults, error) {
	p := owners.NewGetOwnersSearchParams().WithContext(ctx).WithQ(q)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	resp, err := s.gen.Owners.GetOwnersSearch(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
