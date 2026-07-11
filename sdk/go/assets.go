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

// AssetExternalLink is a labeled URL attached to an asset.
type AssetExternalLink = models.AssetExternalLink

// AssetSource records where an asset's metadata originated, with a priority
// used to resolve conflicts when several sources describe the same asset.
type AssetSource = models.AssetSource

// AssetEnvironment describes an asset within a named environment (e.g. prod,
// staging), including an environment-specific path and metadata.
type AssetEnvironment = models.Environment

// CreateAssetInput is the input for AssetsService.Create. Only Name, Type, and
// Providers are required; empty optional fields are omitted from the request.
type CreateAssetInput struct {
	Name          string
	Type          string
	Providers     []string
	Description   string
	Tags          []string
	Metadata      map[string]any
	Schema        map[string]string
	ExternalLinks []*AssetExternalLink
	Sources       []*AssetSource
	Environments  map[string]AssetEnvironment
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
	Tags            []string
	Metadata        map[string]any
	Schema          map[string]string
	ExternalLinks   []*AssetExternalLink
	Sources         []*AssetSource
	Environments    map[string]AssetEnvironment
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
		Name:          &in.Name,
		Type:          &in.Type,
		Providers:     in.Providers,
		Tags:          in.Tags,
		Description:   in.Description,
		Schema:        in.Schema,
		ExternalLinks: in.ExternalLinks,
		Sources:       in.Sources,
		Environments:  in.Environments,
	}
	if len(in.Metadata) > 0 {
		body.Metadata = in.Metadata
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
		Tags:            in.Tags,
		Schema:          in.Schema,
		ExternalLinks:   in.ExternalLinks,
		Sources:         in.Sources,
		Environments:    in.Environments,
	}
	if len(in.Metadata) > 0 {
		body.Metadata = in.Metadata
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

// AddTag adds a tag to an asset.
func (s *AssetsService) AddTag(ctx context.Context, id, tag string) error {
	p := assets.NewPostAssetsTagsIDParams().WithContext(ctx).WithID(id).WithTag(&models.TagRequest{Tag: &tag})
	_, err := s.gen.Assets.PostAssetsTagsID(p)
	return mapErr(err)
}

// RemoveTag removes a tag from an asset.
func (s *AssetsService) RemoveTag(ctx context.Context, id, tag string) error {
	p := assets.NewDeleteAssetsTagsIDParams().WithContext(ctx).WithID(id).WithTag(&models.TagRequest{Tag: &tag})
	_, err := s.gen.Assets.DeleteAssetsTagsID(p)
	return mapErr(err)
}
