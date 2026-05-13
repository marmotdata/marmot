package marmot

import (
	"context"

	"github.com/go-openapi/strfmt"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/lineage"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// DefaultEdgeType is the lineage edge type assigned when WriteEdgeInput
// leaves Type empty.
const DefaultEdgeType = "DIRECT"

// Lineage is the lineage graph for an asset.
type Lineage = models.LineageResponse

// LineageEdge is one edge in the lineage graph.
type LineageEdge = models.LineageEdge

// BatchLineageResult is the per-edge outcome from LineageService.Batch.
type BatchLineageResult = models.BatchLineageResult

// LineageOptions controls LineageService.Get traversal.
type LineageOptions struct {
	// Direction is "upstream", "downstream" or "both" (the API default).
	Direction string
	Depth     int64
	Limit     int64
}

// WriteEdgeInput is the input for LineageService.Write.
type WriteEdgeInput struct {
	Source string
	Target string
	// Type defaults to DefaultEdgeType ("DIRECT") when empty.
	Type   string
	JobMrn string
}

// LineageService reads and writes lineage edges.
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

// Upstream is a convenience for Get with Direction="upstream".
func (s *LineageService) Upstream(ctx context.Context, assetID string, opts LineageOptions) (*Lineage, error) {
	opts.Direction = "upstream"
	return s.Get(ctx, assetID, opts)
}

// Downstream is a convenience for Get with Direction="downstream".
func (s *LineageService) Downstream(ctx context.Context, assetID string, opts LineageOptions) (*Lineage, error) {
	opts.Direction = "downstream"
	return s.Get(ctx, assetID, opts)
}

// Write creates a single lineage edge from in.Source to in.Target.
func (s *LineageService) Write(ctx context.Context, in WriteEdgeInput) (*LineageEdge, error) {
	edge := &models.LineageEdge{
		Source: in.Source,
		Target: in.Target,
		Type:   in.Type,
		JobMrn: in.JobMrn,
	}
	if edge.Type == "" {
		edge.Type = DefaultEdgeType
	}
	p := lineage.NewPostLineageDirectParams().WithContext(ctx).WithEdge(edge)
	resp, err := s.gen.Lineage.PostLineageDirect(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Batch creates many lineage edges in one HTTP call. Edges with empty Type
// are filled in with DefaultEdgeType.
func (s *LineageService) Batch(ctx context.Context, edges []WriteEdgeInput) ([]*BatchLineageResult, error) {
	body := make([]*models.LineageEdge, len(edges))
	for i, e := range edges {
		t := e.Type
		if t == "" {
			t = DefaultEdgeType
		}
		body[i] = &models.LineageEdge{
			Source: e.Source,
			Target: e.Target,
			Type:   t,
			JobMrn: e.JobMrn,
		}
	}
	p := lineage.NewPostLineageBatchParams().WithContext(ctx).WithEdges(body)
	resp, err := s.gen.Lineage.PostLineageBatch(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
