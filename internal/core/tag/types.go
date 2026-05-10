package tag

import (
	"context"
	"errors"
	"time"
)

// Tag represents a structured tag in the central Tags vocabulary
// Used for both asset-level and column-level tagging
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateTagInput represents a request to create a new tag
type CreateTagInput struct {
	Name        string `json:"name"`
	Description *string `json:"description,omitempty"`
}

// UpdateTagInput represents a request to update a tag
type UpdateTagInput struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

var (
	ErrNotFound     = errors.New("tag not found")
	ErrConflict     = errors.New("tag already exists")
	ErrInvalidInput = errors.New("invalid input")
)

type Service interface {
	// GetTag retrieves a tag by ID
	GetTag(ctx context.Context, id string) (*Tag, error)

	// ListTags retrieves all tags
	ListTags(ctx context.Context) ([]Tag, error)

	// CreateTag creates a new tag
	CreateTag(ctx context.Context, input CreateTagInput) (*Tag, error)

	// UpdateTag updates an existing tag
	UpdateTag(ctx context.Context, id string, input UpdateTagInput) (*Tag, error)

	// DeleteTag deletes a tag (cascade delete assignments via FK)
	DeleteTag(ctx context.Context, id string) error

	// ResolveNames resolves tag names to IDs, auto-creating missing ones.
	ResolveNames(ctx context.Context, names []string) ([]string, error)
}

type Repository interface {
	GetTag(ctx context.Context, id string) (*Tag, error)
	ListTags(ctx context.Context) ([]Tag, error)
	CreateTag(ctx context.Context, input CreateTagInput) (*Tag, error)
	UpdateTag(ctx context.Context, id string, input UpdateTagInput) (*Tag, error)
	DeleteTag(ctx context.Context, id string) error
	ResolveNames(ctx context.Context, names []string) ([]string, error)
}
