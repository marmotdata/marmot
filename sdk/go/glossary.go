package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/glossary"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// GlossaryTerm is a single business-glossary term.
type GlossaryTerm = models.GlossaryTerm

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
	body := &models.CreateTermRequest{
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
	body := &models.UpdateTermRequest{
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

// ListTermTags returns all tags on a glossary term.
func (s *GlossaryService) ListTermTags(ctx context.Context, termID string) ([]*models.Tag, error) {
	p := glossary.NewGetGlossaryTagsIDParams().WithContext(ctx).WithID(termID)
	resp, err := s.gen.Glossary.GetGlossaryTagsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// AddTermTag adds a tag to a glossary term.
func (s *GlossaryService) AddTermTag(ctx context.Context, termID, tagID string) error {
	p := glossary.NewPostGlossaryTagsIDParams().WithContext(ctx).WithID(termID)
	p.SetBody(&models.AddGlossaryTermTagRequest{TagID: tagID})
	_, err := s.gen.Glossary.PostGlossaryTagsID(p)
	return mapErr(err)
}

// RemoveTermTag removes a tag from a glossary term.
func (s *GlossaryService) RemoveTermTag(ctx context.Context, termID, tagID string) error {
	p := glossary.NewDeleteGlossaryTagsIDParams().WithContext(ctx).WithID(termID)
	p.SetBody(&models.RemoveGlossaryTermTagRequest{TagID: tagID})
	_, err := s.gen.Glossary.DeleteGlossaryTagsID(p)
	return mapErr(err)
}

// SetTermTags replaces all tags on a glossary term.
func (s *GlossaryService) SetTermTags(ctx context.Context, termID string, tagIDs []string) error {
	p := glossary.NewPutGlossaryTagsIDParams().WithContext(ctx).WithID(termID)
	p.SetBody(&models.ReplaceGlossaryTermTagsRequest{TagIds: tagIDs})
	_, err := s.gen.Glossary.PutGlossaryTagsID(p)
	return mapErr(err)
}
