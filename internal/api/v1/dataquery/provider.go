package dataquery

import (
	"context"
	"errors"

	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/internal/core/runs"
)

// scheduleSourceProvider adapts the ingestion schedule service to the data query
// SourceProvider: a queryable source is an ingestion source with querying
// switched on, so one connection both catalogs and answers queries.
type scheduleSourceProvider struct {
	schedules *runs.ScheduleService
}

// NewScheduleSourceProvider builds the provider used to wire data queries.
func NewScheduleSourceProvider(schedules *runs.ScheduleService) dataquery.SourceProvider {
	return &scheduleSourceProvider{schedules: schedules}
}

func (p *scheduleSourceProvider) ListQueryableSources(ctx context.Context, search string, limit, offset int) ([]*dataquery.Source, int, error) {
	schedules, total, err := p.schedules.ListQueryableSchedules(ctx, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	sources := make([]*dataquery.Source, 0, len(schedules))
	for _, s := range schedules {
		sources = append(sources, sourceFromSchedule(s))
	}
	return sources, total, nil
}

func (p *scheduleSourceProvider) GetQueryableSource(ctx context.Context, name string) (*dataquery.Source, error) {
	s, err := p.schedules.GetScheduleByName(ctx, name)
	if err != nil {
		if errors.Is(err, runs.ErrScheduleNotFound) {
			return nil, dataquery.ErrSourceNotFound
		}
		return nil, err
	}
	if !s.Queryable {
		return nil, dataquery.ErrSourceNotFound
	}
	return sourceFromSchedule(s), nil
}

func sourceFromSchedule(s *runs.Schedule) *dataquery.Source {
	return &dataquery.Source{
		ID:            s.ID,
		Name:          s.Name,
		PluginID:      s.PluginID,
		IngestOnQuery: s.IngestOnQuery && !s.Served(),
		Served:        s.Served(),
		Config:        s.Config,
	}
}

// scheduleIngestTrigger implements dataquery.IngestTrigger by enqueuing a normal
// ingestion job run for the source, the same path a manual trigger takes, so
// the dispatcher runs it under the usual worker limits.
type scheduleIngestTrigger struct {
	schedules *runs.ScheduleService
}

// NewScheduleIngestTrigger builds the ingest-on-query trigger.
func NewScheduleIngestTrigger(schedules *runs.ScheduleService) dataquery.IngestTrigger {
	return &scheduleIngestTrigger{schedules: schedules}
}

func (t *scheduleIngestTrigger) TriggerIngest(ctx context.Context, sourceID string) error {
	_, err := t.schedules.CreateJobRun(ctx, &sourceID, "query")
	return err
}
