package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/assets"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// Asset is a single catalog entry.
type Asset = models.AssetAsset

// AssetSearchResults is the response from AssetsService.Search.
type AssetSearchResults = models.V1AssetsSearchResponse

// AssetSummary is the response from AssetsService.Summary.
type AssetSummary = models.V1AssetsAssetSummaryResponse

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
	Tags        []string
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
	body := &models.V1AssetsCreateRequest{
		Name:        &in.Name,
		Type:        &in.Type,
		Providers:   in.Providers,
		Tags:        in.Tags,
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

// AddTag adds a tag to an asset.
func (s *AssetsService) AddTag(ctx context.Context, id, tag string) error {
	p := assets.NewPostAssetsTagsIDParams().WithContext(ctx).WithID(id).WithTag(&models.V1AssetsTagRequest{Tag: &tag})
	_, err := s.gen.Assets.PostAssetsTagsID(p)
	return mapErr(err)
}

// RemoveTag removes a tag from an asset.
func (s *AssetsService) RemoveTag(ctx context.Context, id, tag string) error {
	p := assets.NewDeleteAssetsTagsIDParams().WithContext(ctx).WithID(id).WithTag(&models.V1AssetsTagRequest{Tag: &tag})
	_, err := s.gen.Assets.DeleteAssetsTagsID(p)
	return mapErr(err)
}
