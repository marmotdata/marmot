package search

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

var ErrReindexInProgress = errors.New("reindex already in progress")

// ReindexBroadcaster defines the interface for broadcasting reindex progress events.
type ReindexBroadcaster interface {
	BroadcastStarted(total int)
	BroadcastProgress(indexed, errors, total int)
	BroadcastCompleted(indexed, errors, total int)
	BroadcastFailed(err error, indexed, errors, total int)
}

// NoopReindexBroadcaster is a broadcaster that does nothing (for when websockets are disabled).
type NoopReindexBroadcaster struct{}

func (n *NoopReindexBroadcaster) BroadcastStarted(total int)                       {}
func (n *NoopReindexBroadcaster) BroadcastProgress(indexed, errors, total int)      {}
func (n *NoopReindexBroadcaster) BroadcastCompleted(indexed, errors, total int)     {}
func (n *NoopReindexBroadcaster) BroadcastFailed(err error, indexed, errors, total int) {}

// Reindexer reads from the search_index table and bulk-indexes to the
// external search backend.
type Reindexer struct {
	indexer     SearchIndexer
	repo        IndexRepository
	batchSize   int
	running     atomic.Bool
	broadcaster ReindexBroadcaster
}

// NewReindexer creates a new reindexer.
func NewReindexer(indexer SearchIndexer, repo IndexRepository, batchSize int) *Reindexer {
	if batchSize <= 0 {
		batchSize = 500
	}
	return &Reindexer{
		indexer:     indexer,
		repo:        repo,
		batchSize:   batchSize,
		broadcaster: &NoopReindexBroadcaster{},
	}
}

// SetBroadcaster sets the broadcaster for reindex progress events.
func (r *Reindexer) SetBroadcaster(b ReindexBroadcaster) {
	r.broadcaster = b
}

// Running returns true if a reindex is in progress.
func (r *Reindexer) Running() bool {
	return r.running.Load()
}

// RunOnce performs a full reindex from the search_index table to the
// external search backend. It is idempotent and can be called concurrently
// (subsequent calls are rejected if one is already running).
func (r *Reindexer) RunOnce(ctx context.Context) error {
	if !r.running.CompareAndSwap(false, true) {
		return ErrReindexInProgress
	}
	defer r.running.Store(false)

	// Ensure the index exists
	if err := r.indexer.CreateIndex(ctx); err != nil {
		return err
	}

	total, err := r.repo.CountSearchDocuments(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to count search documents for reindex")
		return err
	}

	log.Info().Int("total_documents", total).Msg("Starting full search reindex")
	r.broadcaster.BroadcastStarted(total)

	var (
		afterType     string
		afterID       string
		indexed       int
		errors        int
		lastBroadcast time.Time
	)

	const broadcastInterval = 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			log.Warn().Err(ctx.Err()).Int("indexed", indexed).Msg("Reindex cancelled")
			r.broadcaster.BroadcastFailed(ctx.Err(), indexed, errors, total)
			return ctx.Err()
		}

		docs, err := r.repo.ScanSearchDocuments(ctx, afterType, afterID, r.batchSize)
		if err != nil {
			log.Error().Err(err).
				Str("after_type", afterType).
				Str("after_id", afterID).
				Msg("Failed to scan search documents, continuing")
			errors++
			break
		}

		if len(docs) == 0 {
			break
		}

		if err := r.indexer.BulkIndex(ctx, docs); err != nil {
			log.Warn().Err(err).
				Int("batch_size", len(docs)).
				Msg("Failed to bulk index batch, continuing")
			errors++
		} else {
			indexed += len(docs)
		}

		// Throttle progress broadcasts to avoid flooding the websocket
		if time.Since(lastBroadcast) >= broadcastInterval {
			r.broadcaster.BroadcastProgress(indexed, errors, total)
			lastBroadcast = time.Now()
		}

		last := docs[len(docs)-1]
		afterType = last.Type
		afterID = last.EntityID

		if indexed%5000 == 0 && indexed > 0 {
			log.Info().Int("indexed", indexed).Int("total", total).Msg("Reindex progress")
		}
	}

	log.Info().
		Int("indexed", indexed).
		Int("errors", errors).
		Int("total", total).
		Msg("Full search reindex complete")

	r.broadcaster.BroadcastCompleted(indexed, errors, total)

	return nil
}
