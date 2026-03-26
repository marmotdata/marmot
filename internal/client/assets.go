package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/marmotdata/marmot/internal/core/asset"
)

// AssetsSearchResponse represents the response from the assets search endpoint.
type AssetsSearchResponse struct {
	Assets           []asset.Asset          `json:"assets"`
	Total            int                    `json:"total"`
	Limit            int                    `json:"limit"`
	Offset           int                    `json:"offset"`
	AvailableFilters asset.AvailableFilters `json:"available_filters"`
}

// AssetOwnerEntry represents an owner of an asset.
type AssetOwnerEntry struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Username       *string `json:"username,omitempty"`
	Email          *string `json:"email,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
}

// ListAssets searches assets with optional query, pagination, and filters.
func (c *Client) ListAssets(ctx context.Context, query string, limit, offset int, assetTypes, providers, tags []string) (*AssetsSearchResponse, error) {
	q := url.Values{}
	if query != "" {
		q.Set("q", query)
	}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	for _, t := range assetTypes {
		q.Add("types", t)
	}
	for _, p := range providers {
		q.Add("providers", p)
	}
	for _, t := range tags {
		q.Add("tags", t)
	}

	var resp AssetsSearchResponse
	if err := c.get(ctx, "/assets/search", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAsset retrieves a single asset by ID.
func (c *Client) GetAsset(ctx context.Context, id string) (*asset.Asset, error) {
	var a asset.Asset
	if err := c.get(ctx, "/assets/"+id, nil, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteAsset deletes an asset by ID.
func (c *Client) DeleteAsset(ctx context.Context, id string) error {
	return c.del(ctx, "/assets/"+id, nil, nil)
}

// GetAssetSummary retrieves the asset summary (counts by type, provider, tag).
func (c *Client) GetAssetSummary(ctx context.Context) (*asset.AssetSummary, error) {
	var s asset.AssetSummary
	if err := c.get(ctx, "/assets/summary", nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// AddTag adds a tag to an asset.
func (c *Client) AddTag(ctx context.Context, assetID, tag string) error {
	body := map[string]string{
		"asset_id": assetID,
		"tag":      tag,
	}
	return c.post(ctx, "/assets/tags/", body, nil)
}

// RemoveTag removes a tag from an asset.
func (c *Client) RemoveTag(ctx context.Context, assetID, tag string) error {
	body := map[string]string{
		"asset_id": assetID,
		"tag":      tag,
	}
	return c.delWithBody(ctx, "/assets/tags/", body, nil)
}

// ListAssetOwners lists the owners of an asset.
func (c *Client) ListAssetOwners(ctx context.Context, assetID string) ([]AssetOwnerEntry, error) {
	q := url.Values{}
	q.Set("asset_id", assetID)

	var owners []AssetOwnerEntry
	if err := c.get(ctx, "/assets/owners/", q, &owners); err != nil {
		return nil, err
	}
	return owners, nil
}

// GetAssetRaw retrieves a single asset as raw JSON bytes for json/yaml output.
func (c *Client) GetAssetRaw(ctx context.Context, id string) ([]byte, error) {
	return c.getRaw(ctx, "/assets/"+id, nil)
}

// ListAssetsRaw searches assets and returns raw JSON bytes.
func (c *Client) ListAssetsRaw(ctx context.Context, query string, limit, offset int, assetTypes, providers, tags []string) ([]byte, error) {
	q := url.Values{}
	if query != "" {
		q.Set("q", query)
	}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	for _, t := range assetTypes {
		q.Add("types", t)
	}
	for _, p := range providers {
		q.Add("providers", p)
	}
	for _, t := range tags {
		q.Add("tags", t)
	}

	return c.getRaw(ctx, "/assets/search", q)
}

// GetAssetSummaryRaw retrieves the asset summary as raw JSON.
func (c *Client) GetAssetSummaryRaw(ctx context.Context) ([]byte, error) {
	return c.getRaw(ctx, "/assets/summary", nil)
}

// ListAssetOwnersRaw retrieves asset owners as raw JSON.
func (c *Client) ListAssetOwnersRaw(ctx context.Context, assetID string) ([]byte, error) {
	q := url.Values{}
	q.Set("asset_id", assetID)
	return c.getRaw(ctx, "/assets/owners/", q)
}

// Deref safely dereferences a string pointer.
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FormatAssetName returns the display name for an asset.
func FormatAssetName(a asset.Asset) string {
	if a.Name != nil {
		return *a.Name
	}
	if a.MRN != nil {
		return *a.MRN
	}
	return fmt.Sprintf("(%s)", a.ID)
}
