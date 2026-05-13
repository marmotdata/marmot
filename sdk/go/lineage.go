package marmot

import (
	"context"

	"github.com/go-openapi/strfmt"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/lineage"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// Lineage is the lineage graph for an asset.
type Lineage = models.LineageResponse

// LineageOptions controls LineageService.Get traversal.
type LineageOptions struct {
	// Direction is "upstream", "downstream" or "both" (the API default).
	Direction string
	Depth     int64
	Limit     int64
}

// LineageService fetches upstream / downstream lineage graphs.
type LineageService struct {
	gen *apiclient.Marmot
}

// Get returns the lineage graph for an asset.
func (s *LineageService) Get(ctx context.Context, assetID string, opts LineageOptions) (*Lineage, error) {
	p := lineage.NewGetLineageAssetsIDParams().WithContext(ctx).WithID(strfmt.UUID(assetID))
	if opts.Direction != "" {
		p = p.WithDirection(&opts.Direction)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	resp, err := s.gen.Lineage.GetLineageAssetsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
