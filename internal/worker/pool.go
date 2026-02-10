package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// Job represents a unit of work to be processed by the worker pool.
type Job interface {
	// Execute performs the job's work. Returns an error if the job failed.
	Execute(ctx context.Context) error
	// ID returns a unique identifier for logging purposes.
	ID() string
}

// Pool manages a pool of workers that process jobs from a queue.
type Pool struct {
	name          string
	maxWorkers    int
	queueSize     int
	jobQueue      chan Job
	semaphore     chan struct{}
	activeWorkers atomic.Int32

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Optional callbacks
	onJobStart    func(job Job)
	onJobComplete func(job Job, err error, duration time.Duration)
	onJobPanic    func(job Job, recovered interface{})
}

// PoolConfig configures a worker pool.
type PoolConfig struct {
	// Name is used for logging.
	Name string
	// MaxWorkers is the maximum number of concurrent workers. Default: 10.
	MaxWorkers int
	// QueueSize is the buffer size for the job queue. Default: 100.
	QueueSize int
	// OnJobStart is called when a job starts processing.
	OnJobStart func(job Job)
	// OnJobComplete is called when a job finishes (successfully or with error).
	OnJobComplete func(job Job, err error, duration time.Duration)
	// OnJobPanic is called when a job panics.
	OnJobPanic func(job Job, recovered interface{})
}

// NewPool creates a new worker pool with the given configuration.
func NewPool(config PoolConfig) *Pool {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 10
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.Name == "" {
		config.Name = "worker-pool"
	}

	return &Pool{
		name:          config.Name,
		maxWorkers:    config.MaxWorkers,
		queueSize:     config.QueueSize,
		jobQueue:      make(chan Job, config.QueueSize),
		semaphore:     make(chan struct{}, config.MaxWorkers),
		onJobStart:    config.OnJobStart,
		onJobComplete: config.OnJobComplete,
		onJobPanic:    config.OnJobPanic,
	}
}

// Start begins the worker pool. It spawns a dispatcher goroutine that
// reads from the job queue and assigns work to available workers.
func (p *Pool) Start(ctx context.Context) {
	p.ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.dispatcher()
	}()

	log.Info().
		Str("pool", p.name).
		Int("max_workers", p.maxWorkers).
		Int("queue_size", p.queueSize).
		Msg("Worker pool started")
}

// Stop gracefully shuts down the worker pool.
// It stops accepting new jobs, waits for in-flight jobs to complete,
// and then returns.
func (p *Pool) Stop() {
	log.Info().Str("pool", p.name).Msg("Stopping worker pool...")

	if p.cancel != nil {
		p.cancel()
	}

	close(p.jobQueue)
	p.wg.Wait()

	log.Info().Str("pool", p.name).Msg("Worker pool stopped")
}

// Submit adds a job to the queue. Returns true if the job was queued,
// false if the queue is full (non-blocking).
func (p *Pool) Submit(job Job) bool {
	select {
	case p.jobQueue <- job:
		return true
	default:
		log.Warn().
			Str("pool", p.name).
			Str("job_id", job.ID()).
			Msg("Job queue full, dropping job")
		return false
	}
}

// SubmitWait adds a job to the queue, blocking until space is available
// or the context is cancelled.
func (p *Pool) SubmitWait(ctx context.Context, job Job) error {
	select {
	case p.jobQueue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// ActiveWorkers returns the number of currently active workers.
func (p *Pool) ActiveWorkers() int {
	return int(p.activeWorkers.Load())
}

// QueueLength returns the current number of jobs in the queue.
func (p *Pool) QueueLength() int {
	return len(p.jobQueue)
}

func (p *Pool) dispatcher() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}

			// Acquire semaphore slot (blocks if all workers busy)
			p.semaphore <- struct{}{}
			p.activeWorkers.Add(1)

			// Spawn worker goroutine
			go func(j Job) {
				defer func() {
					if r := recover(); r != nil {
						log.Error().
							Str("pool", p.name).
							Str("job_id", j.ID()).
							Interface("panic", r).
							Msg("Worker panic recovered")

						if p.onJobPanic != nil {
							p.onJobPanic(j, r)
						}
					}
					<-p.semaphore
					p.activeWorkers.Add(-1)
				}()

				p.executeJob(j)
			}(job)
		}
	}
}

func (p *Pool) executeJob(job Job) {
	start := time.Now()

	if p.onJobStart != nil {
		p.onJobStart(job)
	}

	err := job.Execute(p.ctx)
	duration := time.Since(start)

	if err != nil {
		log.Error().
			Str("pool", p.name).
			Str("job_id", job.ID()).
			Err(err).
			Dur("duration", duration).
			Msg("Job failed")
	} else {
		log.Debug().
			Str("pool", p.name).
			Str("job_id", job.ID()).
			Dur("duration", duration).
			Msg("Job completed")
	}

	if p.onJobComplete != nil {
		p.onJobComplete(job, err, duration)
	}
}
