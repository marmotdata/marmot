package client

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/plugin"
)

// RunsListResponse represents the response from the runs list endpoint.
type RunsListResponse struct {
	Runs      []*plugin.Run `json:"runs"`
	Total     int           `json:"total"`
	Pipelines []string      `json:"pipelines"`
}

// RunEntitiesResponse represents the response from the run entities endpoint.
type RunEntitiesResponse struct {
	Entities []runs.RunEntity `json:"entities"`
	Total    int              `json:"total"`
}

// ListRuns lists pipeline runs with optional filters.
func (c *Client) ListRuns(ctx context.Context, pipelines, statuses []string, limit, offset int) (*RunsListResponse, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	if len(pipelines) > 0 {
		q.Set("pipelines", strings.Join(pipelines, ","))
	}
	if len(statuses) > 0 {
		q.Set("statuses", strings.Join(statuses, ","))
	}

	var resp RunsListResponse
	if err := c.get(ctx, "/runs", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRun retrieves a single run by ID.
func (c *Client) GetRun(ctx context.Context, id string) (*plugin.Run, error) {
	var run plugin.Run
	if err := c.get(ctx, "/runs/"+id, nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// GetRunEntities retrieves the entities processed in a run.
func (c *Client) GetRunEntities(ctx context.Context, id string, limit, offset int) (*RunEntitiesResponse, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp RunEntitiesResponse
	if err := c.get(ctx, "/runs/entities/"+id, q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListRunsRaw lists runs and returns raw JSON.
func (c *Client) ListRunsRaw(ctx context.Context, pipelines, statuses []string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	if len(pipelines) > 0 {
		q.Set("pipelines", strings.Join(pipelines, ","))
	}
	if len(statuses) > 0 {
		q.Set("statuses", strings.Join(statuses, ","))
	}
	return c.getRaw(ctx, "/runs", q)
}

// GetRunRaw retrieves a run as raw JSON.
func (c *Client) GetRunRaw(ctx context.Context, id string) ([]byte, error) {
	return c.getRaw(ctx, "/runs/"+id, nil)
}

// GetRunEntitiesRaw retrieves run entities as raw JSON.
func (c *Client) GetRunEntitiesRaw(ctx context.Context, id string, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return c.getRaw(ctx, "/runs/entities/"+id, q)
}
