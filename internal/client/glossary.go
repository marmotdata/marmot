package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/marmotdata/marmot/internal/core/glossary"
)

// GlossaryListResponse represents the response from the glossary list endpoint.
type GlossaryListResponse struct {
	Terms []*glossary.GlossaryTerm `json:"terms"`
	Total int                      `json:"total"`
}

// CreateTermRequest represents the request body for creating a glossary term.
type CreateTermRequest struct {
	Name         string                 `json:"name"`
	Definition   string                 `json:"definition"`
	Description  string                 `json:"description,omitempty"`
	ParentTermID string                 `json:"parent_term_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateTermRequest represents the request body for updating a glossary term.
type UpdateTermRequest struct {
	Name         *string                `json:"name,omitempty"`
	Definition   *string                `json:"definition,omitempty"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ListGlossaryTerms lists glossary terms with pagination.
func (c *Client) ListGlossaryTerms(ctx context.Context, limit, offset int) (*GlossaryListResponse, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp GlossaryListResponse
	if err := c.get(ctx, "/glossary/list", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetGlossaryTerm retrieves a single glossary term by ID.
func (c *Client) GetGlossaryTerm(ctx context.Context, id string) (*glossary.GlossaryTerm, error) {
	var term glossary.GlossaryTerm
	if err := c.get(ctx, "/glossary/"+id, nil, &term); err != nil {
		return nil, err
	}
	return &term, nil
}

// CreateGlossaryTerm creates a new glossary term.
func (c *Client) CreateGlossaryTerm(ctx context.Context, req CreateTermRequest) (*glossary.GlossaryTerm, error) {
	var term glossary.GlossaryTerm
	if err := c.post(ctx, "/glossary/", req, &term); err != nil {
		return nil, err
	}
	return &term, nil
}

// UpdateGlossaryTerm updates an existing glossary term.
func (c *Client) UpdateGlossaryTerm(ctx context.Context, id string, req UpdateTermRequest) (*glossary.GlossaryTerm, error) {
	var term glossary.GlossaryTerm
	if err := c.put(ctx, "/glossary/"+id, req, &term); err != nil {
		return nil, err
	}
	return &term, nil
}

// DeleteGlossaryTerm deletes a glossary term by ID.
func (c *Client) DeleteGlossaryTerm(ctx context.Context, id string) error {
	return c.del(ctx, "/glossary/"+id, nil, nil)
}

// SearchGlossaryTerms searches glossary terms.
func (c *Client) SearchGlossaryTerms(ctx context.Context, query string, limit, offset int) (*GlossaryListResponse, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp GlossaryListResponse
	if err := c.get(ctx, "/glossary/search", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListGlossaryTermsRaw lists glossary terms and returns raw JSON.
func (c *Client) ListGlossaryTermsRaw(ctx context.Context, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return c.getRaw(ctx, "/glossary/list", q)
}

// GetGlossaryTermRaw retrieves a glossary term as raw JSON.
func (c *Client) GetGlossaryTermRaw(ctx context.Context, id string) ([]byte, error) {
	return c.getRaw(ctx, "/glossary/"+id, nil)
}

// SearchGlossaryTermsRaw searches glossary terms and returns raw JSON.
func (c *Client) SearchGlossaryTermsRaw(ctx context.Context, query string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return c.getRaw(ctx, "/glossary/search", q)
}
