package search

import (
	"context"
	"sync"
	"testing"
	"time"
)

type trackingIndexer struct {
	mu       sync.Mutex
	indexed  []SearchDocument
	deleted  []string // "type:id"
	searchFn func(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error)
}

func (t *trackingIndexer) Index(ctx context.Context, doc SearchDocument) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.indexed = append(t.indexed, doc)
	return nil
}

func (t *trackingIndexer) BulkIndex(ctx context.Context, docs []SearchDocument) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.indexed = append(t.indexed, docs...)
	return nil
}

func (t *trackingIndexer) Delete(ctx context.Context, entityType, entityID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.deleted = append(t.deleted, entityType+":"+entityID)
	return nil
}

func (t *trackingIndexer) Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
	if t.searchFn != nil {
		return t.searchFn(ctx, filter)
	}
	return nil, 0, nil, nil
}

func (t *trackingIndexer) Healthy(ctx context.Context) bool    { return true }
func (t *trackingIndexer) CreateIndex(ctx context.Context) error { return nil }
func (t *trackingIndexer) Close() error                         { return nil }

type mockIndexRepo struct {
	docs map[string]*SearchDocument // "type:id" -> doc
}

func (r *mockIndexRepo) GetSearchDocument(ctx context.Context, entityType, entityID string) (*SearchDocument, error) {
	doc, ok := r.docs[entityType+":"+entityID]
	if !ok {
		return nil, nil
	}
	return doc, nil
}

func (r *mockIndexRepo) ScanSearchDocuments(ctx context.Context, afterType, afterID string, limit int) ([]SearchDocument, error) {
	return nil, nil
}

func (r *mockIndexRepo) CountSearchDocuments(ctx context.Context) (int, error) {
	return len(r.docs), nil
}

func TestIndexSyncService_OnEntityChanged(t *testing.T) {
	now := time.Now()
	indexer := &trackingIndexer{}
	repo := &mockIndexRepo{
		docs: map[string]*SearchDocument{
			"glossary:term-1": {
				Type:      "glossary",
				EntityID:  "term-1",
				Name:      "Test Term",
				URLPath:   "/glossary/term-1",
				UpdatedAt: now,
			},
		},
	}

	svc := NewIndexSyncService(indexer, repo)
	svc.Start(context.Background())

	svc.OnEntityChanged(context.Background(), "glossary", "term-1")

	// Wait for async goroutine
	svc.Stop()

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.indexed) != 1 {
		t.Fatalf("expected 1 indexed doc, got %d", len(indexer.indexed))
	}
	if indexer.indexed[0].EntityID != "term-1" {
		t.Errorf("expected entity_id=term-1, got %s", indexer.indexed[0].EntityID)
	}
}

func TestIndexSyncService_OnEntityDeleted(t *testing.T) {
	indexer := &trackingIndexer{}
	repo := &mockIndexRepo{docs: map[string]*SearchDocument{}}

	svc := NewIndexSyncService(indexer, repo)
	svc.Start(context.Background())

	svc.OnEntityDeleted(context.Background(), "team", "team-1")

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.deleted) != 1 {
		t.Fatalf("expected 1 deleted entry, got %d", len(indexer.deleted))
	}
	if indexer.deleted[0] != "team:team-1" {
		t.Errorf("expected deleted team:team-1, got %s", indexer.deleted[0])
	}
}

func TestIndexSyncService_OnEntityChanged_NotFound(t *testing.T) {
	indexer := &trackingIndexer{}
	repo := &mockIndexRepo{docs: map[string]*SearchDocument{}}

	svc := NewIndexSyncService(indexer, repo)
	svc.Start(context.Background())

	// Entity not in search_index should trigger a delete
	svc.OnEntityChanged(context.Background(), "glossary", "nonexistent")

	svc.Stop()

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.indexed) != 0 {
		t.Errorf("expected 0 indexed docs, got %d", len(indexer.indexed))
	}
	if len(indexer.deleted) != 1 {
		t.Fatalf("expected 1 deleted entry for missing doc, got %d", len(indexer.deleted))
	}
}

func TestIndexSyncService_SyncAsset(t *testing.T) {
	now := time.Now()
	indexer := &trackingIndexer{}
	repo := &mockIndexRepo{
		docs: map[string]*SearchDocument{
			"asset:asset-1": {
				Type:      "asset",
				EntityID:  "asset-1",
				Name:      "My Table",
				URLPath:   "/discover/my-table",
				UpdatedAt: now,
			},
		},
	}

	svc := NewIndexSyncService(indexer, repo)
	svc.Start(context.Background())

	svc.SyncAsset(context.Background(), "asset-1")

	svc.Stop()

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.indexed) != 1 {
		t.Fatalf("expected 1 indexed doc, got %d", len(indexer.indexed))
	}
	if indexer.indexed[0].Type != "asset" {
		t.Errorf("expected type=asset, got %s", indexer.indexed[0].Type)
	}
}

func TestIndexSyncService_DeleteAsset(t *testing.T) {
	indexer := &trackingIndexer{}
	repo := &mockIndexRepo{docs: map[string]*SearchDocument{}}

	svc := NewIndexSyncService(indexer, repo)
	svc.Start(context.Background())

	svc.DeleteAsset(context.Background(), "asset-1")

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.deleted) != 1 {
		t.Fatalf("expected 1 deleted entry, got %d", len(indexer.deleted))
	}
	if indexer.deleted[0] != "asset:asset-1" {
		t.Errorf("expected deleted asset:asset-1, got %s", indexer.deleted[0])
	}
}
