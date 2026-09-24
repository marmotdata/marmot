package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
)

// The decorators check every write against the entity's domain before
// delegating; reads pass through the embedded service untouched.

// undoCreate reports a create that could not be placed in its target domain.
// The entity is removed so it never lingers in Unassigned by accident.
func undoCreate(placeErr, deleteErr error) error {
	if deleteErr != nil {
		return errors.Join(placeErr, fmt.Errorf("removing the unplaced entity: %w", deleteErr))
	}
	return placeErr
}

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
	a, err := s.Service.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.g.PlaceCreated(ctx, KindAsset, a.ID); err != nil {
		return nil, undoCreate(err, s.Service.Delete(ctx, a.ID))
	}
	return a, nil
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
	dp, err := s.Service.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.g.PlaceCreated(ctx, KindDataProduct, dp.ID); err != nil {
		return nil, undoCreate(err, s.Service.Delete(ctx, dp.ID))
	}
	return dp, nil
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
	term, err := s.Service.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.g.PlaceCreated(ctx, KindGlossaryTerm, term.ID); err != nil {
		return nil, undoCreate(err, s.Service.Delete(ctx, term.ID))
	}
	return term, nil
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

type guardedLineage struct {
	lineage.Service
	g *Guard
}

// GuardLineage scopes lineage edges by their target: the downstream asset
// declares what it reads, so its domain decides, wherever the source lives.
// Creating edges is vetted inside the service (lineage.WithEdgeGuard) so the
// edges an OpenLineage event writes internally are covered too.
func GuardLineage(inner lineage.Service, g *Guard) lineage.Service {
	return &guardedLineage{Service: inner, g: g}
}

func (s *guardedLineage) DeleteDirectLineage(ctx context.Context, edgeID string) error {
	if on, err := s.g.Enforced(ctx); err != nil {
		return err
	} else if on {
		// An unknown edge falls through so the inner service reports it as usual.
		if edge, err := s.GetDirectLineage(ctx, edgeID); err == nil {
			if err := s.g.AuthorizeAssetMRNs(ctx, edge.Target); err != nil {
				return err
			}
		}
	}
	return s.Service.DeleteDirectLineage(ctx, edgeID)
}

func (s *guardedLineage) BatchObservedLineage(ctx context.Context, edges []lineage.ObservedEdge) error {
	targets := make([]string, 0, len(edges))
	for _, e := range edges {
		targets = append(targets, e.Target)
	}
	if err := s.g.AuthorizeAssetMRNs(ctx, targets...); err != nil {
		return err
	}
	return s.Service.BatchObservedLineage(ctx, edges)
}

type guardedAssetDocs struct {
	assetdocs.Service
	g *Guard
}

func GuardAssetDocs(inner assetdocs.Service, g *Guard) assetdocs.Service {
	return &guardedAssetDocs{Service: inner, g: g}
}

func (s *guardedAssetDocs) Create(ctx context.Context, doc assetdocs.Documentation) error {
	if err := s.g.AuthorizeAssetMRNs(ctx, doc.MRN); err != nil {
		return err
	}
	return s.Service.Create(ctx, doc)
}

func (s *guardedAssetDocs) CreateGlobal(ctx context.Context, doc assetdocs.GlobalDocumentation) error {
	if err := s.g.AuthorizeGlobal(ctx); err != nil {
		return err
	}
	return s.Service.CreateGlobal(ctx, doc)
}
