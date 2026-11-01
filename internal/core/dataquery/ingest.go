package dataquery

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ingestThrottle fires asynchronous catalog refreshes, but never faster
// than its per-source and global minimum intervals, so heavy query traffic
// cannot turn into heavy ingestion. Intervals are tracked in memory per
// replica, which makes the global bound soft across replicas; the ingestion
// job machinery still dedupes the actual runs. A nil trigger makes
// maybeTrigger a no-op.
type ingestThrottle struct {
	trigger        IngestTrigger
	sourceInterval time.Duration
	globalInterval time.Duration

	mu            sync.Mutex
	lastGlobal    time.Time
	lastPerSource map[string]time.Time
}

func newIngestThrottle(trigger IngestTrigger, sourceInterval, globalInterval time.Duration) *ingestThrottle {
	return &ingestThrottle{
		trigger:        trigger,
		sourceInterval: sourceInterval,
		globalInterval: globalInterval,
		lastPerSource:  make(map[string]time.Time),
	}
}

// maybeTrigger fires an async refresh for the source if both rate limits
// allow it. It returns immediately and never blocks the query.
func (t *ingestThrottle) maybeTrigger(sourceID string) {
	if t.trigger == nil || sourceID == "" {
		return
	}

	t.mu.Lock()
	now := time.Now()
	if !t.lastGlobal.IsZero() && now.Sub(t.lastGlobal) < t.globalInterval {
		t.mu.Unlock()
		return
	}
	if last, ok := t.lastPerSource[sourceID]; ok && now.Sub(last) < t.sourceInterval {
		t.mu.Unlock()
		return
	}
	// Reserve the slot before releasing the lock so concurrent queries
	// cannot both pass the check and fire twice.
	t.lastGlobal = now
	t.lastPerSource[sourceID] = now
	t.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := t.trigger.TriggerIngest(ctx, sourceID); err != nil {
			log.Warn().Err(err).Str("source", sourceID).Msg("Ingest-on-query trigger failed")
			return
		}
		log.Debug().Str("source", sourceID).Msg("Ingest-on-query enqueued a catalog refresh")
	}()
}
