package search

import (
	"context"
	"testing"
	"time"
)

type scanningRepo struct {
	docs  []SearchDocument
	count int
}

func (r *scanningRepo) GetSearchDocument(ctx context.Context, entityType, entityID string) (*SearchDocument, error) {
	return nil, nil
}

func (r *scanningRepo) ScanSearchDocuments(ctx context.Context, afterType, afterID string, limit int) ([]SearchDocument, error) {
	// Find start position
	start := 0
	if afterType != "" || afterID != "" {
		for i, doc := range r.docs {
			if doc.Type == afterType && doc.EntityID == afterID {
				start = i + 1
				break
			}
		}
	}

	end := start + limit
	if end > len(r.docs) {
		end = len(r.docs)
	}

	if start >= len(r.docs) {
		return nil, nil
	}

	return r.docs[start:end], nil
}

func (r *scanningRepo) CountSearchDocuments(ctx context.Context) (int, error) {
	return r.count, nil
}

func TestReindexer_RunOnce(t *testing.T) {
	now := time.Now()
	indexer := &trackingIndexer{}
	repo := &scanningRepo{
		count: 3,
		docs: []SearchDocument{
			{Type: "asset", EntityID: "a1", Name: "Asset 1", URLPath: "/a1", UpdatedAt: now},
			{Type: "asset", EntityID: "a2", Name: "Asset 2", URLPath: "/a2", UpdatedAt: now},
			{Type: "glossary", EntityID: "g1", Name: "Term 1", URLPath: "/g1", UpdatedAt: now},
		},
	}

	reindexer := NewReindexer(indexer, repo, 2) // batch size 2

	err := reindexer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.indexed) != 3 {
		t.Errorf("expected 3 indexed docs, got %d", len(indexer.indexed))
	}
}

func TestReindexer_RejectsParallel(t *testing.T) {
	now := time.Now()
	// Create enough docs to keep the first reindex busy
	docs := make([]SearchDocument, 100)
	for i := range docs {
		docs[i] = SearchDocument{
			Type: "asset", EntityID: string(rune('a' + i%26)) + string(rune('0'+i/26)),
			Name: "Asset", URLPath: "/a", UpdatedAt: now,
		}
	}

	indexer := &trackingIndexer{}
	repo := &scanningRepo{count: len(docs), docs: docs}

	reindexer := NewReindexer(indexer, repo, 10)

	// Start first reindex
	done := make(chan error, 1)
	go func() {
		done <- reindexer.RunOnce(context.Background())
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// If first is still running, second should fail
	if reindexer.Running() {
		err := reindexer.RunOnce(context.Background())
		if err != ErrReindexInProgress {
			t.Errorf("expected ErrReindexInProgress, got %v", err)
		}
	}

	// Wait for first to complete
	<-done
}

func TestReindexer_CancelledContext(t *testing.T) {
	now := time.Now()
	indexer := &trackingIndexer{}
	repo := &scanningRepo{
		count: 1,
		docs: []SearchDocument{
			{Type: "asset", EntityID: "a1", Name: "Asset 1", URLPath: "/a1", UpdatedAt: now},
		},
	}

	reindexer := NewReindexer(indexer, repo, 500)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := reindexer.RunOnce(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestReindexer_EmptyIndex(t *testing.T) {
	indexer := &trackingIndexer{}
	repo := &scanningRepo{count: 0, docs: nil}

	reindexer := NewReindexer(indexer, repo, 500)

	err := reindexer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.indexed) != 0 {
		t.Errorf("expected 0 indexed docs, got %d", len(indexer.indexed))
	}
}
