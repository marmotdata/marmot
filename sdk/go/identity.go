package marmot

import (
	"context"

	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/assets"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// AssetIdentity is an asset's MRN in parts, the MRNs it used to have and what each writer says about it.
type AssetIdentity = models.AssetIdentityResponse

// ResolvedMRN is what an MRN of any kind names.
type ResolvedMRN = models.ResolveMRNResponse

// ZoneBinding says that a provider reached at a host belongs to a zone.
type ZoneBinding = models.ZoneBinding

// UpsertAssetInput is the input for AssetsService.Upsert: the MRN names the asset and the other fields are what the caller says about it.
type UpsertAssetInput struct {
	MRN         string
	Name        string
	Type        string
	Providers   []string
	Description string
	Tags        []string
	Metadata    map[string]any
	Schema      map[string]string
}

// Identity returns an asset's MRN in parts, its previous MRNs and its writers.
func (s *AssetsService) Identity(ctx context.Context, id string) (*AssetIdentity, error) {
	resp, err := s.gen.Assets.GetAssetsIdentityID(assets.NewGetAssetsIdentityIDParams().WithContext(ctx).WithID(id), nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Merge folds the asset with id into the asset into, which keeps both assets' owners, terms, lineage and documentation; id's MRN becomes an alias of into.
func (s *AssetsService) Merge(ctx context.Context, id, into string) (*Asset, error) {
	p := assets.NewPostAssetsMergeIDParams().WithContext(ctx).WithID(id).WithRequest(&models.MergeAssetRequest{Into: into})
	resp, err := s.gen.Assets.PostAssetsMergeID(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Upsert creates the asset the MRN names or, when it exists, records the caller's fields on it. It reports whether the asset was created.
func (s *AssetsService) Upsert(ctx context.Context, in UpsertAssetInput) (*Asset, bool, error) {
	body := &models.UpsertAssetRequest{
		Mrn:         in.MRN,
		Name:        &in.Name,
		Type:        &in.Type,
		Providers:   in.Providers,
		Description: in.Description,
		Tags:        in.Tags,
		Schema:      in.Schema,
	}
	if len(in.Metadata) > 0 {
		body.Metadata = in.Metadata
	}
	ok, created, err := s.gen.Assets.PutAssetsByMRN(assets.NewPutAssetsByMRNParams().WithContext(ctx).WithAsset(body), nil)
	if err != nil {
		return nil, false, mapErr(err)
	}
	if created != nil {
		return created.Payload, true, nil
	}
	return ok.Payload, false, nil
}

// ResolveMRN says what an MRN of any kind names, following MRNs things used to have.
func (s *AssetsService) ResolveMRN(ctx context.Context, mrn string) (*ResolvedMRN, error) {
	resp, err := s.gen.Assets.GetMRNResolve(assets.NewGetMRNResolveParams().WithContext(ctx).WithMrn(mrn), nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// ZoneBindings lists which zone each host belongs to.
func (s *AssetsService) ZoneBindings(ctx context.Context) ([]*ZoneBinding, error) {
	resp, err := s.gen.Assets.GetZoneBindings(assets.NewGetZoneBindingsParams().WithContext(ctx), nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// BindZone pins a provider's host to a zone; a pinned binding wins over what plugins report.
func (s *AssetsService) BindZone(ctx context.Context, provider, host, zone string) (*ZoneBinding, error) {
	p := assets.NewPutZoneBindingsParams().WithContext(ctx).WithBinding(&models.ZoneBinding{Provider: provider, Host: host, Zone: zone})
	resp, err := s.gen.Assets.PutZoneBindings(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// UnbindZone removes the binding for a provider's host.
func (s *AssetsService) UnbindZone(ctx context.Context, provider, host string) error {
	_, err := s.gen.Assets.DeleteZoneBindings(assets.NewDeleteZoneBindingsParams().WithContext(ctx).WithProvider(provider).WithHost(host), nil)
	return mapErr(err)
}
