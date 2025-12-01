package runs

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// parseCronExpression parses a cron expression and returns the schedule
func parseCronExpression(cronExpr string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	return parser.Parse(cronExpr)
}

type ScheduleService struct {
	repo        ScheduleRepository
	broadcaster EventBroadcaster
}

func NewScheduleService(repo ScheduleRepository) *ScheduleService {
	return &ScheduleService{
		repo:        repo,
		broadcaster: &NoopBroadcaster{},
	}
}

// SetBroadcaster sets the event broadcaster for this service
func (s *ScheduleService) SetBroadcaster(broadcaster EventBroadcaster) {
	s.broadcaster = broadcaster
}

// Schedule operations

func (s *ScheduleService) CreateSchedule(ctx context.Context, name, pluginID string, config map[string]interface{}, cronExpression string, enabled bool, createdBy *string) (*Schedule, error) {
	schedule := &Schedule{
		Name:           name,
		PluginID:       pluginID,
		Config:         config,
		CronExpression: cronExpression,
		Enabled:        enabled,
		CreatedBy:      createdBy,
	}

	if err := s.repo.CreateSchedule(ctx, schedule); err != nil {
		return nil, err
	}

	return schedule, nil
}

func (s *ScheduleService) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	return s.repo.GetSchedule(ctx, id)
}

func (s *ScheduleService) GetScheduleByName(ctx context.Context, name string) (*Schedule, error) {
	return s.repo.GetScheduleByName(ctx, name)
}

func (s *ScheduleService) UpdateSchedule(ctx context.Context, id string, name, pluginID string, config map[string]interface{}, cronExpression string, enabled bool) (*Schedule, error) {
	existing, err := s.repo.GetSchedule(ctx, id)
	if err != nil {
		return nil, err
	}

	existing.Name = name
	existing.PluginID = pluginID
	existing.Config = config
	existing.CronExpression = cronExpression
	existing.Enabled = enabled

	if err := s.repo.UpdateSchedule(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *ScheduleService) DeleteSchedule(ctx context.Context, id string) error {
	return s.repo.DeleteSchedule(ctx, id)
}

func (s *ScheduleService) ListSchedules(ctx context.Context, enabled *bool, limit, offset int) ([]*Schedule, int, error) {
	return s.repo.ListSchedules(ctx, enabled, limit, offset)
}

func (s *ScheduleService) GetSchedulesDueForRun(ctx context.Context, limit int) ([]*Schedule, error) {
	return s.repo.GetSchedulesDueForRun(ctx, limit)
}

// CalculateNextRun calculates the next run time for a schedule
func (s *ScheduleService) CalculateNextRun(cronExpression string, fromTime time.Time) (time.Time, error) {
	cronSchedule, err := parseCronExpression(cronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression: %w", err)
	}
	return cronSchedule.Next(fromTime), nil
}

// UpdateScheduleNextRun updates the next_run_at timestamp for a schedule
func (s *ScheduleService) UpdateScheduleNextRun(ctx context.Context, id string, nextRunAt time.Time) error {
	return s.repo.UpdateScheduleNextRun(ctx, id, nextRunAt)
}

// UpdateScheduleLastRun updates the last_run_at timestamp for a schedule
func (s *ScheduleService) UpdateScheduleLastRun(ctx context.Context, id string, lastRunAt time.Time) error {
	return s.repo.UpdateScheduleLastRun(ctx, id, lastRunAt)
}

// Job run operations

func (s *ScheduleService) CreateJobRun(ctx context.Context, scheduleID *string) (*JobRun, error) {
	run := &JobRun{
		ScheduleID: scheduleID,
		Status:     JobStatusPending,
	}

	if err := s.repo.CreateJobRun(ctx, run); err != nil {
		return nil, err
	}

	// Broadcast event
	s.broadcaster.BroadcastJobRunCreated(run)

	return run, nil
}

func (s *ScheduleService) GetJobRun(ctx context.Context, id string) (*JobRun, error) {
	return s.repo.GetJobRun(ctx, id)
}

func (s *ScheduleService) ListJobRuns(ctx context.Context, scheduleID *string, status *string, limit, offset int) ([]*JobRun, int, error) {
	return s.repo.ListJobRuns(ctx, scheduleID, status, limit, offset)
}

func (s *ScheduleService) ClaimJobRun(ctx context.Context, id, workerID string) (*JobRun, error) {
	run, err := s.repo.ClaimJobRun(ctx, id, workerID)
	if err != nil {
		return nil, err
	}

	// Broadcast event
	s.broadcaster.BroadcastJobRunClaimed(run)

	return run, nil
}

func (s *ScheduleService) StartJobRun(ctx context.Context, id string) error {
	run, err := s.repo.GetJobRun(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()
	run.Status = JobStatusRunning
	run.StartedAt = &now

	if err := s.repo.UpdateJobRun(ctx, run); err != nil {
		return err
	}

	// Broadcast event
	s.broadcaster.BroadcastJobRunStarted(run)

	return nil
}

func (s *ScheduleService) UpdateJobRunProgress(ctx context.Context, id string, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded int) error {
	if err := s.repo.UpdateJobRunProgress(ctx, id, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded); err != nil {
		return err
	}

	// Get updated run for broadcasting
	run, err := s.repo.GetJobRun(ctx, id)
	if err == nil {
		s.broadcaster.BroadcastJobRunProgress(run)
	}

	return nil
}

func (s *ScheduleService) CompleteJobRun(ctx context.Context, id string, success bool, errorMessage *string, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded int) error {
	status := JobStatusSucceeded
	if !success {
		status = JobStatusFailed
	}

	if err := s.repo.CompleteJobRun(ctx, id, status, errorMessage, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded); err != nil {
		return err
	}

	// Get updated run for broadcasting
	run, err := s.repo.GetJobRun(ctx, id)
	if err == nil {
		s.broadcaster.BroadcastJobRunCompleted(run)
	}

	return nil
}

func (s *ScheduleService) CancelJobRun(ctx context.Context, id string) error {
	if err := s.repo.CancelJobRun(ctx, id); err != nil {
		return err
	}

	// Get updated run for broadcasting
	run, err := s.repo.GetJobRun(ctx, id)
	if err == nil {
		s.broadcaster.BroadcastJobRunCancelled(run)
	}

	return nil
}

func (s *ScheduleService) ReleaseExpiredClaims(ctx context.Context, expiry time.Duration) (int, error) {
	return s.repo.ReleaseExpiredClaims(ctx, expiry)
}

// GetJobRunPluginRunID gets the plugin run ID for a job run
func (s *ScheduleService) GetJobRunPluginRunID(ctx context.Context, jobRunID string) (*string, error) {
	run, err := s.repo.GetJobRun(ctx, jobRunID)
	if err != nil {
		return nil, err
	}
	return run.PluginRunID, nil
}

// SetJobRunPluginRunID sets the plugin run ID for a job run
func (s *ScheduleService) SetJobRunPluginRunID(ctx context.Context, jobRunID, pluginRunID string) error {
	return s.repo.SetJobRunPluginRunID(ctx, jobRunID, pluginRunID)
}
