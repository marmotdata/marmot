package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/tags"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

type Tag = models.GithubComMarmotdataMarmotInternalCoreTagTag

// TagsService manages tags in the catalog.
type TagsService struct {
	gen *apiclient.Marmot
}

// TagsListOptions paginates TagsService.List.
type TagsListOptions struct {
	Limit  int64
	Offset int64
}

// List returns all tags.
func (s *TagsService) List(ctx context.Context, opts TagsListOptions) ([]*Tag, error) {
	p := tags.NewGetTagsParams().WithContext(ctx)
	resp, err := s.gen.Tags.GetTags(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a tag by ID.
func (s *TagsService) Get(ctx context.Context, id string) (*Tag, error) {
	p := tags.NewGetTagsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Tags.GetTagsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

type CreateTagInput struct {
	Name        string
	Description string
}

// Create creates a new tag.
func (s *TagsService) Create(ctx context.Context, in CreateTagInput) (*Tag, error) {
	p := tags.NewPostTagsParams().WithContext(ctx)
	p.SetBody(&models.V1TagsTagRequest{
		Name:        in.Name,
		Description: in.Description,
	})
	resp, err := s.gen.Tags.PostTags(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// UpdateTagInput is the input for TagsService.Update.
type UpdateTagInput struct {
	Name        string
	Description string
}

// Update modifies an existing tag.
func (s *TagsService) Update(ctx context.Context, id string, in UpdateTagInput) (*Tag, error) {
	p := tags.NewPutTagsIDParams().WithContext(ctx).WithID(id)
	p.SetBody(&models.V1TagsTagRequest{
		Name:        in.Name,
		Description: in.Description,
	})
	resp, err := s.gen.Tags.PutTagsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete removes a tag.
func (s *TagsService) Delete(ctx context.Context, id string) error {
	p := tags.NewDeleteTagsIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Tags.DeleteTagsID(p)
	return mapErr(err)
}
