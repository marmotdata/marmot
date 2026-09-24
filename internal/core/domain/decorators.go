package domain

import (
	"context"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/glossary"
)

// The decorators check every write against the entity's domain before
// delegating; reads pass through the embedded service untouched.

type guardedAssets struct {
	asset.Service
	g *Guard
}

func GuardAssets(inner asset.Service, g *Guard) asset.Service {
	return &guardedAssets{Service: inner, g: g}
}

func (s *guardedAssets) Create(ctx context.Context, in asset.CreateInput) (*asset.Asset, error) {
	if err := s.g.AuthorizeCreate(ctx); err != nil {
		return nil, err
	}
	return s.Service.Create(ctx, in)
}

func (s *guardedAssets) Update(ctx context.Context, id string, in asset.UpdateInput) (*asset.Asset, error) {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, id); err != nil {
		return nil, err
	}
	return s.Service.Update(ctx, id, in)
}

func (s *guardedAssets) PatchFields(ctx context.Context, id string, version int64, fields map[string]any) (*asset.Asset, error) {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, id); err != nil {
		return nil, err
	}
	return s.Service.PatchFields(ctx, id, version, fields)
}

func (s *guardedAssets) Delete(ctx context.Context, id string) error {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, id); err != nil {
		return err
	}
	return s.Service.Delete(ctx, id)
}

func (s *guardedAssets) DeleteByMRN(ctx context.Context, mrn string) error {
	if on, err := s.g.Enforced(ctx); err != nil {
		return err
	} else if on {
		// An unknown MRN falls through so the inner service reports it as usual.
		if a, err := s.GetByMRN(ctx, mrn); err == nil {
			if err := s.g.AuthorizeEntities(ctx, KindAsset, a.ID); err != nil {
				return err
			}
		}
	}
	return s.Service.DeleteByMRN(ctx, mrn)
}

func (s *guardedAssets) AddTag(ctx context.Context, id, tag string) (*asset.Asset, error) {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, id); err != nil {
		return nil, err
	}
	return s.Service.AddTag(ctx, id, tag)
}

func (s *guardedAssets) RemoveTag(ctx context.Context, id, tag string) (*asset.Asset, error) {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, id); err != nil {
		return nil, err
	}
	return s.Service.RemoveTag(ctx, id, tag)
}

func (s *guardedAssets) AddTerms(ctx context.Context, assetID string, termIDs []string, source, createdBy string) error {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, assetID); err != nil {
		return err
	}
	return s.Service.AddTerms(ctx, assetID, termIDs, source, createdBy)
}

func (s *guardedAssets) RemoveTerm(ctx context.Context, assetID, termID string) error {
	if err := s.g.AuthorizeEntities(ctx, KindAsset, assetID); err != nil {
		return err
	}
	return s.Service.RemoveTerm(ctx, assetID, termID)
}

type guardedProducts struct {
	dataproduct.Service
	g *Guard
}

// GuardDataProducts scopes product writes, including asset membership, by the
// product's domain: adding an asset to a product changes the product.
func GuardDataProducts(inner dataproduct.Service, g *Guard) dataproduct.Service {
	return &guardedProducts{Service: inner, g: g}
}

func (s *guardedProducts) Create(ctx context.Context, in dataproduct.CreateInput) (*dataproduct.DataProduct, error) {
	if err := s.g.AuthorizeCreate(ctx); err != nil {
		return nil, err
	}
	return s.Service.Create(ctx, in)
}

func (s *guardedProducts) Update(ctx context.Context, id string, in dataproduct.UpdateInput) (*dataproduct.DataProduct, error) {
	if err := s.g.AuthorizeEntities(ctx, KindDataProduct, id); err != nil {
		return nil, err
	}
	return s.Service.Update(ctx, id, in)
}

func (s *guardedProducts) Delete(ctx context.Context, id string) error {
	if err := s.g.AuthorizeEntities(ctx, KindDataProduct, id); err != nil {
		return err
	}
	return s.Service.Delete(ctx, id)
}

func (s *guardedProducts) AddAssets(ctx context.Context, productID string, assetIDs []string, createdBy string) error {
	if err := s.g.AuthorizeEntities(ctx, KindDataProduct, productID); err != nil {
		return err
	}
	return s.Service.AddAssets(ctx, productID, assetIDs, createdBy)
}

