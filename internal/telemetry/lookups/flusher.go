package lookups

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// DefaultFlushInterval is how often in-memory deltas are persisted.
const DefaultFlushInterval = 30 * time.Second

// Flusher periodically drains the recorder and upserts deltas into the store.
type Flusher struct {
	recorder  Recorder
	store     Store
	installID string
	interval  time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewFlusher wires a recorder to a store. installID is captured once; if the
// install has no ID yet the flusher is a no-op (Marmot boots even without one).
func NewFlusher(recorder Recorder, store Store, installID string, interval time.Duration) *Flusher {
	if interval <= 0 {
		interval = DefaultFlushInterval
	}
	return &Flusher{
		recorder:  recorder,
		store:     store,
		installID: installID,
		interval:  interval,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start runs the flush loop until Stop or ctx cancellation.
func (f *Flusher) Start(ctx context.Context) {
	if f.installID == "" {
		close(f.doneCh)
		return
	}
	go f.run(ctx)
}

// Stop signals the loop to exit and blocks until the final flush completes.
func (f *Flusher) Stop() {
	select {
	case <-f.stopCh:
		// already stopped
	default:
		close(f.stopCh)
	}
	<-f.doneCh
}

func (f *Flusher) run(ctx context.Context) {
	defer close(f.doneCh)
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	flush := func() {
		snap := f.recorder.Snapshot()
		if len(snap) == 0 {
			return
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := f.store.AddDeltas(flushCtx, f.installID, snap); err != nil {
			log.Warn().Err(err).Msg("lookups: failed to persist deltas; dropping this window")
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-f.stopCh:
			flush()
			return
		case <-ticker.C:
			flush()
		}
	}
}
