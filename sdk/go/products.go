package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/products"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// DataProduct is a data product entry.
type DataProduct = models.DataproductDataProduct

// DataProductListResult is a paginated list of data products.
type DataProductListResult = models.DataproductListResult

// ProductsService manages data products.
type ProductsService struct {
	gen *apiclient.Marmot
}

// ProductsListOptions paginates ProductsService.List.
type ProductsListOptions struct {
	Limit  int64
	Offset int64
}

// List returns paginated data products.
func (s *ProductsService) List(ctx context.Context, opts ProductsListOptions) (*DataProductListResult, error) {
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

// Get fetches a data product by ID.
func (s *ProductsService) Get(ctx context.Context, id string) (*DataProduct, error) {
	p := products.NewGetProductsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Products.GetProductsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// ListProductTags returns all tags on a data product.
func (s *ProductsService) ListProductTags(ctx context.Context, productID string) ([]*models.GithubComMarmotdataMarmotInternalCoreTagTag, error) {
	p := products.NewGetProductsTagsIDParams().WithContext(ctx).WithID(productID)
	resp, err := s.gen.Products.GetProductsTagsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// AddProductTag adds a tag to a data product.
func (s *ProductsService) AddProductTag(ctx context.Context, productID, tagID string) error {
	p := products.NewPostProductsTagsIDParams().WithContext(ctx).WithID(productID)
	p.SetBody(&models.V1DataproductsAddProductTagRequest{TagID: tagID})
	_, err := s.gen.Products.PostProductsTagsID(p)
	return mapErr(err)
}

// RemoveProductTag removes a tag from a data product.
func (s *ProductsService) RemoveProductTag(ctx context.Context, productID, tagID string) error {
	p := products.NewDeleteProductsTagsIDParams().WithContext(ctx).WithID(productID)
	p.SetBody(&models.V1DataproductsRemoveProductTagRequest{TagID: tagID})
	_, err := s.gen.Products.DeleteProductsTagsID(p)
	return mapErr(err)
}

// SetProductTags replaces all tags on a data product.
func (s *ProductsService) SetProductTags(ctx context.Context, productID string, tagIDs []string) error {
	p := products.NewPutProductsTagsIDParams().WithContext(ctx).WithID(productID)
	p.SetBody(&models.V1DataproductsReplaceProductTagsRequest{TagIds: tagIDs})
	_, err := s.gen.Products.PutProductsTagsID(p)
	return mapErr(err)
}