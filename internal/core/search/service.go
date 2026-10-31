package search

import (
	"context"
	"fmt"
	"time"

	validator "github.com/go-playground/validator/v10"
)

// ResultType represents the type of search result
type ResultType string // @name ResultType

const (
	ResultTypeAsset       ResultType = "asset"
	ResultTypeGlossary    ResultType = "glossary"
	ResultTypeTeam        ResultType = "team"
	ResultTypeDataProduct ResultType = "data_product"
)

// Result represents a unified search result
type Result struct {
	Type        ResultType             `json:"type"`
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	URL         string                 `json:"url"`
	Rank        float32                `json:"rank"`
	UpdatedAt   *time.Time             `json:"updated_at,omitempty"`

	// Owners are the people and teams that own an asset, glossary term or data product.
	Owners []ResultOwner `json:"owners,omitempty"`
	// Popularity is an asset's level from 0 to 4, from its views over the last 30 days.
	Popularity *int `json:"popularity,omitempty"`
	// RowCount is an asset's row count where its source reports one.
	RowCount *int64 `json:"row_count,omitempty"`
	// ColumnCount is how many columns or top-level fields an asset's schema has, where it can be read.
	ColumnCount *int `json:"column_count,omitempty"`
	// Stale is set on assets: true when the pipeline that refreshes it has missed more than two runs' worth of time.
	Stale *bool `json:"stale,omitempty"`
	// LinkedAssetCount is how many assets a glossary term is linked to.
	LinkedAssetCount *int `json:"linked_asset_count,omitempty"`
	// AssetCount is how many assets a data product holds or a team owns.
	AssetCount *int `json:"asset_count,omitempty"`
	// MemberCount is how many people are in a team.
	MemberCount *int `json:"member_count,omitempty"`
} // @name Result

// ResultOwner is a person or team that owns a search result.
type ResultOwner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // "user" or "team"
} // @name SearchResultOwner

// Sort orders search results.
type Sort string

const (
	// SortRelevance ranks by match quality, or by most recently updated when there is no text to match.
	SortRelevance Sort = "relevance"
	// SortPopular ranks assets by views over the last 30 days; other kinds have none and follow.
	SortPopular Sort = "popular"
	// SortUpdated ranks by most recently updated.
	SortUpdated Sort = "updated"
	// SortName orders alphabetically by name.
	SortName Sort = "name"
)

// ParseSort reads a sort parameter, reporting false for values it does not know.
func ParseSort(v string) (Sort, bool) {
	switch Sort(v) {
	case "", SortRelevance:
		return SortRelevance, true
	case SortPopular, SortUpdated, SortName:
		return Sort(v), true
	}
	return "", false
}

// Filter represents search filter options
type Filter struct {
	Query      string       `json:"query" validate:"omitempty,max=256"` // Optional query for full-text search
	Types      []ResultType `json:"types,omitempty"`
	AssetTypes []string     `json:"asset_types,omitempty"` // Filter assets by type (TABLE, VIEW, etc.)
	Providers  []string     `json:"providers,omitempty"`   // Filter assets by provider
	Tags       []string     `json:"tags,omitempty"`        // Filter assets by tags
	Owners     []string     `json:"owners,omitempty"`      // Filter by owning user or team ID
	Sort       Sort         `json:"sort,omitempty"`
	Limit      int          `json:"limit" validate:"omitempty,gte=1,lte=100"`
	Offset     int          `json:"offset" validate:"omitempty,gte=0"`
}

type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
} // @name FacetValue

// OwnerFacetValue is an owner in the owner facet, with the name and kind to show it by.
type OwnerFacetValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // "user" or "team"
	Count int    `json:"count"`
} // @name OwnerFacetValue

type Facets struct {
	Types      map[ResultType]int `json:"types"`
	AssetTypes []FacetValue       `json:"asset_types"`
	Providers  []FacetValue       `json:"providers"`
	Tags       []FacetValue       `json:"tags"`
	Owners     []OwnerFacetValue  `json:"owners"`
} // @name Facets

type Response struct {
	Results []*Result `json:"results"`
	Total   int       `json:"total"`
	Facets  *Facets   `json:"facets"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
} // @name SearchResponse

type Service interface {
	Search(ctx context.Context, filter Filter) (*Response, error)
}

// resultEnricher adds the per-kind details (owners, counts and asset signals) to a page of results. Both search paths share it so results read the same whichever backend answered.
type resultEnricher interface {
	EnrichResults(ctx context.Context, results []*Result)
}

type service struct {
	repo      Repository
	validator *validator.Validate
}

func NewService(repo Repository) Service {
	return &service{
		repo:      repo,
		validator: validator.New(),
	}
}

func (s *service) Search(ctx context.Context, filter Filter) (*Response, error) {
	// Set defaults
	if filter.Limit <= 0 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// Validate filter
	if err := s.validator.Struct(filter); err != nil {
		return nil, fmt.Errorf("invalid search filter: %w", err)
	}

	// Execute search
	results, total, facets, err := s.repo.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	s.EnrichResults(ctx, results)

	return &Response{
		Results: results,
		Total:   total,
		Facets:  facets,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}

// EnrichResults fills in owners, counts and asset signals when the repository can.
func (s *service) EnrichResults(ctx context.Context, results []*Result) {
	if e, ok := s.repo.(resultEnricher); ok {
		e.EnrichResults(ctx, results)
	}
}
