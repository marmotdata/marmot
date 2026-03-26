package client

import (
	"context"
)

// ReindexStatus represents the status of a reindex operation.
type ReindexStatus struct {
	Status    string `json:"status"`
	Total     int    `json:"total,omitempty"`
	Processed int    `json:"processed,omitempty"`
	Errors    int    `json:"errors,omitempty"`
}

// StartReindex triggers a search reindex.
func (c *Client) StartReindex(ctx context.Context) (*ReindexStatus, error) {
	var status ReindexStatus
	if err := c.post(ctx, "/admin/search/reindex", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// GetReindexStatus retrieves the current reindex status.
func (c *Client) GetReindexStatus(ctx context.Context) (*ReindexStatus, error) {
	var status ReindexStatus
	if err := c.get(ctx, "/admin/search/reindex", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}
