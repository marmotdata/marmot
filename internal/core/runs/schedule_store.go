package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/robfig/cron/v3"
)

// Job run status constants
const (
	JobStatusPending   = "pending"
	JobStatusClaimed   = "claimed"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"
)

var (
	ErrScheduleNotFound      = errors.New("schedule not found")
	ErrScheduleNameExists    = errors.New("schedule name already exists")
	ErrJobRunNotFound        = errors.New("job run not found")
	ErrJobRunNotClaimable    = errors.New("job run not claimable")
	ErrInvalidJobStatus      = errors.New("invalid job status")
	ErrInvalidCronExpression = errors.New("invalid cron expression")
)

type Schedule struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	PluginID       string                 `json:"plugin_id"`
	Config         map[string]interface{} `json:"config"`
	CronExpression string                 `json:"cron_expression"`
	Enabled        bool                   `json:"enabled"`
	LastRunAt      *time.Time             `json:"last_run_at,omitempty"`
	LastRunStatus  *string                `json:"last_run_status,omitempty"`
	NextRunAt      *time.Time             `json:"next_run_at,omitempty"`
	CreatedBy      *string                `json:"created_by,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type JobRun struct {
	ID                 string     `json:"id"`
	ScheduleID         *string    `json:"schedule_id,omitempty"`
	PluginRunID        *string    `json:"plugin_run_id,omitempty"`
	PipelineName       string     `json:"pipeline_name"`
	SourceName         string     `json:"source_name"`
	RunID              string     `json:"run_id"`
	Status             string     `json:"status"`
	ClaimedBy          *string    `json:"claimed_by,omitempty"`
	ClaimedAt          *time.Time `json:"claimed_at,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	Log                *string    `json:"log,omitempty"`
	ErrorMessage       *string    `json:"error_message,omitempty"`
	AssetsCreated      int        `json:"assets_created"`
	AssetsUpdated      int        `json:"assets_updated"`
	AssetsDeleted      int        `json:"assets_deleted"`
	LineageCreated     int        `json:"lineage_created"`
	DocumentationAdded int        `json:"documentation_added"`
	Config             map[string]interface{} `json:"config,omitempty"`
	CreatedBy          string     `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ValidJobStatus checks if a job status is valid
func ValidJobStatus(status string) bool {
	switch status {
	case JobStatusPending, JobStatusClaimed, JobStatusRunning, JobStatusSucceeded, JobStatusFailed, JobStatusCancelled:
		return true
	default:
		return false
	}
}

// ScheduleRepository defines the interface for schedule data access
type ScheduleRepository interface {
	// Schedule operations
	CreateSchedule(ctx context.Context, schedule *Schedule) error
	GetSchedule(ctx context.Context, id string) (*Schedule, error)
	GetScheduleByName(ctx context.Context, name string) (*Schedule, error)
	UpdateSchedule(ctx context.Context, schedule *Schedule) error
	DeleteSchedule(ctx context.Context, id string) error
	ListSchedules(ctx context.Context, enabled *bool, limit, offset int) ([]*Schedule, int, error)
	UpdateScheduleNextRun(ctx context.Context, id string, nextRunAt time.Time) error
	UpdateScheduleLastRun(ctx context.Context, id string, lastRunAt time.Time) error
	GetSchedulesDueForRun(ctx context.Context, limit int) ([]*Schedule, error)

	// Job run operations
	CreateJobRun(ctx context.Context, run *JobRun) error
	GetJobRun(ctx context.Context, id string) (*JobRun, error)
	UpdateJobRun(ctx context.Context, run *JobRun) error
	ListJobRuns(ctx context.Context, scheduleID *string, status *string, limit, offset int) ([]*JobRun, int, error)
	ClaimJobRun(ctx context.Context, id, workerID string) (*JobRun, error)
	UpdateJobRunStatus(ctx context.Context, id, status string) error
	UpdateJobRunProgress(ctx context.Context, id string, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded int) error
	SetJobRunPluginRunID(ctx context.Context, jobRunID, pluginRunID string) error
	CompleteJobRun(ctx context.Context, id string, status string, errorMessage *string, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded int) error
	ReleaseExpiredClaims(ctx context.Context, expiry time.Duration) (int, error)
	CancelJobRun(ctx context.Context, id string) error
}

type SchedulePostgresRepository struct {
	db *pgxpool.Pool
}

func NewSchedulePostgresRepository(db *pgxpool.Pool) ScheduleRepository {
	return &SchedulePostgresRepository{db: db}
}

// validateCronExpression validates a cron expression and returns the next run time
func validateCronExpression(cronExpr string) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(time.Now()), nil
}

// Schedule operations

func (r *SchedulePostgresRepository) CreateSchedule(ctx context.Context, schedule *Schedule) error {
	// Validate cron expression and calculate next run time if provided
	// Empty cron expression means manual-only pipeline
	if schedule.CronExpression != "" {
		nextRun, err := validateCronExpression(schedule.CronExpression)
		if err != nil {
			return ErrInvalidCronExpression
		}
		schedule.NextRunAt = &nextRun
	}

	configJSON, err := json.Marshal(schedule.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		INSERT INTO ingestion_schedules (name, plugin_id, config, cron_expression, enabled, next_run_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	err = r.db.QueryRow(ctx, query,
		schedule.Name,
		schedule.PluginID,
		configJSON,
		schedule.CronExpression,
		schedule.Enabled,
		schedule.NextRunAt,
		schedule.CreatedBy,
	).Scan(&schedule.ID, &schedule.CreatedAt, &schedule.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrScheduleNameExists
		}
		return fmt.Errorf("failed to create schedule: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	query := `
		SELECT id, name, plugin_id, config, cron_expression, enabled, last_run_at, next_run_at, created_by, created_at, updated_at
		FROM ingestion_schedules
		WHERE id = $1`

	schedule := &Schedule{}
	var configJSON []byte
	err := r.db.QueryRow(ctx, query, id).Scan(
		&schedule.ID,
		&schedule.Name,
		&schedule.PluginID,
		&configJSON,
		&schedule.CronExpression,
		&schedule.Enabled,
		&schedule.LastRunAt,
		&schedule.NextRunAt,
		&schedule.CreatedBy,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScheduleNotFound
		}
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	if err := json.Unmarshal(configJSON, &schedule.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return schedule, nil
}

func (r *SchedulePostgresRepository) GetScheduleByName(ctx context.Context, name string) (*Schedule, error) {
	query := `
		SELECT id, name, plugin_id, config, cron_expression, enabled, last_run_at, next_run_at, created_by, created_at, updated_at
		FROM ingestion_schedules
		WHERE name = $1`

	schedule := &Schedule{}
	var configJSON []byte
	err := r.db.QueryRow(ctx, query, name).Scan(
		&schedule.ID,
		&schedule.Name,
		&schedule.PluginID,
		&configJSON,
		&schedule.CronExpression,
		&schedule.Enabled,
		&schedule.LastRunAt,
		&schedule.NextRunAt,
		&schedule.CreatedBy,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScheduleNotFound
		}
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	if err := json.Unmarshal(configJSON, &schedule.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return schedule, nil
}

func (r *SchedulePostgresRepository) UpdateSchedule(ctx context.Context, schedule *Schedule) error {
	// Validate cron expression if provided (empty means manual-only pipeline)
	if schedule.CronExpression != "" {
		if _, err := validateCronExpression(schedule.CronExpression); err != nil {
			return ErrInvalidCronExpression
		}
	}

	configJSON, err := json.Marshal(schedule.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		UPDATE ingestion_schedules
		SET name = $1, plugin_id = $2, config = $3, cron_expression = $4, enabled = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING updated_at`

	err = r.db.QueryRow(ctx, query,
		schedule.Name,
		schedule.PluginID,
		configJSON,
		schedule.CronExpression,
		schedule.Enabled,
		schedule.ID,
	).Scan(&schedule.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrScheduleNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrScheduleNameExists
		}
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) DeleteSchedule(ctx context.Context, id string) error {
	query := `DELETE FROM ingestion_schedules WHERE id = $1 RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrScheduleNotFound
		}
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) ListSchedules(ctx context.Context, enabled *bool, limit, offset int) ([]*Schedule, int, error) {
	var countQuery string
	var listQuery string
	var args []interface{}

	if enabled != nil {
		countQuery = `SELECT COUNT(*) FROM ingestion_schedules WHERE enabled = $1`
		listQuery = `
			SELECT
				s.id, s.name, s.plugin_id, s.config, s.cron_expression, s.enabled,
				s.last_run_at, s.next_run_at, s.created_by, s.created_at, s.updated_at,
				(
					SELECT status
					FROM ingestion_job_runs jr
					WHERE jr.schedule_id = s.id
					ORDER BY jr.created_at DESC
					LIMIT 1
				) as last_run_status
			FROM ingestion_schedules s
			WHERE s.enabled = $1
			ORDER BY s.name
			LIMIT $2 OFFSET $3`
		args = append(args, *enabled)
	} else {
		countQuery = `SELECT COUNT(*) FROM ingestion_schedules`
		listQuery = `
			SELECT
				s.id, s.name, s.plugin_id, s.config, s.cron_expression, s.enabled,
				s.last_run_at, s.next_run_at, s.created_by, s.created_at, s.updated_at,
				(
					SELECT status
					FROM ingestion_job_runs jr
					WHERE jr.schedule_id = s.id
					ORDER BY jr.created_at DESC
					LIMIT 1
				) as last_run_status
			FROM ingestion_schedules s
			ORDER BY s.name
			LIMIT $1 OFFSET $2`
	}

	// Get total count
	var total int
	var countArgs []interface{}
	if enabled != nil {
		countArgs = []interface{}{*enabled}
	}
	err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count schedules: %w", err)
	}

	// Get list
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list schedules: %w", err)
	}
	defer rows.Close()

	schedules := []*Schedule{}
	for rows.Next() {
		schedule := &Schedule{}
		var configJSON []byte
		err := rows.Scan(
			&schedule.ID,
			&schedule.Name,
			&schedule.PluginID,
			&configJSON,
			&schedule.CronExpression,
			&schedule.Enabled,
			&schedule.LastRunAt,
			&schedule.NextRunAt,
			&schedule.CreatedBy,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
			&schedule.LastRunStatus,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan schedule: %w", err)
		}
		if err := json.Unmarshal(configJSON, &schedule.Config); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling config: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	return schedules, total, nil
}

func (r *SchedulePostgresRepository) UpdateScheduleNextRun(ctx context.Context, id string, nextRunAt time.Time) error {
	query := `
		UPDATE ingestion_schedules
		SET next_run_at = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, nextRunAt, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrScheduleNotFound
		}
		return fmt.Errorf("failed to update next run: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) UpdateScheduleLastRun(ctx context.Context, id string, lastRunAt time.Time) error {
	query := `
		UPDATE ingestion_schedules
		SET last_run_at = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, lastRunAt, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrScheduleNotFound
		}
		return fmt.Errorf("failed to update last run: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) GetSchedulesDueForRun(ctx context.Context, limit int) ([]*Schedule, error) {
	query := `
		SELECT id, name, plugin_id, config, cron_expression, enabled, last_run_at, next_run_at, created_by, created_at, updated_at
		FROM ingestion_schedules
		WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= NOW()
		ORDER BY next_run_at
		LIMIT $1`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules due for run: %w", err)
	}
	defer rows.Close()

	schedules := []*Schedule{}
	for rows.Next() {
		schedule := &Schedule{}
		var configJSON []byte
		err := rows.Scan(
			&schedule.ID,
			&schedule.Name,
			&schedule.PluginID,
			&configJSON,
			&schedule.CronExpression,
			&schedule.Enabled,
			&schedule.LastRunAt,
			&schedule.NextRunAt,
			&schedule.CreatedBy,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		if err := json.Unmarshal(configJSON, &schedule.Config); err != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// Job run operations

func (r *SchedulePostgresRepository) CreateJobRun(ctx context.Context, run *JobRun) error {
	if !ValidJobStatus(run.Status) {
		return ErrInvalidJobStatus
	}

	query := `
		INSERT INTO ingestion_job_runs (schedule_id, status)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, query, run.ScheduleID, run.Status).Scan(
		&run.ID,
		&run.CreatedAt,
		&run.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create job run: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) GetJobRun(ctx context.Context, id string) (*JobRun, error) {
	query := `
		SELECT
			jr.id, jr.schedule_id, jr.plugin_run_id, jr.status, jr.claimed_by, jr.claimed_at, jr.started_at, jr.finished_at,
			jr.log, jr.error_message, jr.assets_created, jr.assets_updated, jr.assets_deleted,
			jr.lineage_created, jr.documentation_added, jr.created_at, jr.updated_at,
			COALESCE(s.name, 'Manual Run') as pipeline_name,
			COALESCE(s.plugin_id, '') as source_name,
			COALESCE(s.config, '{}'::jsonb) as config,
			COALESCE(u.username, '') as created_by
		FROM ingestion_job_runs jr
		LEFT JOIN ingestion_schedules s ON jr.schedule_id = s.id
		LEFT JOIN users u ON s.created_by = u.id::text
		WHERE jr.id = $1`

	run := &JobRun{}
	var configJSON []byte
	err := r.db.QueryRow(ctx, query, id).Scan(
		&run.ID,
		&run.ScheduleID,
		&run.PluginRunID,
		&run.Status,
		&run.ClaimedBy,
		&run.ClaimedAt,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Log,
		&run.ErrorMessage,
		&run.AssetsCreated,
		&run.AssetsUpdated,
		&run.AssetsDeleted,
		&run.LineageCreated,
		&run.DocumentationAdded,
		&run.CreatedAt,
		&run.UpdatedAt,
		&run.PipelineName,
		&run.SourceName,
		&configJSON,
		&run.CreatedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobRunNotFound
		}
		return nil, fmt.Errorf("failed to get job run: %w", err)
	}

	// Set RunID same as ID for now
	run.RunID = run.ID

	// Unmarshal config
	if err := json.Unmarshal(configJSON, &run.Config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Mask sensitive fields in config
	r.maskJobRunConfig(run)

	return run, nil
}

