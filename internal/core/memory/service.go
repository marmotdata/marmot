package memory

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Service does no access checks. Callers establish that the principal may
// read or write the entity first.
type Service interface {
	Remember(ctx context.Context, e Entity, in RememberInput) (*Memory, error)
	Get(ctx context.Context, e Entity, id string) (*Memory, error)
	Update(ctx context.Context, e Entity, id string, in UpdateInput) (*Memory, error)
	Forget(ctx context.Context, e Entity, id string) error
	List(ctx context.Context, e Entity, filter ListFilter) (*ListResult, error)
	Search(ctx context.Context, e Entity, q SearchQuery) (*SearchResult, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func validID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// validEntity reports whether e can name a stored entity. Anything else
// cannot match a row, so callers answer not found.
func validEntity(e Entity) bool {
	switch e.Type {
	case EntityAsset:
		return e.ID != "" && len(e.ID) <= MaxAssetIDLength
	case EntityDataProduct:
		return validID(e.ID)
	}
	return false
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// validateContent collapses whitespace, including line breaks, so a memory
// is always one line.
func validateContent(content string) (string, error) {
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return "", invalid("content is required")
	}
	if utf8.RuneCountInString(content) > MaxContentLength {
		return "", invalid("content exceeds %d characters; keep a memory to one short fact", MaxContentLength)
	}
	return content, nil
}

func validateWriter(sessionID string, author Author) error {
	if len(sessionID) > MaxSessionLength {
		return invalid("session_id exceeds %d characters", MaxSessionLength)
	}
	if author.ID == "" {
		return invalid("author is required")
	}
	return nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	return min(limit, MaxLimit)
}

func (s *service) Remember(ctx context.Context, e Entity, in RememberInput) (*Memory, error) {
	if !validEntity(e) {
		return nil, ErrNotFound
	}
	var err error
	if in.Content, err = validateContent(in.Content); err != nil {
		return nil, err
	}
	if err := validateWriter(in.SessionID, in.Author); err != nil {
		return nil, err
	}
	return s.repo.Remember(ctx, e, in)
}

func (s *service) Get(ctx context.Context, e Entity, id string) (*Memory, error) {
	if !validEntity(e) || !validID(id) {
		return nil, ErrNotFound
	}
	return s.repo.Get(ctx, e, id)
}

func (s *service) Update(ctx context.Context, e Entity, id string, in UpdateInput) (*Memory, error) {
	if !validEntity(e) || !validID(id) {
		return nil, ErrNotFound
	}
	var err error
	if in.Content, err = validateContent(in.Content); err != nil {
		return nil, err
	}
	if err := validateWriter(in.SessionID, in.Author); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, e, id, in)
}

func (s *service) Forget(ctx context.Context, e Entity, id string) error {
	if !validEntity(e) || !validID(id) {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, e, id)
}

func (s *service) List(ctx context.Context, e Entity, f ListFilter) (*ListResult, error) {
	if !validEntity(e) {
		return nil, ErrNotFound
	}
	if !f.Sort.Valid() {
		return nil, invalid("unknown sort %q", f.Sort)
	}
	f.Limit = clampLimit(f.Limit)
	f.Offset = max(f.Offset, 0)
	return s.repo.List(ctx, e, f)
}

func (s *service) Search(ctx context.Context, e Entity, q SearchQuery) (*SearchResult, error) {
	if !validEntity(e) {
		return nil, ErrNotFound
	}
	return s.search(q, func(q SearchQuery) ([]*Memory, error) {
		return s.repo.Search(ctx, e, q)
	})
}

func (s *service) search(q SearchQuery, run func(SearchQuery) ([]*Memory, error)) (*SearchResult, error) {
	q.Query = strings.TrimSpace(q.Query)
	if q.Query == "" {
		return nil, invalid("query is required")
	}
	q.Limit = clampLimit(q.Limit)

	memories, err := run(q)
	if err != nil {
		return nil, err
	}
	return &SearchResult{Memories: memories}, nil
}
