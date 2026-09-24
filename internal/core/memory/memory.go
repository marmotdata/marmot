// Package memory stores durable, searchable agent memory attached to a
// catalog entity.
package memory

import (
	"errors"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
)

var (
	ErrNotFound = errors.New("memory not found")
	ErrInvalid  = errors.New("invalid memory")
)

// EntityType is the kind of catalog entity an entry belongs to.
type EntityType string

const (
	EntityAsset       EntityType = "asset"
	EntityDataProduct EntityType = "data_product"
)

func (t EntityType) Valid() bool {
	return t == EntityAsset || t == EntityDataProduct
}

// Entity is the catalog entity an entry belongs to.
type Entity struct {
	Type EntityType
	ID   string
}

const (
	// A memory is one short fact enriching its entity.
	MaxContentLength = 280
	MaxSessionLength = 255
	MaxAssetIDLength = 255

	DefaultLimit = 20
	MaxLimit     = 100
)

// Author is who wrote or edited an entry. It is provenance; reads are not
// filtered on it.
type Author struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AuthorFrom converts a principal into an Author.
func AuthorFrom(p auth.Principal) Author {
	return Author{Type: string(p.Type()), ID: p.ID(), Name: p.DisplayName()}
}

type Memory struct {
	ID         string     `json:"id"`
	EntityType EntityType `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	Content    string     `json:"content"`
	CreatedBy  Author     `json:"created_by"`
	// SessionID is the session the entry was created in.
	SessionID        string    `json:"session_id,omitempty"`
	UpdatedBy        Author    `json:"updated_by"`
	UpdatedSessionID string    `json:"updated_session_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// FoundCount is how often a search has returned the memory.
	FoundCount  int        `json:"found_count"`
	LastFoundAt *time.Time `json:"last_found_at,omitempty"`

	// Score is the search relevance; higher is more relevant. Set on search
	// results only.
	Score *float64 `json:"score,omitempty"`
} // @name Memory

type RememberInput struct {
	Content   string
	SessionID string
	Author    Author
}

type UpdateInput struct {
	Content   string
	SessionID string
	Author    Author
}

// Filter narrows a list or a search.
type Filter struct {
	SessionID string
}

// Sort orders a list of memories.
type Sort string

const (
	// SortChanged lists the most recently changed memory first. It is the
	// default.
	SortChanged Sort = "changed"
	// SortCreated lists the newest memory first.
	SortCreated Sort = "created"
	// SortUsed lists the most used memory first: uses counted with a
	// half-life, so frequent and recent use both count.
	SortUsed Sort = "used"
)

func (s Sort) Valid() bool {
	switch s {
	case "", SortChanged, SortCreated:
		return true
	case SortUsed:
		return true
	}
	return false
}

type ListFilter struct {
	Filter
	Sort   Sort
	Limit  int
	Offset int
}

type ListResult struct {
	Memories []*Memory `json:"memories"`
	Total    int       `json:"total"`
} // @name MemoryList

type SearchQuery struct {
	Filter
	Query string
	Limit int
	// CountUse records the returned memories as found. Set for agent searches.
	CountUse bool
}

type SearchResult struct {
	Memories []*Memory `json:"memories"`
} // @name MemorySearchResult