func (r *SchedulePostgresRepository) UpdateJobRun(ctx context.Context, run *JobRun) error {
	if !ValidJobStatus(run.Status) {
		return ErrInvalidJobStatus
	}

	query := `
		UPDATE ingestion_job_runs
		SET status = $1, claimed_by = $2, claimed_at = $3, started_at = $4, finished_at = $5,
			log = $6, error_message = $7, assets_created = $8, assets_updated = $9, assets_deleted = $10,
			lineage_created = $11, documentation_added = $12, updated_at = NOW()
		WHERE id = $13
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, query,
		run.Status,
		run.ClaimedBy,
		run.ClaimedAt,
		run.StartedAt,
		run.FinishedAt,
		run.Log,
		run.ErrorMessage,
		run.AssetsCreated,
		run.AssetsUpdated,
		run.AssetsDeleted,
		run.LineageCreated,
		run.DocumentationAdded,
		run.ID,
	).Scan(&run.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobRunNotFound
		}
		return fmt.Errorf("failed to update job run: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) ListJobRuns(ctx context.Context, scheduleID *string, status *string, limit, offset int) ([]*JobRun, int, error) {
	// Build dynamic query based on filters
	var countQuery string
	var listQuery string
	var countArgs []interface{}
	var listArgs []interface{}
	argPos := 1

	whereConditions := []string{}
	if scheduleID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("jr.schedule_id = $%d", argPos))
		countArgs = append(countArgs, *scheduleID)
		listArgs = append(listArgs, *scheduleID)
		argPos++
	}
	if status != nil {
		if !ValidJobStatus(*status) {
			return nil, 0, ErrInvalidJobStatus
		}
		whereConditions = append(whereConditions, fmt.Sprintf("jr.status = $%d", argPos))
		countArgs = append(countArgs, *status)
		listArgs = append(listArgs, *status)
		argPos++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + whereConditions[0]
		for i := 1; i < len(whereConditions); i++ {
			whereClause += " AND " + whereConditions[i]
		}
	}

	countQuery = fmt.Sprintf("SELECT COUNT(*) FROM ingestion_job_runs jr %s", whereClause)
	listQuery = fmt.Sprintf(`
		SELECT
			jr.id, jr.schedule_id, jr.plugin_run_id, jr.status, jr.claimed_by, jr.claimed_at, jr.started_at, jr.finished_at,
			jr.log, jr.error_message, jr.assets_created, jr.assets_updated, jr.assets_deleted,
			jr.lineage_created, jr.documentation_added, jr.created_at, jr.updated_at,
			COALESCE(s.name, 'Manual Run') as pipeline_name,
			COALESCE(s.plugin_id, '') as source_name,
			COALESCE(s.config, '{}'::jsonb) as config,
			COALESCE(u.username, '') as created_by
		FROM ingestion_job_runs jr
		LEFT JOIN ingestion_schedules s ON jr.schedule_id = s.id
		LEFT JOIN users u ON s.created_by = u.id::text
		%s
		ORDER BY jr.created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, argPos, argPos+1)

	// Get total count
	var total int
	err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count job runs: %w", err)
	}

	// Get list
	listArgs = append(listArgs, limit, offset)
	rows, err := r.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list job runs: %w", err)
	}
	defer rows.Close()

	runs := []*JobRun{}
	for rows.Next() {
		run := &JobRun{}
		var configJSON []byte
		err := rows.Scan(
			&run.ID,
			&run.ScheduleID,
			&run.PluginRunID,
			&run.Status,
			&run.ClaimedBy,
			&run.ClaimedAt,
			&run.StartedAt,
			&run.FinishedAt,
			&run.Log,
			&run.ErrorMessage,
			&run.AssetsCreated,
			&run.AssetsUpdated,
			&run.AssetsDeleted,
			&run.LineageCreated,
			&run.DocumentationAdded,
			&run.CreatedAt,
			&run.UpdatedAt,
			&run.PipelineName,
			&run.SourceName,
			&configJSON,
			&run.CreatedBy,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan job run: %w", err)
		}

		// Set RunID same as ID for now
		run.RunID = run.ID

		// Unmarshal config
		if err := json.Unmarshal(configJSON, &run.Config); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling config: %w", err)
		}

		// Mask sensitive fields in config
		r.maskJobRunConfig(run)

		runs = append(runs, run)
	}

	return runs, total, nil
}

