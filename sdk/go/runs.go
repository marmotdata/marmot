package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/runs"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// Run is a single pipeline ingestion run.
type Run = models.PluginRun

// RunList is a paginated set of runs.
type RunList = runs.GetRunsOKBody

// RunEntities is the response from RunsService.Entities.
type RunEntities = models.V1RunsRunEntitiesResponse

// RunsListOptions filters RunsService.List. Pipelines and Statuses are comma-separated.
type RunsListOptions struct {
	Pipelines string
	Statuses  string
	Limit     int64
	Offset    int64
}

// RunEntitiesOptions filters RunsService.Entities.
type RunEntitiesOptions struct {
	EntityType string
	Status     string
	Limit      int64
	Offset     int64
}

// RunsService lists and inspects pipeline ingestion runs.
type RunsService struct {
	gen *apiclient.Marmot
}

// List returns paginated runs.
func (s *RunsService) List(ctx context.Context, opts RunsListOptions) (*RunList, error) {
	p := runs.NewGetRunsParams().WithContext(ctx)
	if opts.Pipelines != "" {
		p = p.WithPipelines(&opts.Pipelines)
	}
	if opts.Statuses != "" {
		p = p.WithStatuses(&opts.Statuses)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Runs.GetRuns(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a single run by ID.
func (s *RunsService) Get(ctx context.Context, id string) (*Run, error) {
	p := runs.NewGetRunsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Runs.GetRunsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Entities lists the entities processed in a run.
func (s *RunsService) Entities(ctx context.Context, runID string, opts RunEntitiesOptions) (*RunEntities, error) {
	p := runs.NewGetRunsIDEntitiesParams().WithContext(ctx).WithID(runID)
	if opts.EntityType != "" {
		p = p.WithEntityType(&opts.EntityType)
	}
	if opts.Status != "" {
		p = p.WithStatus(&opts.Status)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Runs.GetRunsIDEntities(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
