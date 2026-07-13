package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/ingestion"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// Schedule is a single ingestion schedule: a plugin, its config and a cron
// expression that together discover and catalog assets on a recurring basis.
type Schedule = models.Schedule

// ScheduleList is a paginated set of ingestion schedules.
type ScheduleList = models.ListSchedulesResponse

// SchedulesListOptions filters IngestionService.ListSchedules.
type SchedulesListOptions struct {
	// Enabled, when set, returns only enabled or only disabled schedules.
	Enabled *bool
	Limit   int64
	Offset  int64
}

// CreateScheduleInput is the input for IngestionService.CreateSchedule.
type CreateScheduleInput struct {
	Name           string
	PluginID       string
	Config         map[string]any
	CronExpression string
	Enabled        bool
}

// UpdateScheduleInput is the input for IngestionService.UpdateSchedule.
type UpdateScheduleInput struct {
	Name           string
	PluginID       string
	Config         map[string]any
	CronExpression string
	Enabled        bool
}

// IngestionService manages ingestion schedules, the plugin-driven pipelines
// that populate the catalog.
type IngestionService struct {
	gen *apiclient.Marmot
}

// ListSchedules returns paginated ingestion schedules.
func (s *IngestionService) ListSchedules(ctx context.Context, opts SchedulesListOptions) (*ScheduleList, error) {
	p := ingestion.NewGetIngestionSchedulesParams().WithContext(ctx)
	if opts.Enabled != nil {
		p = p.WithEnabled(opts.Enabled)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Ingestion.GetIngestionSchedules(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// GetSchedule fetches a single ingestion schedule by ID.
func (s *IngestionService) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	p := ingestion.NewGetIngestionSchedulesIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Ingestion.GetIngestionSchedulesID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// CreateSchedule creates a new ingestion schedule. The server validates Config
// against the plugin identified by PluginID and rejects an invalid config.
func (s *IngestionService) CreateSchedule(ctx context.Context, in CreateScheduleInput) (*Schedule, error) {
	body := &models.CreateScheduleRequest{
		Name:           in.Name,
		PluginID:       in.PluginID,
		Config:         in.Config,
		CronExpression: in.CronExpression,
		Enabled:        in.Enabled,
	}
	p := ingestion.NewPostIngestionSchedulesParams().WithContext(ctx).WithSchedule(body)
	resp, err := s.gen.Ingestion.PostIngestionSchedules(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// UpdateSchedule replaces an existing ingestion schedule. The update is a full
// replace: every field is set from the input, so pass the complete desired state.
func (s *IngestionService) UpdateSchedule(ctx context.Context, id string, in UpdateScheduleInput) (*Schedule, error) {
	body := &models.UpdateScheduleRequest{
		Name:           in.Name,
		PluginID:       in.PluginID,
		Config:         in.Config,
		CronExpression: in.CronExpression,
		Enabled:        in.Enabled,
	}
	p := ingestion.NewPutIngestionSchedulesIDParams().WithContext(ctx).WithID(id).WithSchedule(body)
	resp, err := s.gen.Ingestion.PutIngestionSchedulesID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// DeleteSchedule removes an ingestion schedule.
func (s *IngestionService) DeleteSchedule(ctx context.Context, id string) error {
	p := ingestion.NewDeleteIngestionSchedulesIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Ingestion.DeleteIngestionSchedulesID(p)
	return mapErr(err)
}

// TriggerSchedule runs an ingestion schedule immediately, outside its cron
// cadence.
func (s *IngestionService) TriggerSchedule(ctx context.Context, id string) error {
	p := ingestion.NewPostIngestionSchedulesIDTriggerParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Ingestion.PostIngestionSchedulesIDTrigger(p)
	return mapErr(err)
}
