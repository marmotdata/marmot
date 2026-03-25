package search

import (
	"context"
	"time"
)

// SearchDocument mirrors search_index table rows for external indexing.
type SearchDocument struct {
	Type            string
	EntityID        string
	Name            string
	Description     *string
	URLPath         string
	AssetType       *string
	PrimaryProvider *string
	Providers       []string
	Tags            []string
	MRN             *string
	CreatedBy       *string
	CreatedAt       *time.Time
	UpdatedAt       time.Time
	Metadata        map[string]interface{}
	Documentation   *string
}

// SearchIndexer defines the interface for external search backends.
type SearchIndexer interface {
	Index(ctx context.Context, doc SearchDocument) error
	BulkIndex(ctx context.Context, docs []SearchDocument) error
	Delete(ctx context.Context, entityType, entityID string) error
	Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error)
	Healthy(ctx context.Context) bool
	CreateIndex(ctx context.Context) error
	Close() error
}
