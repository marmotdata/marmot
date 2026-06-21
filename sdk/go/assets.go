package marmot

import (
	"context"
	"errors"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/assets"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// Asset is a single catalog entry.
type Asset = models.Asset

// AssetSearchResults is the response from AssetsService.Search.
type AssetSearchResults = models.AssetSearchResponse

// AssetSummary is the response from AssetsService.Summary.
type AssetSummary = models.AssetSummaryResponse

// AssetSearchOptions filters AssetsService.Search.
type AssetSearchOptions struct {
	Query     string
	Types     []string
	Providers []string
	Tags      []string
	Limit     int64
	Offset    int64
}

// CreateAssetInput is the input for AssetsService.Create.
type CreateAssetInput struct {
	Name        string
	Type        string
	Providers   []string
	Description string
}

// LookupInput identifies an asset by its natural key (type, service, name).
type LookupInput struct {
	Type    string
	Service string
	Name    string
}

// UpdateAssetInput is the input for AssetsService.Update. Empty fields are
// omitted from the patch — the server treats absent fields as "leave unchanged".
type UpdateAssetInput struct {
	Name            string
	Type            string
	Description     string
	UserDescription string
	Providers       []string
}

// AssetsService covers asset CRUD, search, summary, and tag management.
type AssetsService struct {
	gen *apiclient.Marmot
}

// Search returns assets matching opts.
func (s *AssetsService) Search(ctx context.Context, opts AssetSearchOptions) (*AssetSearchResults, error) {
	p := assets.NewGetAssetsSearchParams().WithContext(ctx)
	if opts.Query != "" {
		p = p.WithQ(&opts.Query)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	if len(opts.Types) > 0 {
		p = p.WithTypes(opts.Types)
	}
	if len(opts.Providers) > 0 {
		p = p.WithServices(opts.Providers)
	}
	if len(opts.Tags) > 0 {
		p = p.WithTags(opts.Tags)
	}
	resp, err := s.gen.Assets.GetAssetsSearch(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Create creates a new asset.
func (s *AssetsService) Create(ctx context.Context, in CreateAssetInput) (*Asset, error) {
	body := &models.CreateAssetRequest{
		Name:        &in.Name,
		Type:        &in.Type,
		Providers:   in.Providers,
		Description: in.Description,
	}
	p := assets.NewPostAssetsParams().WithContext(ctx).WithAsset(body)
	resp, err := s.gen.Assets.PostAssets(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches an asset by ID.
func (s *AssetsService) Get(ctx context.Context, id string) (*Asset, error) {
	p := assets.NewGetAssetsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Assets.GetAssetsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Lookup fetches an asset by its natural key. Returns *NotFoundError if
// no asset matches; see Find for a nil-on-miss variant.
func (s *AssetsService) Lookup(ctx context.Context, in LookupInput) (*Asset, error) {
	p := assets.NewGetAssetsLookupTypeServiceNameParams().
		WithContext(ctx).
		WithType(in.Type).
		WithService(in.Service).
		WithName(in.Name)
	resp, err := s.gen.Assets.GetAssetsLookupTypeServiceName(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Find is like Lookup but returns (nil, nil) on a 404 instead of an error.
func (s *AssetsService) Find(ctx context.Context, in LookupInput) (*Asset, error) {
	asset, err := s.Lookup(ctx, in)
	if err != nil {
		var nf *NotFoundError
		if errors.As(err, &nf) {
			return nil, nil
		}
		return nil, err
	}
	return asset, nil
}

// Update modifies an existing asset by ID. Empty fields on in are skipped.
func (s *AssetsService) Update(ctx context.Context, id string, in UpdateAssetInput) (*Asset, error) {
	body := &models.UpdateAssetRequest{
		Name:            in.Name,
		Type:            in.Type,
		Description:     in.Description,
		UserDescription: in.UserDescription,
		Providers:       in.Providers,
	}
	p := assets.NewPutAssetsIDParams().WithContext(ctx).WithID(id).WithAsset(body)
	resp, err := s.gen.Assets.PutAssetsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete removes an asset by ID.
func (s *AssetsService) Delete(ctx context.Context, id string) error {
	p := assets.NewDeleteAssetsIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Assets.DeleteAssetsID(p)
	return mapErr(err)
}

// Summary returns aggregate counts for the catalog.
func (s *AssetsService) Summary(ctx context.Context) (*AssetSummary, error) {
	p := assets.NewGetAssetsSummaryParams().WithContext(ctx)
	resp, err := s.gen.Assets.GetAssetsSummary(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// ListTags lists all tags on an asset.
func (s *AssetsService) ListTags(ctx context.Context, id string) ([]*Tag, error) {
	p := assets.NewGetAssetsTagsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Assets.GetAssetsTagsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// AddTag adds a tag to an asset by tag ID.
func (s *AssetsService) AddTag(ctx context.Context, id, tagID string) error {
	p := assets.NewPostAssetsTagsIDParams().WithContext(ctx).WithID(id)
	p.SetBody(&models.AddAssetTagRequest{TagID: tagID})
	_, err := s.gen.Assets.PostAssetsTagsID(p)
	return mapErr(err)
}

// RemoveTag removes a tag from an asset by tag ID.
func (s *AssetsService) RemoveTag(ctx context.Context, id, tagID string) error {
	p := assets.NewDeleteAssetsTagsIDParams().WithContext(ctx).WithID(id)
	p.SetBody(&models.RemoveAssetTagRequest{TagID: tagID})
	_, err := s.gen.Assets.DeleteAssetsTagsID(p)
	return mapErr(err)
}

// SetTags replaces all tags on an asset.
func (s *AssetsService) SetTags(ctx context.Context, id string, tagIDs []string) error {
	p := assets.NewPutAssetsTagsIDParams().WithContext(ctx).WithID(id)
	p.SetBody(&models.ReplaceAssetTagsRequest{TagIds: tagIDs})
	_, err := s.gen.Assets.PutAssetsTagsID(p)
	return mapErr(err)
}

// SetColumnTags replaces all tags on a specific column of an asset.
func (s *AssetsService) SetColumnTags(ctx context.Context, id string, columnPath string, tagIDs []string) error {
	p := assets.NewPutAssetsColumnTagsIDParams().WithContext(ctx).WithID(id)
	p.SetBody(&models.ReplaceAssetColumnTagsRequest{
		ColumnPath: columnPath,
		TagIds:     tagIDs,
	})
	_, err := s.gen.Assets.PutAssetsColumnTagsID(p)
	return mapErr(err)
}

// RemoveColumnTag removes a single tag from a specific column of an asset.
func (s *AssetsService) RemoveColumnTag(ctx context.Context, id string, columnPath string, tagID string) error {
	p := assets.NewDeleteAssetsColumnTagsIDParams().WithContext(ctx).WithID(id)
	p.SetBody(&models.RemoveAssetColumnTagRequest{
		ColumnPath: columnPath,
		TagID:      tagID,
	})
	_, err := s.gen.Assets.DeleteAssetsColumnTagsID(p)
	return mapErr(err)
}

