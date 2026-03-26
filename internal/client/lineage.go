package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/marmotdata/marmot/internal/core/lineage"
)

// GetAssetLineage retrieves the lineage graph for an asset.
func (c *Client) GetAssetLineage(ctx context.Context, assetID string, depth int) (*lineage.LineageResponse, error) {
	q := url.Values{}
	if depth > 0 {
		q.Set("depth", strconv.Itoa(depth))
	}

	var resp lineage.LineageResponse
	if err := c.get(ctx, "/lineage/assets/"+assetID, q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAssetLineageRaw retrieves the lineage graph as raw JSON.
func (c *Client) GetAssetLineageRaw(ctx context.Context, assetID string, depth int) ([]byte, error) {
	q := url.Values{}
	if depth > 0 {
		q.Set("depth", strconv.Itoa(depth))
	}
	return c.getRaw(ctx, "/lineage/assets/"+assetID, q)
}
