package common

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/marmotdata/marmot/internal/config"
	"github.com/rs/zerolog/log"
)

// RateLimitStore manages rate limit counters in memory
type RateLimitStore struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

type bucket struct {
	count      int
	resetTime  time.Time
	windowSize time.Duration
}

// NewRateLimitStore creates a new in-memory rate limit store
func NewRateLimitStore() *RateLimitStore {
	store := &RateLimitStore{
		buckets: make(map[string]*bucket),
	}

	// Start cleanup goroutine to remove expired buckets
	go store.cleanup()

	return store
}

// cleanup removes expired buckets every minute
func (s *RateLimitStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, b := range s.buckets {
			if now.After(b.resetTime) {
				delete(s.buckets, key)
			}
		}
		s.mu.Unlock()
	}
}

// allow checks if a request should be allowed with custom limit/window
func (s *RateLimitStore) allow(key string, limit int, window int) (bool, *bucket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	windowSize := time.Duration(window) * time.Second

	b, exists := s.buckets[key]
	if !exists || now.After(b.resetTime) {
		b = &bucket{
			count:      1,
			resetTime:  now.Add(windowSize),
			windowSize: windowSize,
		}
		s.buckets[key] = b
		return true, b
	}

	// Check if limit exceeded
	if b.count >= limit {
		return false, b
	}

	b.count++
	return true, b
}

var (
	globalRateLimitStore *RateLimitStore
	rateLimitOnce        sync.Once
)

// initRateLimitStore initializes the global rate limit store
func initRateLimitStore() *RateLimitStore {
	rateLimitOnce.Do(func() {
		globalRateLimitStore = NewRateLimitStore()
	})
	return globalRateLimitStore
}

// WithRateLimit middleware enforces rate limiting per user/IP with per-endpoint limits
func WithRateLimit(cfg *config.Config, limit int, window int) func(http.HandlerFunc) http.HandlerFunc {
	if !cfg.RateLimit.Enabled {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return next
		}
	}

	store := initRateLimitStore()

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var rateLimitID string
			user, ok := GetAuthenticatedUser(r.Context())

			if ok && user.Username != "anonymous" {
				rateLimitID = fmt.Sprintf("user:%s", user.ID)
			} else {
				rateLimitID = fmt.Sprintf("ip:%s", r.RemoteAddr)
			}

			key := fmt.Sprintf("%s:endpoint:%s", rateLimitID, r.URL.Path)
			allowed, bucket := store.allow(key, limit, window)

			remaining := limit - bucket.count
			if remaining < 0 {
				remaining = 0
			}

			retryAfter := int(time.Until(bucket.resetTime).Seconds())
			if retryAfter < 0 {
				retryAfter = 0
			}

			w.Header().Set("RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("RateLimit-Reset", fmt.Sprintf("%d", bucket.resetTime.Unix()))

			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))

				log.Warn().
					Str("rate_limit_id", rateLimitID).
					Str("endpoint", r.URL.Path).
					Str("method", r.Method).
					Int("limit", limit).
					Int("window", window).
					Msg("Rate limit exceeded")

				RespondError(w, http.StatusTooManyRequests, fmt.Sprintf(
					"Rate limit exceeded. Limit: %d requests per %d seconds. Try again in %d seconds.",
					limit,
					window,
					retryAfter,
				))
				return
			}

			next(w, r)
		}
	}
}