func (r *SchedulePostgresRepository) ClaimJobRun(ctx context.Context, id, workerID string) (*JobRun, error) {
	// Attempt to claim the job using UPDATE...RETURNING, then fetch full details
	updateQuery := `
		UPDATE ingestion_job_runs
		SET status = $1, claimed_by = $2, claimed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND status = $4
		RETURNING id`

	var runID string
	err := r.db.QueryRow(ctx, updateQuery, JobStatusClaimed, workerID, id, JobStatusPending).Scan(&runID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobRunNotClaimable
		}
		return nil, fmt.Errorf("failed to claim job run: %w", err)
	}

	// Now fetch the full run with joined data
	return r.GetJobRun(ctx, runID)
}

func (r *SchedulePostgresRepository) UpdateJobRunStatus(ctx context.Context, id, status string) error {
	if !ValidJobStatus(status) {
		return ErrInvalidJobStatus
	}

	query := `
		UPDATE ingestion_job_runs
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, status, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobRunNotFound
		}
		return fmt.Errorf("failed to update job run status: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) SetJobRunPluginRunID(ctx context.Context, jobRunID, pluginRunID string) error {
	query := `
		UPDATE ingestion_job_runs
		SET plugin_run_id = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, pluginRunID, jobRunID).Scan(&returnedID)
	if err != nil {
		return fmt.Errorf("setting plugin run ID: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) UpdateJobRunProgress(ctx context.Context, id string, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded int) error {
	query := `
		UPDATE ingestion_job_runs
		SET assets_created = $1, assets_updated = $2, assets_deleted = $3,
			lineage_created = $4, documentation_added = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query,
		assetsCreated,
		assetsUpdated,
		assetsDeleted,
		lineageCreated,
		documentationAdded,
		id,
	).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobRunNotFound
		}
		return fmt.Errorf("failed to update job run progress: %w", err)
	}

	return nil
}

func (r *SchedulePostgresRepository) CompleteJobRun(ctx context.Context, id string, status string, errorMessage *string, assetsCreated, assetsUpdated, assetsDeleted, lineageCreated, documentationAdded int) error {
	if status != JobStatusSucceeded && status != JobStatusFailed {
		return ErrInvalidJobStatus
	}

	query := `
		UPDATE ingestion_job_runs
		SET status = $1, finished_at = NOW(), error_message = $2,
			assets_created = $3, assets_updated = $4, assets_deleted = $5,
			lineage_created = $6, documentation_added = $7, updated_at = NOW()
		WHERE id = $8
		RETURNING id, schedule_id`

	var returnedID string
	var scheduleID *string
	err := r.db.QueryRow(ctx, query,
		status,
		errorMessage,
		assetsCreated,
		assetsUpdated,
		assetsDeleted,
		lineageCreated,
		documentationAdded,
		id,
	).Scan(&returnedID, &scheduleID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobRunNotFound
		}
		return fmt.Errorf("failed to complete job run: %w", err)
	}

	if scheduleID != nil {
		updateScheduleQuery := `
			UPDATE ingestion_schedules
			SET last_run_at = NOW(), updated_at = NOW()
			WHERE id = $1`

		_, err := r.db.Exec(ctx, updateScheduleQuery, *scheduleID)
		if err != nil {
			return fmt.Errorf("failed to update schedule last_run_at: %w", err)
		}
	}

	return nil
}

func (r *SchedulePostgresRepository) ReleaseExpiredClaims(ctx context.Context, expiry time.Duration) (int, error) {
	expiryTime := time.Now().Add(-expiry)
	query := `
		UPDATE ingestion_job_runs
		SET status = $1, claimed_by = NULL, claimed_at = NULL, updated_at = NOW()
		WHERE status = $2 AND claimed_at < $3`

	result, err := r.db.Exec(ctx, query, JobStatusPending, JobStatusClaimed, expiryTime)
	if err != nil {
		return 0, fmt.Errorf("failed to release expired claims: %w", err)
	}

	return int(result.RowsAffected()), nil
}

func (r *SchedulePostgresRepository) CancelJobRun(ctx context.Context, id string) error {
	query := `
		UPDATE ingestion_job_runs
		SET status = $1, finished_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND status IN ($3, $4, $5)
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, JobStatusCancelled, id, JobStatusPending, JobStatusClaimed, JobStatusRunning).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobRunNotFound
		}
		return fmt.Errorf("failed to cancel job run: %w", err)
	}

	return nil
}

// maskJobRunConfig masks sensitive fields in a job run's config
func (r *SchedulePostgresRepository) maskJobRunConfig(run *JobRun) {
	if run.Config == nil || len(run.Config) == 0 {
		return
	}

	// Get the plugin registry
	registry := plugin.GetRegistry()

	// We need the plugin ID to get the ConfigSpec
	// The SourceName field actually contains the plugin_id
	if run.SourceName == "" {
		return
	}

	// Get the plugin entry from registry
	entry, err := registry.Get(run.SourceName)
	if err != nil {
		// Plugin not found, skip masking
		return
	}

	// Mask sensitive fields using the ConfigSpec
	run.Config = plugin.MaskSensitiveFieldsFromSpec(plugin.RawPluginConfig(run.Config), entry.Meta.ConfigSpec)
}
