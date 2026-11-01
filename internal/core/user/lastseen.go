package user

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// LastSeenInterval is how stale a member's last seen time may get. Every authenticated request would otherwise be a write, and the member list only needs to tell minutes from days.
const LastSeenInterval = 5 * time.Minute

// LastSeenTracker records when members were last seen, writing at most once per LastSeenInterval for each of them.
type LastSeenTracker struct {
	repo     Repository
	interval time.Duration

	mu   sync.Mutex
	seen map[string]time.Time
}

// NewLastSeenTracker returns a tracker writing through repo.
func NewLastSeenTracker(repo Repository) *LastSeenTracker {
	return &LastSeenTracker{repo: repo, interval: LastSeenInterval, seen: make(map[string]time.Time)}
}

// Touch notes that the member made a request. The write happens in the background so it never slows the request down, and the query itself skips rows written recently, so replicas that each keep their own memory still write rarely.
func (t *LastSeenTracker) Touch(userID string) {
	if t == nil || userID == "" {
		return
	}
	now := time.Now()
	t.mu.Lock()
	if last, ok := t.seen[userID]; ok && now.Sub(last) < t.interval {
		t.mu.Unlock()
		return
	}
	t.seen[userID] = now
	// Members who have gone quiet would otherwise stay in the map forever.
	if len(t.seen) > 10000 {
		for id, at := range t.seen {
			if now.Sub(at) >= t.interval {
				delete(t.seen, id)
			}
		}
	}
	t.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := t.repo.TouchLastSeen(ctx, userID, t.interval); err != nil {
			log.Debug().Err(err).Str("user_id", userID).Msg("Failed to record last seen")
		}
	}()
}
