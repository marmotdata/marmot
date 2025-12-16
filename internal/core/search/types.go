package search

import "time"

// ResultType represents the type of search result
type ResultType string

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
}

// Filter represents search filter options
type Filter struct {
	Query      string       `json:"query" validate:"omitempty,max=256"` // Optional query for full-text search
	Types      []ResultType `json:"types,omitempty"`
	AssetTypes []string     `json:"asset_types,omitempty"` // Filter assets by type (TABLE, VIEW, etc.)
	Providers  []string     `json:"providers,omitempty"`   // Filter assets by provider
	Tags       []string     `json:"tags,omitempty"`        // Filter assets by tags
	Limit      int          `json:"limit" validate:"omitempty,gte=1,lte=100"`
	Offset     int          `json:"offset" validate:"omitempty,gte=0"`
}

type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type Facets struct {
	Types      map[ResultType]int `json:"types"`
	AssetTypes []FacetValue       `json:"asset_types"`
	Providers  []FacetValue       `json:"providers"`
	Tags       []FacetValue       `json:"tags"`
}

type Response struct {
	Results []*Result `json:"results"`
	Total   int       `json:"total"`
	Facets  *Facets   `json:"facets"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
}
