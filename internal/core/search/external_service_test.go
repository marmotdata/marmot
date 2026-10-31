package search

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockIndexer implements SearchIndexer for testing.
type mockIndexer struct {
	searchFunc func(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error)
}

func (m *mockIndexer) Index(ctx context.Context, doc SearchDocument) error       { return nil }
func (m *mockIndexer) BulkIndex(ctx context.Context, docs []SearchDocument) error { return nil }
func (m *mockIndexer) Delete(ctx context.Context, entityType, entityID string) error {
	return nil
}
func (m *mockIndexer) Search(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
	return m.searchFunc(ctx, filter)
}
func (m *mockIndexer) Healthy(ctx context.Context) bool    { return true }
func (m *mockIndexer) CreateIndex(ctx context.Context) error { return nil }
func (m *mockIndexer) Close() error                         { return nil }

// mockPGService implements Service for testing.
type mockPGService struct {
	searchFunc func(ctx context.Context, filter Filter) (*Response, error)
}

func (m *mockPGService) Search(ctx context.Context, filter Filter) (*Response, error) {
	return m.searchFunc(ctx, filter)
}

func TestExternalSearchService_TextQueryGoesToIndexer(t *testing.T) {
	indexerCalled := false
	pgCalled := false

	indexer := &mockIndexer{
		searchFunc: func(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
			indexerCalled = true
			return []*Result{{ID: "es-1", Name: "ES Result"}}, 1, &Facets{
				Types:      map[ResultType]int{},
				AssetTypes: []FacetValue{},
				Providers:  []FacetValue{},
				Tags:       []FacetValue{},
			}, nil
		},
	}

	pgSvc := &mockPGService{
		searchFunc: func(ctx context.Context, filter Filter) (*Response, error) {
			pgCalled = true
			return &Response{}, nil
		},
	}

	svc := NewExternalSearchService(indexer, pgSvc, 10*time.Second)

	resp, err := svc.Search(context.Background(), Filter{Query: "test query", Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !indexerCalled {
		t.Error("expected indexer to be called for text query")
	}
	if pgCalled {
		t.Error("did not expect PG service to be called for text query")
	}
	if len(resp.Results) != 1 || resp.Results[0].ID != "es-1" {
		t.Error("expected ES result")
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
}

func TestExternalSearchService_EmptyQueryGoesToPG(t *testing.T) {
	indexerCalled := false
	pgCalled := false

	indexer := &mockIndexer{
		searchFunc: func(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
			indexerCalled = true
			return nil, 0, nil, nil
		},
	}

	pgSvc := &mockPGService{
		searchFunc: func(ctx context.Context, filter Filter) (*Response, error) {
			pgCalled = true
			return &Response{
				Results: []*Result{{ID: "pg-1", Name: "PG Result"}},
				Total:   1,
				Facets: &Facets{
					Types:      map[ResultType]int{ResultTypeAsset: 1},
					AssetTypes: []FacetValue{},
					Providers:  []FacetValue{},
					Tags:       []FacetValue{},
				},
				Limit: 20,
			}, nil
		},
	}

	svc := NewExternalSearchService(indexer, pgSvc, 10*time.Second)

	resp, err := svc.Search(context.Background(), Filter{Query: "", Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if indexerCalled {
		t.Error("did not expect indexer to be called for empty query")
	}
	if !pgCalled {
		t.Error("expected PG service to be called for empty query")
	}
	if len(resp.Results) != 1 || resp.Results[0].ID != "pg-1" {
		t.Error("expected PG result")
	}
}

func TestExternalSearchService_IndexerErrorPropagates(t *testing.T) {
	indexer := &mockIndexer{
		searchFunc: func(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
			return nil, 0, nil, errors.New("connection refused")
		},
	}

	pgSvc := &mockPGService{
		searchFunc: func(ctx context.Context, filter Filter) (*Response, error) {
			t.Error("PG service should not be called on indexer error")
			return nil, nil
		},
	}

	svc := NewExternalSearchService(indexer, pgSvc, 10*time.Second)

	_, err := svc.Search(context.Background(), Filter{Query: "test", Limit: 20})
	if err == nil {
		t.Fatal("expected error to propagate from indexer")
	}
}

func TestExternalSearchService_DefaultLimits(t *testing.T) {
	var capturedFilter Filter

	indexer := &mockIndexer{
		searchFunc: func(ctx context.Context, filter Filter) ([]*Result, int, *Facets, error) {
			capturedFilter = filter
			return nil, 0, &Facets{
				Types:      map[ResultType]int{},
				AssetTypes: []FacetValue{},
				Providers:  []FacetValue{},
				Tags:       []FacetValue{},
			}, nil
		},
	}

	pgSvc := &mockPGService{}
	svc := NewExternalSearchService(indexer, pgSvc, 10*time.Second)

	// Zero limit should default to 20
	_, err := svc.Search(context.Background(), Filter{Query: "test", Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Limit != 20 {
		t.Errorf("expected default limit=20, got %d", capturedFilter.Limit)
	}

	// Over-limit should cap at 100
	_, err = svc.Search(context.Background(), Filter{Query: "test", Limit: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Limit != 100 {
		t.Errorf("expected capped limit=100, got %d", capturedFilter.Limit)
	}

	// Negative offset should default to 0
	_, err = svc.Search(context.Background(), Filter{Query: "test", Limit: 20, Offset: -5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Offset != 0 {
		t.Errorf("expected default offset=0, got %d", capturedFilter.Offset)
	}
}

func TestExternalSearchService_UnsupportedQueriesGoToPG(t *testing.T) {
	cases := map[string]Filter{
		"popular sort":     {Query: "orders", Sort: SortPopular},
		"owner filter":     {Query: "orders", Owners: []string{"u1"}},
		"field conditions": {Query: `orders AND @tag = "pii"`},
		"kind condition":   {Query: `@kind = "glossary" revenue`},
	}
	for name, filter := range cases {
		t.Run(name, func(t *testing.T) {
			indexer := &mockIndexer{searchFunc: func(ctx context.Context, f Filter) ([]*Result, int, *Facets, error) {
				t.Fatal("the external index should not be asked")
				return nil, 0, nil, nil
			}}
			pgCalled := false
			pg := &mockPGService{searchFunc: func(ctx context.Context, f Filter) (*Response, error) {
				pgCalled = true
				return &Response{}, nil
			}}
			svc := NewExternalSearchService(indexer, pg, time.Second)
			if _, err := svc.Search(context.Background(), filter); err != nil {
				t.Fatal(err)
			}
			if !pgCalled {
				t.Fatal("expected Postgres to answer")
			}
		})
	}
}

func TestExternalSearchService_NameSortStaysOnIndexer(t *testing.T) {
	called := false
	indexer := &mockIndexer{searchFunc: func(ctx context.Context, f Filter) ([]*Result, int, *Facets, error) {
		called = true
		return nil, 0, &Facets{}, nil
	}}
	pg := &mockPGService{searchFunc: func(ctx context.Context, f Filter) (*Response, error) {
		t.Fatal("Postgres should not be asked")
		return nil, nil
	}}
	svc := NewExternalSearchService(indexer, pg, time.Second)
	resp, err := svc.Search(context.Background(), Filter{Query: "orders", Sort: SortName})
	if err != nil {
		t.Fatal(err)
	}
	if !called || resp.Facets.Owners == nil {
		t.Fatal("expected the indexer to answer with an empty owner facet")
	}
}
