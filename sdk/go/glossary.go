package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/glossary"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// GlossaryTerm is a single business-glossary term.
type GlossaryTerm = models.GlossaryGlossaryTerm

// GlossaryTermList is a paginated set of glossary terms.
type GlossaryTermList = models.GlossaryListResult

// GlossaryListOptions paginates GlossaryService.List.
type GlossaryListOptions struct {
	Limit  int64
	Offset int64
}

// GlossarySearchOptions filters GlossaryService.Search.
type GlossarySearchOptions struct {
	Query        string
	ParentTermID string
	Limit        int64
	Offset       int64
}

// CreateTermInput is the input for GlossaryService.Create.
type CreateTermInput struct {
	Name         string
	Definition   string
	Description  string
	ParentTermID string
}

// UpdateTermInput is the input for GlossaryService.Update.
type UpdateTermInput struct {
	Name         string
	Definition   string
	Description  string
	ParentTermID string
}

// GlossaryService manages glossary terms.
type GlossaryService struct {
	gen *apiclient.Marmot
}

// List returns paginated glossary terms.
func (s *GlossaryService) List(ctx context.Context, opts GlossaryListOptions) (*GlossaryTermList, error) {
	p := glossary.NewGetGlossaryListParams().WithContext(ctx)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Glossary.GetGlossaryList(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Search returns glossary terms matching opts.
func (s *GlossaryService) Search(ctx context.Context, opts GlossarySearchOptions) (*GlossaryTermList, error) {
	p := glossary.NewGetGlossarySearchParams().WithContext(ctx)
	if opts.Query != "" {
		p = p.WithQ(&opts.Query)
	}
	if opts.ParentTermID != "" {
		p = p.WithParentTermID(&opts.ParentTermID)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Glossary.GetGlossarySearch(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a glossary term by ID.
func (s *GlossaryService) Get(ctx context.Context, id string) (*GlossaryTerm, error) {
	p := glossary.NewGetGlossaryIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Glossary.GetGlossaryID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Create creates a new glossary term.
func (s *GlossaryService) Create(ctx context.Context, in CreateTermInput) (*GlossaryTerm, error) {
	body := &models.V1GlossaryCreateTermRequest{
		Name:         &in.Name,
		Definition:   &in.Definition,
		Description:  in.Description,
		ParentTermID: in.ParentTermID,
	}
	p := glossary.NewPostGlossaryParams().WithContext(ctx).WithTerm(body)
	resp, err := s.gen.Glossary.PostGlossary(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Update modifies an existing glossary term.
func (s *GlossaryService) Update(ctx context.Context, id string, in UpdateTermInput) (*GlossaryTerm, error) {
	body := &models.V1GlossaryUpdateTermRequest{
		Name:         in.Name,
		Definition:   in.Definition,
		Description:  in.Description,
		ParentTermID: in.ParentTermID,
	}
	p := glossary.NewPutGlossaryIDParams().WithContext(ctx).WithID(id).WithTerm(body)
	resp, err := s.gen.Glossary.PutGlossaryID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete removes a glossary term.
func (s *GlossaryService) Delete(ctx context.Context, id string) error {
	p := glossary.NewDeleteGlossaryIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Glossary.DeleteGlossaryID(p)
	return mapErr(err)
}
