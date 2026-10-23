// Package accessexpiry tells people when something that lets them or their automation in, such as an API key, is about to expire or has expired. Each occurrence is announced once: the notice is claimed by a key that includes the expiry time, so dismissing it never brings it back and extending the expiry arms a fresh one.
package accessexpiry

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/background"
	"github.com/marmotdata/marmot/internal/core/notification"
)

const (
	// Window is how far ahead an expiry is announced, matching what the Settings overview lists as expiring soon.
	Window = 7 * 24 * time.Hour
	// Lookback bounds how long ago an expiry may have happened and still be announced, so the first run after an upgrade or a long outage does not replay history.
	Lookback = 7 * 24 * time.Hour
	// Interval is how often the sources are checked.
	Interval = time.Hour

	StateExpiring = "expiring"
	StateExpired  = "expired"
)

// Notice is one notification to send once, under Key.
type Notice struct {
	Key   string
	Input notification.CreateNotificationInput
}

// Source finds what is expiring or has expired as of now.
type Source interface {
	Notices(ctx context.Context, now time.Time) ([]Notice, error)
}

// Sender sends a notification once per key; notification.Service implements it.
type Sender interface {
	CreateOnce(ctx context.Context, key string, input notification.CreateNotificationInput) (bool, error)
}

// Runner checks every source on a schedule and sends what is due.
type Runner struct {
	sender  Sender
	sources []Source
	task    *background.SingletonTask
}

// NewRunner returns a runner over the given sources. Further sources can be added with Add before Start.
func NewRunner(db *pgxpool.Pool, sender Sender, sources ...Source) *Runner {
	r := &Runner{sender: sender, sources: sources}
	r.task = background.NewSingletonTask(background.SingletonConfig{
		Name:         "access-expiry-notices",
		DB:           db,
		Interval:     Interval,
		InitialDelay: 2 * time.Minute,
		TaskFn: func(ctx context.Context) error {
			return r.Run(ctx, time.Now())
		},
	})
	return r
}

// Add registers another source, such as one an edition adds.
func (r *Runner) Add(source Source) {
	r.sources = append(r.sources, source)
}

// Start begins the schedule.
func (r *Runner) Start(ctx context.Context) {
	r.task.Start(ctx)
}

// Stop ends the schedule.
func (r *Runner) Stop() {
	r.task.Stop()
}

// Run checks every source once. A failing source or notice is logged and skipped so the rest still go out, and the first error is returned.
func (r *Runner) Run(ctx context.Context, now time.Time) error {
	var firstErr error
	sent := 0
	for _, source := range r.sources {
		notices, err := source.Notices(ctx, now)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to check for expiring access")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, n := range notices {
			if len(n.Input.Recipients) == 0 {
				continue
			}
			ok, err := r.sender.CreateOnce(ctx, n.Key, n.Input)
			if err != nil {
				log.Warn().Err(err).Str("key", n.Key).Msg("Failed to send access expiry notice")
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if ok {
				sent++
			}
		}
	}
	if sent > 0 {
		log.Info().Int("notices", sent).Msg("Sent access expiry notices")
	}
	return firstErr
}

// State reports whether expiresAt has passed as of now.
func State(expiresAt, now time.Time) string {
	if expiresAt.After(now) {
		return StateExpiring
	}
	return StateExpired
}

// FormatDate renders an expiry for the English copy kept with each notification; the UI rebuilds the copy from data in the reader's language and time zone.
func FormatDate(t time.Time) string {
	return t.UTC().Format("2 January 2006 15:04 UTC")
}
