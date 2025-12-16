package worker

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// BatchProcessor collects items and processes them in batches.
// It flushes when either the batch size is reached or the flush interval expires.
type BatchProcessor[T any] struct {
	name          string
	batchSize     int
	flushInterval time.Duration
	processFn     func(ctx context.Context, items []T) error

	mu      sync.Mutex
	buffer  []T
	timer   *time.Timer
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	flushed chan struct{}
}

// BatchConfig configures a batch processor.
type BatchConfig[T any] struct {
	// Name is used for logging.
	Name string
	// BatchSize is the maximum items per batch. Default: 100.
	BatchSize int
	// FlushInterval is the maximum time to wait before flushing. Default: 1s.
	FlushInterval time.Duration
	// ProcessFn is called to process a batch of items.
	ProcessFn func(ctx context.Context, items []T) error
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor[T any](config BatchConfig[T]) *BatchProcessor[T] {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = time.Second
	}
	if config.Name == "" {
		config.Name = "batch-processor"
	}

	return &BatchProcessor[T]{
		name:          config.Name,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		processFn:     config.ProcessFn,
		buffer:        make([]T, 0, config.BatchSize),
		flushed:       make(chan struct{}, 1),
	}
}

// Start begins the batch processor.
func (b *BatchProcessor[T]) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)

	log.Info().
		Str("processor", b.name).
		Int("batch_size", b.batchSize).
		Dur("flush_interval", b.flushInterval).
		Msg("Batch processor started")
}

// Stop gracefully shuts down the batch processor, flushing any remaining items.
func (b *BatchProcessor[T]) Stop() {
	log.Info().Str("processor", b.name).Msg("Stopping batch processor...")

	if b.cancel != nil {
		b.cancel()
	}

	// Flush remaining items
	b.mu.Lock()
	if len(b.buffer) > 0 {
		items := b.buffer
		b.buffer = make([]T, 0, b.batchSize)
		b.mu.Unlock()

		if err := b.processFn(context.Background(), items); err != nil {
			log.Error().
				Str("processor", b.name).
				Err(err).
				Int("count", len(items)).
				Msg("Failed to flush remaining items on shutdown")
		}
	} else {
		b.mu.Unlock()
	}

	if b.timer != nil {
		b.timer.Stop()
	}

	b.wg.Wait()
	log.Info().Str("processor", b.name).Msg("Batch processor stopped")
}

// Add adds an item to the batch. If the batch is full, it triggers a flush.
func (b *BatchProcessor[T]) Add(item T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, item)

	// Start timer on first item
	if len(b.buffer) == 1 {
		b.resetTimer()
	}

	// Flush if batch is full
	if len(b.buffer) >= b.batchSize {
		b.flushLocked()
	}
}

// AddBatch adds multiple items at once.
func (b *BatchProcessor[T]) AddBatch(items []T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, item := range items {
		b.buffer = append(b.buffer, item)

		if len(b.buffer) == 1 {
			b.resetTimer()
		}

		if len(b.buffer) >= b.batchSize {
			b.flushLocked()
		}
	}
}

// Flush forces an immediate flush of the current batch.
func (b *BatchProcessor[T]) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// BufferLength returns the current number of items in the buffer.
func (b *BatchProcessor[T]) BufferLength() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buffer)
}

func (b *BatchProcessor[T]) resetTimer() {
	if b.timer != nil {
		b.timer.Stop()
	}

	b.timer = time.AfterFunc(b.flushInterval, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.flushLocked()
	})
}

func (b *BatchProcessor[T]) flushLocked() {
	if len(b.buffer) == 0 {
		return
	}

	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}

	items := b.buffer
	b.buffer = make([]T, 0, b.batchSize)

	// Process in background
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		ctx := b.ctx
		if ctx == nil {
			ctx = context.Background()
		}

		if err := b.processFn(ctx, items); err != nil {
			log.Error().
				Str("processor", b.name).
				Err(err).
				Int("count", len(items)).
				Msg("Failed to process batch")
		} else {
			log.Debug().
				Str("processor", b.name).
				Int("count", len(items)).
				Msg("Batch processed")
		}

		// Signal flush complete
		select {
		case b.flushed <- struct{}{}:
		default:
		}
	}()
}
