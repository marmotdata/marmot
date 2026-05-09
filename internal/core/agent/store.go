package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrRunNotFound   = errors.New("agent run not found")
)

// Run is a single agent invocation as recorded by the SDK.
type Run struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agent_id"`
	RunID      string     `json:"run_id"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	DurationMs *int       `json:"duration_ms,omitempty"`
	Status     string     `json:"status"`
	Model      string     `json:"model,omitempty"`
	TokensIn   int        `json:"tokens_in"`
	TokensOut  int        `json:"tokens_out"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is a single tool invocation inside a Run.
type ToolCall struct {
	Ordinal    int       `json:"ordinal"`
	ToolName   string    `json:"tool_name"`
	TargetMRN  string    `json:"target_mrn,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs *int      `json:"duration_ms,omitempty"`
	Status     string    `json:"status"`
}

// Bucket is an hour-aligned aggregate of runs by status.
type Bucket struct {
	Hour    time.Time `json:"hour"`
	Success int       `json:"success"`
	Error   int       `json:"error"`
}

// Stats summarises agent activity over a window.
type Stats struct {
	RunCount      int     `json:"run_count"`
	SuccessRate   float64 `json:"success_rate"`
	MedianLatency int     `json:"median_latency_ms"`
	P95Latency    int     `json:"p95_latency_ms"`
	TokensIn      int     `json:"tokens_in"`
	TokensOut     int     `json:"tokens_out"`
}

type Repository interface {
	InsertRun(ctx context.Context, r *Run) error
	GetByAgentRunID(ctx context.Context, agentID, runID string) (*Run, error)
	ListRuns(ctx context.Context, agentID string, since time.Time, limit int) ([]*Run, error)
	BucketRuns(ctx context.Context, agentID string, since time.Time) ([]Bucket, error)
	Stats(ctx context.Context, agentID string, since time.Time) (*Stats, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) InsertRun(ctx context.Context, run *Run) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now()
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
        INSERT INTO agent_runs (
            id, agent_id, run_id, started_at, ended_at, duration_ms,
            status, model, tokens_in, tokens_out, error, created_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		run.ID, run.AgentID, run.RunID, run.StartedAt, run.EndedAt, run.DurationMs,
		run.Status, run.Model, run.TokensIn, run.TokensOut, run.Error, run.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting agent run: %w", err)
	}

	for _, tc := range run.ToolCalls {
		_, err := tx.Exec(ctx, `
            INSERT INTO agent_tool_calls (
                run_pk, ordinal, tool_name, target_mrn, started_at, duration_ms, status
            ) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			run.ID, tc.Ordinal, tc.ToolName, nullableString(tc.TargetMRN),
			tc.StartedAt, tc.DurationMs, tc.Status,
		)
		if err != nil {
			return fmt.Errorf("inserting tool call %d: %w", tc.Ordinal, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetByAgentRunID(ctx context.Context, agentID, runID string) (*Run, error) {
	row := r.db.QueryRow(ctx, `
        SELECT id, agent_id, run_id, started_at, ended_at, duration_ms,
               status, model, tokens_in, tokens_out, error, created_at
        FROM agent_runs WHERE agent_id = $1 AND run_id = $2`,
		agentID, runID,
	)
	run := &Run{}
	err := scanRun(row, run)
	if err == pgx.ErrNoRows {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *PostgresRepository) ListRuns(ctx context.Context, agentID string, since time.Time, limit int) ([]*Run, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, agent_id, run_id, started_at, ended_at, duration_ms,
               status, model, tokens_in, tokens_out, error, created_at
        FROM agent_runs
        WHERE agent_id = $1 AND started_at >= $2
        ORDER BY started_at DESC
        LIMIT $3`,
		agentID, since, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing agent runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run := &Run{}
		if err := scanRun(rows, run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating runs: %w", err)
	}

	if len(runs) == 0 {
		return runs, nil
	}

	ids := make([]string, len(runs))
	byID := make(map[string]*Run, len(runs))
	for i, run := range runs {
		ids[i] = run.ID
		byID[run.ID] = run
	}

	tcRows, err := r.db.Query(ctx, `
        SELECT run_pk, ordinal, tool_name, target_mrn, started_at, duration_ms, status
        FROM agent_tool_calls
        WHERE run_pk = ANY($1)
        ORDER BY run_pk, ordinal`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("listing tool calls: %w", err)
	}
	defer tcRows.Close()

	for tcRows.Next() {
		var runPK string
		var tc ToolCall
		var target *string
		if err := tcRows.Scan(&runPK, &tc.Ordinal, &tc.ToolName, &target, &tc.StartedAt, &tc.DurationMs, &tc.Status); err != nil {
			return nil, fmt.Errorf("scanning tool call: %w", err)
		}
		if target != nil {
			tc.TargetMRN = *target
		}
		if run := byID[runPK]; run != nil {
			run.ToolCalls = append(run.ToolCalls, tc)
		}
	}
	if err := tcRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tool calls: %w", err)
	}

	return runs, nil
}

func (r *PostgresRepository) BucketRuns(ctx context.Context, agentID string, since time.Time) ([]Bucket, error) {
	rows, err := r.db.Query(ctx, `
        SELECT date_trunc('hour', started_at) AS hour,
               COUNT(*) FILTER (WHERE status = 'success') AS success,
               COUNT(*) FILTER (WHERE status = 'error')   AS errored
        FROM agent_runs
        WHERE agent_id = $1 AND started_at >= $2
        GROUP BY 1
        ORDER BY 1`,
		agentID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("bucketing agent runs: %w", err)
	}
	defer rows.Close()

	var buckets []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Hour, &b.Success, &b.Error); err != nil {
			return nil, fmt.Errorf("scanning bucket: %w", err)
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating buckets: %w", err)
	}
	return buckets, nil
}

func (r *PostgresRepository) Stats(ctx context.Context, agentID string, since time.Time) (*Stats, error) {
	row := r.db.QueryRow(ctx, `
        SELECT
            COUNT(*),
            COALESCE(AVG(CASE WHEN status='success' THEN 1.0 ELSE 0.0 END), 0),
            COALESCE(percentile_cont(0.5)  WITHIN GROUP (ORDER BY duration_ms), 0)::int,
            COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::int,
            COALESCE(SUM(tokens_in), 0),
            COALESCE(SUM(tokens_out), 0)
        FROM agent_runs
        WHERE agent_id = $1 AND started_at >= $2`,
		agentID, since,
	)
	stats := &Stats{}
	if err := row.Scan(&stats.RunCount, &stats.SuccessRate, &stats.MedianLatency, &stats.P95Latency, &stats.TokensIn, &stats.TokensOut); err != nil {
		return nil, fmt.Errorf("scanning stats: %w", err)
	}
	return stats, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(s rowScanner, run *Run) error {
	var model, errMsg *string
	if err := s.Scan(
		&run.ID, &run.AgentID, &run.RunID, &run.StartedAt, &run.EndedAt, &run.DurationMs,
		&run.Status, &model, &run.TokensIn, &run.TokensOut, &errMsg, &run.CreatedAt,
	); err != nil {
		return err
	}
	if model != nil {
		run.Model = *model
	}
	if errMsg != nil {
		run.Error = *errMsg
	}
	return nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
