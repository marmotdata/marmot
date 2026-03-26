package client

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// SearchResult represents a single result from the unified search endpoint.
type SearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	// Asset-specific fields
	AssetType string   `json:"asset_type,omitempty"`
	Providers []string `json:"providers,omitempty"`
}

// SearchResponse represents the response from the unified search endpoint.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
}

// Search performs a unified search across assets, glossary, teams, and users.
func (c *Client) Search(ctx context.Context, query string, types []string, limit, offset int) (*SearchResponse, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	if len(types) > 0 {
		q.Set("types", strings.Join(types, ","))
	}

	var resp SearchResponse
	if err := c.get(ctx, "/search", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchRaw performs a unified search and returns raw JSON bytes.
func (c *Client) SearchRaw(ctx context.Context, query string, types []string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	if len(types) > 0 {
		q.Set("types", strings.Join(types, ","))
	}

	return c.getRaw(ctx, "/search", q)
}
