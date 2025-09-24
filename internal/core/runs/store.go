package runs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

const (
	duplicateKeyErrorCode = "23505"
)

var (
	ErrNotFound = errors.New("run not found")
	ErrConflict = errors.New("run already exists")
)

type RunEntity struct {
	ID           string    `json:"id"`
	RunID        string    `json:"run_id"`
	EntityType   string    `json:"entity_type"`
	EntityMRN    string    `json:"entity_mrn"`
	EntityName   string    `json:"entity_name,omitempty"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, run *plugin.Run) error
	Get(ctx context.Context, id string) (*plugin.Run, error)
	GetByRunID(ctx context.Context, runID string) (*plugin.Run, error)
	Update(ctx context.Context, run *plugin.Run) error
	List(ctx context.Context, pipelineName string, limit, offset int) ([]*plugin.Run, int, error)
	ListWithFilters(ctx context.Context, pipelines, statuses []string, limit, offset int) ([]*plugin.Run, int, []string, error)
	AddCheckpoint(ctx context.Context, runDBID string, checkpoint *plugin.RunCheckpoint) error
	DeleteCheckpoints(ctx context.Context, pipelineName, sourceName string) error
	GetLastRunCheckpoints(ctx context.Context, pipelineName, sourceName string) (map[string]*plugin.RunCheckpoint, error)
	CleanupStaleRuns(ctx context.Context, timeout time.Duration) (int, error)
	AddRunEntity(ctx context.Context, runDBID string, entity *RunEntity) error
	ListRunEntities(ctx context.Context, runDBID, entityType, status string, limit, offset int) ([]*RunEntity, int, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, run *plugin.Run) error {
	configJSON, err := json.Marshal(run.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		INSERT INTO runs (id, pipeline_name, source_name, run_id, status, started_at, config, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = r.db.Exec(ctx, query,
		run.ID, run.PipelineName, run.SourceName, run.RunID,
		run.Status, run.StartedAt, configJSON, run.CreatedBy)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == duplicateKeyErrorCode {
			return ErrConflict
		}
		return fmt.Errorf("inserting run: %w", err)
	}

	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*plugin.Run, error) {
	return r.scanSingleRun(ctx, `
		SELECT id, pipeline_name, source_name, run_id, status, started_at,
		       completed_at, error_message, config, summary, created_by
		FROM runs WHERE id = $1`, id)
}

func (r *PostgresRepository) GetByRunID(ctx context.Context, runID string) (*plugin.Run, error) {
	return r.scanSingleRun(ctx, `
		SELECT id, pipeline_name, source_name, run_id, status, started_at,
		       completed_at, error_message, config, summary, created_by
		FROM runs WHERE run_id = $1`, runID)
}

func (r *PostgresRepository) Update(ctx context.Context, run *plugin.Run) error {
	var summaryJSON []byte
	var err error
	if run.Summary != nil {
		summaryJSON, err = json.Marshal(run.Summary)
		if err != nil {
			return fmt.Errorf("marshaling summary: %w", err)
		}
	}

	query := `
		UPDATE runs 
		SET status = $1, completed_at = $2, error_message = $3, summary = $4
		WHERE id = $5`

	commandTag, err := r.db.Exec(ctx, query,
		run.Status, run.CompletedAt, run.ErrorMessage, summaryJSON, run.ID)

	if err != nil {
		return fmt.Errorf("updating run: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) List(ctx context.Context, pipelineName string, limit, offset int) ([]*plugin.Run, int, error) {
	var total int
	countQuery := "SELECT COUNT(*) FROM runs"
	countArgs := []interface{}{}

	if pipelineName != "" {
		countQuery += " WHERE pipeline_name = $1"
		countArgs = append(countArgs, pipelineName)
	}

	err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting runs: %w", err)
	}

	query := `
		SELECT id, pipeline_name, source_name, run_id, status, started_at,
		       completed_at, error_message, config, summary, created_by
		FROM runs`

	args := []interface{}{}
	if pipelineName != "" {
		query += " WHERE pipeline_name = $1 ORDER BY started_at DESC LIMIT $2 OFFSET $3"
		args = append(args, pipelineName, limit, offset)
	} else {
		query += " ORDER BY started_at DESC LIMIT $1 OFFSET $2"
		args = append(args, limit, offset)
	}

	runs, err := r.scanMultipleRuns(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("scanning runs: %w", err)
	}

	return runs, total, nil
}

func (r *PostgresRepository) AddCheckpoint(ctx context.Context, runDBID string, checkpoint *plugin.RunCheckpoint) error {
	query := `
		INSERT INTO run_checkpoints (id, run_id, entity_type, entity_mrn, operation, source_fields, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (run_id, entity_type, entity_mrn) 
		DO UPDATE SET operation = $5, source_fields = $6, created_at = $7`

	_, err := r.db.Exec(ctx, query,
		checkpoint.ID, runDBID, checkpoint.EntityType, checkpoint.EntityMRN,
		checkpoint.Operation, checkpoint.SourceFields, checkpoint.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting checkpoint: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetLastRunCheckpoints(ctx context.Context, pipelineName, sourceName string) (map[string]*plugin.RunCheckpoint, error) {
	query := `
		WITH last_successful_run AS (
			SELECT id, run_id 
			FROM runs 
			WHERE pipeline_name = $1 AND source_name = $2 AND status = 'completed'
			ORDER BY completed_at DESC 
			LIMIT 1
		)
		SELECT c.id, c.entity_type, c.entity_mrn, c.operation, c.source_fields, c.created_at, r.run_id
		FROM run_checkpoints c
		JOIN last_successful_run r ON c.run_id = r.id`

	rows, err := r.db.Query(ctx, query, pipelineName, sourceName)
	if err != nil {
		return nil, fmt.Errorf("querying checkpoints: %w", err)
	}
	defer rows.Close()

	checkpoints := make(map[string]*plugin.RunCheckpoint)
	for rows.Next() {
		var checkpoint plugin.RunCheckpoint
		err := rows.Scan(
			&checkpoint.ID,
			&checkpoint.EntityType,
			&checkpoint.EntityMRN,
			&checkpoint.Operation,
			&checkpoint.SourceFields,
			&checkpoint.CreatedAt,
			&checkpoint.RunID,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning checkpoint: %w", err)
		}

		checkpoints[checkpoint.EntityMRN] = &checkpoint
	}

	return checkpoints, nil
}

func (r *PostgresRepository) CleanupStaleRuns(ctx context.Context, timeout time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-timeout)

	query := `
		UPDATE runs 
		SET status = 'failed', 
		    completed_at = NOW(), 
		    error_message = 'Run timed out - CLI may have been terminated'
		WHERE status = 'running' 
		  AND started_at < $1`

	commandTag, err := r.db.Exec(ctx, query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("cleaning up stale runs: %w", err)
	}

	return int(commandTag.RowsAffected()), nil
}

func (r *PostgresRepository) AddRunEntity(ctx context.Context, runDBID string, entity *RunEntity) error {
	query := `
		INSERT INTO run_entities (id, run_id, entity_type, entity_mrn, entity_name, status, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (run_id, entity_type, entity_mrn) 
		DO UPDATE SET status = $6, error_message = $7, created_at = $8`

	_, err := r.db.Exec(ctx, query,
		entity.ID, runDBID, entity.EntityType, entity.EntityMRN,
		entity.EntityName, entity.Status, entity.ErrorMessage, entity.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting run entity: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListRunEntities(ctx context.Context, runDBID, entityType, status string, limit, offset int) ([]*RunEntity, int, error) {
	countQuery := "SELECT COUNT(*) FROM run_entities WHERE run_id = $1"
	countArgs := []interface{}{runDBID}

	if entityType != "" {
		countQuery += " AND entity_type = $" + fmt.Sprintf("%d", len(countArgs)+1)
		countArgs = append(countArgs, entityType)
	}
	if status != "" {
		countQuery += " AND status = $" + fmt.Sprintf("%d", len(countArgs)+1)
		countArgs = append(countArgs, status)
	}

	var total int
	err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting run entities: %w", err)
	}

	query := `
		SELECT id, run_id, entity_type, entity_mrn, entity_name, status, error_message, created_at
		FROM run_entities 
		WHERE run_id = $1`

	args := []interface{}{runDBID}

	if entityType != "" {
		query += " AND entity_type = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, entityType)
	}
	if status != "" {
		query += " AND status = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", len(args)+1) +
		" OFFSET $" + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying run entities: %w", err)
	}
	defer rows.Close()

	var entities []*RunEntity
	for rows.Next() {
		var entity RunEntity
		var entityName sql.NullString
		var errorMessage sql.NullString

		err := rows.Scan(
			&entity.ID,
			&entity.RunID,
			&entity.EntityType,
			&entity.EntityMRN,
			&entityName,
			&entity.Status,
			&errorMessage,
			&entity.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning run entity: %w", err)
		}

		if entityName.Valid {
			entity.EntityName = entityName.String
		}
		if errorMessage.Valid {
			entity.ErrorMessage = errorMessage.String
		}

		entities = append(entities, &entity)
	}

	return entities, total, nil
}

func (r *PostgresRepository) scanSingleRun(ctx context.Context, query string, args ...interface{}) (*plugin.Run, error) {
	row := r.db.QueryRow(ctx, query, args...)
	return r.scanRun(ctx, row)
}

func (r *PostgresRepository) scanMultipleRuns(ctx context.Context, query string, args ...interface{}) ([]*plugin.Run, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying runs: %w", err)
	}
	defer rows.Close()

	var runs []*plugin.Run
	for rows.Next() {
		run, err := r.scanRun(ctx, rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (r *PostgresRepository) scanRun(ctx context.Context, row pgx.Row) (*plugin.Run, error) {
	var run plugin.Run
	var completedAt sql.NullTime
	var errorMessage sql.NullString
	var configJSON, summaryJSON []byte

	err := row.Scan(
		&run.ID, &run.PipelineName, &run.SourceName, &run.RunID,
		&run.Status, &run.StartedAt, &completedAt, &errorMessage,
		&configJSON, &summaryJSON, &run.CreatedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning run: %w", err)
	}

	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}

	if errorMessage.Valid {
		run.ErrorMessage = errorMessage.String
	}

	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &run.Config); err != nil {
			log.Warn().Err(err).Msg("Failed to unmarshal run config")
			run.Config = make(plugin.RawPluginConfig)
		}
	}

	if len(summaryJSON) > 0 {
		var summary plugin.RunSummary
		if err := json.Unmarshal(summaryJSON, &summary); err != nil {
			log.Warn().Err(err).Msg("Failed to unmarshal run summary")
		} else {
			run.Summary = &summary
		}
	}

	return &run, nil
}

func (r *PostgresRepository) ListWithFilters(ctx context.Context, pipelines, statuses []string, limit, offset int) ([]*plugin.Run, int, []string, error) {
	baseWhere := "WHERE 1=1"
	var args []interface{}

	filters := []string{}

	if len(pipelines) > 0 {
		placeholders := make([]string, len(pipelines))
		for i, pipeline := range pipelines {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, pipeline)
		}
		filters = append(filters, fmt.Sprintf("pipeline_name IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, status := range statuses {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, status)
		}
		filters = append(filters, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
	}

	whereClause := baseWhere
	if len(filters) > 0 {
		whereClause += " AND " + strings.Join(filters, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM runs " + whereClause
	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("counting runs: %w", err)
	}

	query := `SELECT id, pipeline_name, source_name, run_id, status, started_at,
		       completed_at, error_message, config, summary, created_by
		FROM runs ` + whereClause +
		fmt.Sprintf(" ORDER BY started_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)

	args = append(args, limit, offset)

	runs, err := r.scanMultipleRuns(ctx, query, args...)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("scanning runs: %w", err)
	}

	availablePipelines, err := r.GetPipelines(ctx)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("getting pipelines: %w", err)
	}

	return runs, total, availablePipelines, nil
}

func (r *PostgresRepository) GetPipelines(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT pipeline_name 
		FROM runs 
		WHERE pipeline_name IS NOT NULL AND pipeline_name != ''
		ORDER BY pipeline_name`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying pipelines: %w", err)
	}
	defer rows.Close()

	var pipelines []string
	for rows.Next() {
		var pipeline string
		if err := rows.Scan(&pipeline); err != nil {
			return nil, fmt.Errorf("scanning pipeline: %w", err)
		}
		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

func (r *PostgresRepository) DeleteCheckpoints(ctx context.Context, pipelineName, sourceName string) error {
	query := `
		DELETE FROM run_checkpoints 
		WHERE run_id IN (
			SELECT id FROM runs 
			WHERE pipeline_name = $1 AND source_name = $2
		)`

	_, err := r.db.Exec(ctx, query, pipelineName, sourceName)
	if err != nil {
		return fmt.Errorf("deleting checkpoints: %w", err)
	}

	return nil
}

