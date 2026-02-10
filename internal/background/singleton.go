package background

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// TaskFunc is the function signature for a singleton task.
type TaskFunc func(ctx context.Context) error

// SingletonConfig configures a SingletonTask.
type SingletonConfig struct {
	// Name identifies the task (used for logging and lock ID generation).
	Name string
	// DB is the PostgreSQL connection pool.
	DB *pgxpool.Pool
	// Interval is the time between executions.
	Interval time.Duration
	// InitialDelay is an optional delay before the first execution.
	InitialDelay time.Duration
	// TaskFn is the function to execute on each tick.
	TaskFn TaskFunc
}

// SingletonTask runs a periodic function protected by a PostgreSQL advisory lock.
// Only one instance across the cluster will execute the task at any given interval.
type SingletonTask struct {
	name         string
	db           *pgxpool.Pool
	interval     time.Duration
	initialDelay time.Duration
	taskFn       TaskFunc
	lockID       int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSingletonTask creates a new singleton task.
func NewSingletonTask(config SingletonConfig) *SingletonTask {
	return &SingletonTask{
		name:         config.Name,
		db:           config.DB,
		interval:     config.Interval,
		initialDelay: config.InitialDelay,
		taskFn:       config.TaskFn,
		lockID:       GenerateLockID(config.Name),
	}
}

// Start begins the periodic execution loop.
func (t *SingletonTask) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.loop()
	}()

	log.Info().
		Str("task", t.name).
		Dur("interval", t.interval).
		Int64("lock_id", t.lockID).
		Msg("Singleton task started")
}

// Stop gracefully shuts down the task.
func (t *SingletonTask) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
}

func (t *SingletonTask) loop() {
	if t.initialDelay > 0 {
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(t.initialDelay):
			t.tryExecute()
		}
	}

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.tryExecute()
		}
	}
}

func (t *SingletonTask) tryExecute() {
	conn, err := t.db.Acquire(t.ctx)
	if err != nil {
		if t.ctx.Err() != nil {
			return
		}
		log.Error().Err(err).Str("task", t.name).Msg("Failed to acquire connection for singleton task")
		return
	}
	defer conn.Release()

	var acquired bool
	err = conn.QueryRow(t.ctx, "SELECT pg_try_advisory_lock($1)", t.lockID).Scan(&acquired)
	if err != nil {
		if t.ctx.Err() != nil {
			return
		}
		log.Error().Err(err).Str("task", t.name).Msg("Failed to try advisory lock")
		return
	}

	if !acquired {
		log.Debug().Str("task", t.name).Msg("Singleton task skipped - lock held by another instance")
		return
	}

	defer func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unlockCancel()
		_, unlockErr := conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", t.lockID)
		if unlockErr != nil {
			log.Warn().Err(unlockErr).Str("task", t.name).Msg("Failed to release advisory lock")
		}
	}()

	if err := t.taskFn(t.ctx); err != nil {
		if t.ctx.Err() != nil {
			return
		}
		log.Error().Err(err).Str("task", t.name).Msg("Singleton task failed")
	}
}

// GenerateLockID creates a deterministic int64 lock ID from a task name using FNV-64a hash.
func GenerateLockID(name string) int64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}
