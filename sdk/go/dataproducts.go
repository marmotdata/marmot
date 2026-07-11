package marmot

import (
	"context"
	"strings"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/products"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// DataProduct groups related assets into a single addressable product.
type DataProduct = models.DataProduct

// DataProductList is a paginated set of data products.
type DataProductList = models.DataProductListResult

// DataProductRule is a rule that dynamically matches assets to a data product.
type DataProductRule = models.DataProductRule

// DataProductRulePreview is the set of assets a rule would match.
type DataProductRulePreview = models.DataProductRulePreview

// DataProductAssets is a paginated set of manually added asset IDs.
type DataProductAssets = models.DataProductAssetsResult

// DataProductResolvedAssets holds all asset IDs of a data product, both
// manually added and matched by rules.
type DataProductResolvedAssets = models.DataProductResolvedAssets

// DataProductListOptions paginates DataProductsService.List.
type DataProductListOptions struct {
	Limit  int64
	Offset int64
}

// DataProductSearchOptions filters DataProductsService.Search.
type DataProductSearchOptions struct {
	Query  string
	Tags   []string
	Limit  int64
	Offset int64
}

// DataProductAssetsOptions paginates DataProductsService.Assets and
// DataProductsService.ResolvedAssets.
type DataProductAssetsOptions struct {
	Limit  int64
	Offset int64
}

// ProductOwner references a user or team that owns a data product.
type ProductOwner struct {
	ID   string
	Type string
}

// ProductRuleInput is the input for the rule methods of DataProductsService.
type ProductRuleInput struct {
	Name            string
	Description     string
	RuleType        string // "query" or "metadata_match"
	QueryExpression string
	MetadataField   string
	PatternType     string // "exact", "wildcard", "regex", or "prefix"
	PatternValue    string
	Priority        int64
	IsEnabled       bool
}

// CreateDataProductInput is the input for DataProductsService.Create.
type CreateDataProductInput struct {
	Name        string
	Description string
	Metadata    map[string]any
	Tags        []string
	Owners      []ProductOwner
	Rules       []ProductRuleInput
}

// UpdateDataProductInput is the input for DataProductsService.Update.
type UpdateDataProductInput struct {
	Name        string
	Description string
	Metadata    map[string]any
	Tags        []string
	Owners      []ProductOwner
}

func productOwnerRequests(owners []ProductOwner) []*models.DataProductOwnerRequest {
	if len(owners) == 0 {
		return nil
	}
	out := make([]*models.DataProductOwnerRequest, len(owners))
	for i, o := range owners {
		id, typ := o.ID, o.Type
		out[i] = &models.DataProductOwnerRequest{ID: &id, Type: &typ}
	}
	return out
}

func productRuleRequest(in ProductRuleInput) *models.DataProductRuleRequest {
	name, ruleType := in.Name, in.RuleType
	return &models.DataProductRuleRequest{
		Name:            &name,
		Description:     in.Description,
		RuleType:        &ruleType,
		QueryExpression: in.QueryExpression,
		MetadataField:   in.MetadataField,
		PatternType:     in.PatternType,
		PatternValue:    in.PatternValue,
		Priority:        in.Priority,
		IsEnabled:       in.IsEnabled,
	}
}

// DataProductsService manages data products, their assets, and their
// membership rules.
type DataProductsService struct {
	gen *apiclient.Marmot
}

// List returns paginated data products.
func (s *DataProductsService) List(ctx context.Context, opts DataProductListOptions) (*DataProductList, error) {
	p := products.NewGetProductsListParams().WithContext(ctx)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Products.GetProductsList(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Search returns data products matching opts.
func (s *DataProductsService) Search(ctx context.Context, opts DataProductSearchOptions) (*DataProductList, error) {
	p := products.NewGetProductsSearchParams().WithContext(ctx)
	if opts.Query != "" {
		p = p.WithQ(&opts.Query)
	}
	if len(opts.Tags) > 0 {
		tags := strings.Join(opts.Tags, ",")
		p = p.WithTags(&tags)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Products.GetProductsSearch(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a data product by ID.
func (s *DataProductsService) Get(ctx context.Context, id string) (*DataProduct, error) {
	p := products.NewGetProductsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Products.GetProductsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Create creates a new data product.
func (s *DataProductsService) Create(ctx context.Context, in CreateDataProductInput) (*DataProduct, error) {
	body := &models.CreateDataProductRequest{
		Name:        &in.Name,
		Description: in.Description,
		Tags:        in.Tags,
		Owners:      productOwnerRequests(in.Owners),
	}
	if len(in.Metadata) > 0 {
		body.Metadata = in.Metadata
	}
	if len(in.Rules) > 0 {
		body.Rules = make([]*models.DataProductRuleRequest, len(in.Rules))
		for i, r := range in.Rules {
			body.Rules[i] = productRuleRequest(r)
		}
	}
	p := products.NewPostProductsParams().WithContext(ctx).WithProduct(body)
	resp, err := s.gen.Products.PostProducts(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Update modifies an existing data product.
func (s *DataProductsService) Update(ctx context.Context, id string, in UpdateDataProductInput) (*DataProduct, error) {
	body := &models.UpdateDataProductRequest{
		Name:        in.Name,
		Description: in.Description,
		Tags:        in.Tags,
		Owners:      productOwnerRequests(in.Owners),
	}
	if len(in.Metadata) > 0 {
		body.Metadata = in.Metadata
	}
	p := products.NewPutProductsIDParams().WithContext(ctx).WithID(id).WithProduct(body)
	resp, err := s.gen.Products.PutProductsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete removes a data product.
func (s *DataProductsService) Delete(ctx context.Context, id string) error {
	p := products.NewDeleteProductsIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Products.DeleteProductsID(p)
	return mapErr(err)
}

// Assets returns the manually added assets of a data product.
func (s *DataProductsService) Assets(ctx context.Context, id string, opts DataProductAssetsOptions) (*DataProductAssets, error) {
	p := products.NewGetProductsAssetsIDParams().WithContext(ctx).WithID(id)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Products.GetProductsAssetsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// AddAssets manually adds assets to a data product.
func (s *DataProductsService) AddAssets(ctx context.Context, id string, assetIDs []string) error {
	body := &models.AddDataProductAssetsRequest{AssetIds: assetIDs}
	p := products.NewPostProductsAssetsIDParams().WithContext(ctx).WithID(id).WithAssets(body)
	_, err := s.gen.Products.PostProductsAssetsID(p)
	return mapErr(err)
}

// RemoveAsset removes a manually added asset from a data product.
func (s *DataProductsService) RemoveAsset(ctx context.Context, id, assetID string) error {
	p := products.NewDeleteProductsAssetsIDAssetIDParams().WithContext(ctx).WithID(id).WithAssetID(assetID)
	_, err := s.gen.Products.DeleteProductsAssetsIDAssetID(p)
	return mapErr(err)
}

// ResolvedAssets returns all assets of a data product, both manually added
// and matched by rules.
func (s *DataProductsService) ResolvedAssets(ctx context.Context, id string, opts DataProductAssetsOptions) (*DataProductResolvedAssets, error) {
	p := products.NewGetProductsResolvedAssetsIDParams().WithContext(ctx).WithID(id)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Products.GetProductsResolvedAssetsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Rules returns the membership rules of a data product.
func (s *DataProductsService) Rules(ctx context.Context, id string) ([]*DataProductRule, error) {
	p := products.NewGetProductsRulesIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Products.GetProductsRulesID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload.Rules, nil
}

// CreateRule adds a membership rule to a data product.
func (s *DataProductsService) CreateRule(ctx context.Context, id string, in ProductRuleInput) (*DataProductRule, error) {
	p := products.NewPostProductsRulesIDParams().WithContext(ctx).WithID(id).WithRule(productRuleRequest(in))
	resp, err := s.gen.Products.PostProductsRulesID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// UpdateRule modifies a membership rule of a data product.
func (s *DataProductsService) UpdateRule(ctx context.Context, id, ruleID string, in ProductRuleInput) (*DataProductRule, error) {
	p := products.NewPutProductsRulesIDRuleIDParams().WithContext(ctx).WithID(id).WithRuleID(ruleID).WithRule(productRuleRequest(in))
	resp, err := s.gen.Products.PutProductsRulesIDRuleID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// DeleteRule removes a membership rule from a data product.
func (s *DataProductsService) DeleteRule(ctx context.Context, id, ruleID string) error {
	p := products.NewDeleteProductsRulesIDRuleIDParams().WithContext(ctx).WithID(id).WithRuleID(ruleID)
	_, err := s.gen.Products.DeleteProductsRulesIDRuleID(p)
	return mapErr(err)
}

// PreviewRule returns the assets a rule would match without saving it.
// limit caps the number of returned asset IDs; zero uses the server default.
func (s *DataProductsService) PreviewRule(ctx context.Context, in ProductRuleInput, limit int64) (*DataProductRulePreview, error) {
	p := products.NewPostProductsRulePreviewParams().WithContext(ctx).WithRule(productRuleRequest(in))
	if limit > 0 {
		p = p.WithLimit(&limit)
	}
	resp, err := s.gen.Products.PostProductsRulePreview(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
