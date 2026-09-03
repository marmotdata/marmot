package gateway

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/gateway"
	"github.com/marmotdata/marmot/internal/core/runs"
)

// scheduleTargetProvider adapts the ingestion schedule service to the
// gateway's TargetProvider: a query target is a queryable source. This is the
// single seam that keeps the gateway core unaware of the ingestion domain
// while letting one connection both catalogue and serve queries.
type scheduleTargetProvider struct {
	schedules *runs.ScheduleService
}

// NewScheduleTargetProvider builds the provider used to wire the gateway.
func NewScheduleTargetProvider(schedules *runs.ScheduleService) gateway.TargetProvider {
	return &scheduleTargetProvider{schedules: schedules}
}

// scheduleIngestTrigger implements gateway.IngestTrigger by enqueuing a normal
// ingestion job run for the source, the same path a manual trigger uses. The
// dispatcher then runs it under the usual worker limits.
type scheduleIngestTrigger struct {
	schedules *runs.ScheduleService
}

// NewScheduleIngestTrigger builds the ingest-on-query trigger.
func NewScheduleIngestTrigger(schedules *runs.ScheduleService) gateway.IngestTrigger {
	return &scheduleIngestTrigger{schedules: schedules}
}

func (t *scheduleIngestTrigger) TriggerIngest(ctx context.Context, sourceID string) error {
	_, err := t.schedules.CreateJobRun(ctx, &sourceID, "gateway:ingest-on-query")
	return err
}

func (p *scheduleTargetProvider) ListQueryTargets(ctx context.Context) ([]*gateway.Target, error) {
	schedules, _, err := p.schedules.ListSchedules(ctx, nil, 500, 0)
	if err != nil {
		return nil, err
	}
	var targets []*gateway.Target
	for _, s := range schedules {
		if !s.Queryable {
			continue
		}
		targets = append(targets, targetFromSchedule(s))
	}
	return targets, nil
}

func (p *scheduleTargetProvider) GetQueryTarget(ctx context.Context, name string) (*gateway.Target, error) {
	s, err := p.schedules.GetScheduleByName(ctx, name)
	if err != nil {
		return nil, gateway.ErrTargetNotFound
	}
	if !s.Queryable {
		return nil, gateway.ErrTargetNotFound
	}
	return targetFromSchedule(s), nil
}

func targetFromSchedule(s *runs.Schedule) *gateway.Target {
	modes := s.QueryModes
	if len(modes) == 0 {
		modes = []string{"direct"}
	}
	return &gateway.Target{
		ID:            s.ID,
		Name:          s.Name,
		PluginID:      s.PluginID,
		Modes:         modes,
		Config:        s.Config,
		Enabled:       true, // only queryable sources are returned
		IngestOnQuery: s.IngestOnQuery,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}
