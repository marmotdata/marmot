package search

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SearchObserver is notified when non-asset entities change.
type SearchObserver interface {
	OnEntityChanged(ctx context.Context, entityType, entityID string)
	OnEntityDeleted(ctx context.Context, entityType, entityID string)
}

// IndexSyncService keeps an external search index in sync with the
// search_index table. It implements SearchObserver for all entity types
// and is wired as an asset observer adapter in server.go.
type IndexSyncService struct {
	indexer SearchIndexer
	repo    IndexRepository

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// IndexRepository provides read access to the search_index table for syncing.
type IndexRepository interface {
	GetSearchDocument(ctx context.Context, entityType, entityID string) (*SearchDocument, error)
	ScanSearchDocuments(ctx context.Context, afterType, afterID string, limit int) ([]SearchDocument, error)
	CountSearchDocuments(ctx context.Context) (int, error)
}

// NewIndexSyncService creates a new sync service.
func NewIndexSyncService(indexer SearchIndexer, repo IndexRepository) *IndexSyncService {
	return &IndexSyncService{
		indexer: indexer,
		repo:    repo,
	}
}

// Start begins processing sync events.
func (s *IndexSyncService) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)
}

// Stop stops processing sync events and waits for in-flight operations.
func (s *IndexSyncService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// OnEntityChanged implements SearchObserver. It reads the entity from
// the search_index table and pushes it to the external index.
func (s *IndexSyncService) OnEntityChanged(ctx context.Context, entityType, entityID string) {
	s.syncEntity(ctx, entityType, entityID)
}

// OnEntityDeleted implements SearchObserver. It removes the entity from
// the external index.
func (s *IndexSyncService) OnEntityDeleted(ctx context.Context, entityType, entityID string) {
	if err := s.indexer.Delete(ctx, entityType, entityID); err != nil {
		log.Warn().Err(err).
			Str("type", entityType).
			Str("id", entityID).
			Msg("Failed to delete entity from search index")
	}
}

// SyncAsset syncs an asset by ID. Called by the adapter in server.go.
func (s *IndexSyncService) SyncAsset(ctx context.Context, assetID string) {
	s.syncEntity(ctx, string(ResultTypeAsset), assetID)
}

// DeleteAsset deletes an asset from the index. Called by the adapter in server.go.
func (s *IndexSyncService) DeleteAsset(ctx context.Context, assetID string) {
	if err := s.indexer.Delete(ctx, string(ResultTypeAsset), assetID); err != nil {
		log.Warn().Err(err).Str("asset_id", assetID).Msg("Failed to delete asset from search index")
	}
}

// syncEntity reads the entity from search_index and pushes it to ES.
// It runs in a background goroutine with a detached context so that
// HTTP request cancellation doesn't abort the index sync.
func (s *IndexSyncService) syncEntity(_ context.Context, entityType, entityID string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		doc, err := s.repo.GetSearchDocument(ctx, entityType, entityID)
		if err != nil {
			log.Warn().Err(err).
				Str("type", entityType).
				Str("id", entityID).
				Msg("Failed to read entity from search_index for sync")
			return
		}
		if doc == nil {
			// Entity not in search_index (might have been deleted), remove from ES
			if err := s.indexer.Delete(ctx, entityType, entityID); err != nil {
				log.Warn().Err(err).
					Str("type", entityType).
					Str("id", entityID).
					Msg("Failed to delete entity from search index")
			}
			return
		}

		if err := s.indexer.Index(ctx, *doc); err != nil {
			log.Warn().Err(err).
				Str("type", entityType).
				Str("id", entityID).
				Msg("Failed to index entity to search index")
		}
	}()
}
