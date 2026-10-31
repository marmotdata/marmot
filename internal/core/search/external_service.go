package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/query"
)

// ExternalSearchService routes text queries to an external search indexer
// and browse/empty queries to the PostgreSQL-backed service.
type ExternalSearchService struct {
	indexer SearchIndexer
	pgSvc   Service
	timeout time.Duration
}

// NewExternalSearchService creates a new service that routes queries between
// an external indexer and the existing PG search service.
func NewExternalSearchService(indexer SearchIndexer, pgSvc Service, timeout time.Duration) Service {
	return &ExternalSearchService{
		indexer: indexer,
		pgSvc:   pgSvc,
		timeout: timeout,
	}
}

// Search routes the query based on whether there is text to search for.
// Text queries go to the external indexer; browse/empty queries go to PG.
func (s *ExternalSearchService) Search(ctx context.Context, filter Filter) (*Response, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// Empty/browse queries stay on PG, as do queries the external index cannot answer: it holds no owners or views, and it matches text only, so @field conditions would be searched as words.
	if filter.Query == "" || filter.Sort == SortPopular || len(filter.Owners) > 0 || hasFieldConditions(filter.Query) {
		return s.pgSvc.Search(ctx, filter)
	}

	// Text queries go to external indexer
	searchCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	results, total, facets, err := s.indexer.Search(searchCtx, filter)
	if err != nil {
		return nil, fmt.Errorf("external search: %w", err)
	}

	if e, ok := s.pgSvc.(resultEnricher); ok {
		e.EnrichResults(ctx, results)
	}
	if facets != nil && facets.Owners == nil {
		facets.Owners = []OwnerFacetValue{}
	}

	return &Response{
		Results: results,
		Total:   total,
		Facets:  facets,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}

// hasFieldConditions reports whether a query uses the query language's @field conditions rather than plain text.
func hasFieldConditions(q string) bool {
	if !strings.Contains(q, "@") {
		return false
	}
	if len(extractKindFilters(q)) > 0 {
		return true
	}
	parsed, err := query.NewParser().Parse(q)
	return err == nil && parsed.HasStructuredFilters()
}