func (s *guardedProducts) RemoveAsset(ctx context.Context, productID, assetID string) error {
	if err := s.g.AuthorizeEntities(ctx, KindDataProduct, productID); err != nil {
		return err
	}
	return s.Service.RemoveAsset(ctx, productID, assetID)
}

func (s *guardedProducts) UploadImage(ctx context.Context, productID string, purpose dataproduct.ImagePurpose, in dataproduct.UploadImageInput, createdBy *string) (*dataproduct.ProductImageMeta, error) {
	if err := s.g.AuthorizeEntities(ctx, KindDataProduct, productID); err != nil {
		return nil, err
	}
	return s.Service.UploadImage(ctx, productID, purpose, in, createdBy)
}

func (s *guardedProducts) DeleteImage(ctx context.Context, productID string, purpose dataproduct.ImagePurpose) error {
	if err := s.g.AuthorizeEntities(ctx, KindDataProduct, productID); err != nil {
		return err
	}
	return s.Service.DeleteImage(ctx, productID, purpose)
}

// Membership rules match assets across the catalog, so they stay global.

func (s *guardedProducts) CreateRule(ctx context.Context, productID string, in dataproduct.RuleInput) (*dataproduct.Rule, error) {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return nil, err
	}
	return s.Service.CreateRule(ctx, productID, in)
}

func (s *guardedProducts) UpdateRule(ctx context.Context, ruleID string, in dataproduct.RuleInput) (*dataproduct.Rule, error) {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return nil, err
	}
	return s.Service.UpdateRule(ctx, ruleID, in)
}

func (s *guardedProducts) DeleteRule(ctx context.Context, ruleID string) error {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return err
	}
	return s.Service.DeleteRule(ctx, ruleID)
}

type guardedGlossary struct {
	glossary.Service
	g *Guard
}

func GuardGlossary(inner glossary.Service, g *Guard) glossary.Service {
	return &guardedGlossary{Service: inner, g: g}
}

func (s *guardedGlossary) Create(ctx context.Context, in glossary.CreateTermInput) (*glossary.GlossaryTerm, error) {
	if err := s.g.AuthorizeCreate(ctx); err != nil {
		return nil, err
	}
	return s.Service.Create(ctx, in)
}

func (s *guardedGlossary) Update(ctx context.Context, id string, in glossary.UpdateTermInput) (*glossary.GlossaryTerm, error) {
	if err := s.g.AuthorizeEntities(ctx, KindGlossaryTerm, id); err != nil {
		return nil, err
	}
	return s.Service.Update(ctx, id, in)
}

func (s *guardedGlossary) Delete(ctx context.Context, id string) error {
	if err := s.g.AuthorizeEntities(ctx, KindGlossaryTerm, id); err != nil {
		return err
	}
	return s.Service.Delete(ctx, id)
}

// SyncTerms is all or nothing: a run that may not rewrite one of its terms
// syncs none, rather than leaving the glossary half updated.
func (s *guardedGlossary) SyncTerms(ctx context.Context, source string, inputs []glossary.TermInput) (*glossary.SyncResult, error) {
	names := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if name := strings.TrimSpace(in.Name); name != "" {
			names = append(names, name)
		}
	}
	if err := s.g.AuthorizeTerms(ctx, names); err != nil {
		return nil, err
	}
	return s.Service.SyncTerms(ctx, source, inputs)
}

type guardedAssetRules struct {
	assetrule.Service
	g *Guard
}

// GuardAssetRules keeps asset rules global: they link terms and resources to
// assets in any domain.
func GuardAssetRules(inner assetrule.Service, g *Guard) assetrule.Service {
	return &guardedAssetRules{Service: inner, g: g}
}

func (s *guardedAssetRules) Create(ctx context.Context, in assetrule.CreateInput, createdBy *string) (*assetrule.AssetRule, error) {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return nil, err
	}
	return s.Service.Create(ctx, in, createdBy)
}

func (s *guardedAssetRules) Update(ctx context.Context, id string, in assetrule.UpdateInput) (*assetrule.AssetRule, error) {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return nil, err
	}
	return s.Service.Update(ctx, id, in)
}

func (s *guardedAssetRules) Delete(ctx context.Context, id string) error {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return err
	}
	return s.Service.Delete(ctx, id)
}
