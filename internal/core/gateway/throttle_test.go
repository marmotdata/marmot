package gateway

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingTrigger struct {
	mu      sync.Mutex
	count   int64
	sources []string
	done    chan struct{}
	target  int64
}

func (c *countingTrigger) TriggerIngest(ctx context.Context, sourceID string) error {
	c.mu.Lock()
	c.sources = append(c.sources, sourceID)
	c.mu.Unlock()
	if atomic.AddInt64(&c.count, 1) == c.target && c.done != nil {
		close(c.done)
	}
	return nil
}

// waitFor waits briefly for the async triggers to run.
func waitFor(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for triggers")
	}
}

func TestIngestThrottlePerSource(t *testing.T) {
	trig := &countingTrigger{target: 1, done: make(chan struct{})}
	// Long per-source interval, no global gap: a second call for the same
	// source inside the interval must not fire.
	th := newIngestThrottle(trig, time.Hour, 0)

	th.maybeTrigger("src-1")
	waitFor(t, trig.done)
	th.maybeTrigger("src-1") // throttled by per-source interval
	th.maybeTrigger("src-1")

	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt64(&trig.count); got != 1 {
		t.Fatalf("expected 1 trigger for a source within its interval, got %d", got)
	}
}

func TestIngestThrottleGlobalFloor(t *testing.T) {
	trig := &countingTrigger{target: 1, done: make(chan struct{})}
	// Zero per-source interval but a long global floor: even different sources
	// cannot both fire inside the global window.
	th := newIngestThrottle(trig, 0, time.Hour)

	th.maybeTrigger("src-1")
	waitFor(t, trig.done)
	th.maybeTrigger("src-2") // blocked by the global floor
	th.maybeTrigger("src-3")

	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt64(&trig.count); got != 1 {
		t.Fatalf("global floor should cap to 1 trigger, got %d", got)
	}
}

func TestIngestThrottleAllowsAfterInterval(t *testing.T) {
	trig := &countingTrigger{target: 2, done: make(chan struct{})}
	// Tiny intervals: two calls spaced past the window both fire.
	th := newIngestThrottle(trig, time.Millisecond, time.Millisecond)

	th.maybeTrigger("src-1")
	time.Sleep(20 * time.Millisecond)
	th.maybeTrigger("src-1")
	waitFor(t, trig.done)

	if got := atomic.LoadInt64(&trig.count); got != 2 {
		t.Fatalf("expected 2 triggers spaced past the interval, got %d", got)
	}
}

func TestIngestThrottleNilTriggerIsNoop(t *testing.T) {
	th := newIngestThrottle(nil, 0, 0)
	th.maybeTrigger("src-1") // must not panic
}
