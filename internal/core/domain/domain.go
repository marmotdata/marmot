// Package domain organizes catalog entities into a tree of domains and
// subdomains. It is fork-only: see DOMAINS.md for its seams with upstream code.
package domain

import (
	"errors"
	"time"
)

// UnassignedID is the domain every entity without a membership row belongs to.
const UnassignedID = "00000000-0000-4000-8000-000000000001"

// MaxDepth bounds the tree so a subtree query stays a short prefix match.
const MaxDepth = 8

const maxNameLength = 255

type Kind string

const (
	KindAsset             Kind = "asset"
	KindDataProduct       Kind = "data_product"
	KindGlossaryTerm      Kind = "glossary_term"
	KindIngestionSchedule Kind = "ingestion_schedule"
)

// Kinds lists every kind that can belong to a domain.
var Kinds = []Kind{KindAsset, KindDataProduct, KindGlossaryTerm, KindIngestionSchedule}

func (k Kind) valid() bool {
	for _, known := range Kinds {
		if k == known {
			return true
		}
	}
	return false
}

var (
	ErrNotFound       = errors.New("domain not found")
	ErrEntityNotFound = errors.New("entity not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrNameConflict   = errors.New("a sibling domain already has this name")
	ErrHasChildren    = errors.New("domain has subdomains")
	ErrNotEmpty       = errors.New("domain still has members")
	ErrCycle          = errors.New("a domain cannot move under itself or its subdomains")
	ErrTooDeep        = errors.New("domain tree would exceed the maximum depth")
	ErrProtected      = errors.New("the unassigned domain cannot be renamed, moved or deleted")
)

type Domain struct {
	ID          string         `json:"id"`
	ParentID    *string        `json:"parent_id,omitempty"`
	Path        string         `json:"path"`
	Depth       int            `json:"depth"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	Tags        []string       `json:"tags"`
	Restricted  bool           `json:"restricted"`
	CreatedBy   *string        `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CreateInput struct {
	ParentID    *string        `json:"parent_id"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	Tags        []string       `json:"tags"`
	CreatedBy   string         `json:"-"`
}

// UpdateInput changes only the fields it sets. Restricted is not settable
// until read restriction is enforced on every channel (delivery 3).
type UpdateInput struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	Tags        []string       `json:"tags"`
}
