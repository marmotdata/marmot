package glossary

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// synonymsKey holds a term's alternative names. The table has no column
// for them, so they ride along in metadata where search can still reach
// them.
const synonymsKey = "synonyms"

// TermInput is one glossary term as a discovery run found it. Terms are
// keyed by Name, which the source system makes unique and stable, so
// Parent names another term instead of pointing at an ID.
type TermInput struct {
	Name        string                 `json:"name"`
	Definition  string                 `json:"definition"`
	Description string                 `json:"description,omitempty"`
	Parent      string                 `json:"parent,omitempty"`
	Synonyms    []string               `json:"synonyms,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SyncResult reports what a sync did. IDsByName lets the caller turn the
// term names an asset carries into the IDs the link table needs.
type SyncResult struct {
	IDsByName map[string]string `json:"-"`
	Created   int               `json:"created"`
	Updated   int               `json:"updated"`
	Unchanged int               `json:"unchanged"`
}

// SyncTerms upserts the terms a run discovered, keyed by name, and
// resolves their hierarchy in a second pass so the order they arrive in
// does not matter. It is idempotent: running the same input twice leaves
// the same rows behind.
//
// The upsert is unconditional. A run owns definition, description and
// metadata and rewrites them every time; what a person wrote lives in
// user_definition, which nothing here writes and the read path prefers.
//
// Terms whose parent cannot be found stay roots, and a term the sync
// cannot write is reported but does not fail the run: a glossary is
// supporting detail, not a reason to lose an ingestion.
func (s *service) SyncTerms(ctx context.Context, source string, inputs []TermInput) (*SyncResult, error) {
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("%w: source is required", ErrInvalidInput)
	}

	result := &SyncResult{IDsByName: make(map[string]string, len(inputs))}

	// First pass: make sure every term in the batch exists. Nothing here
	// touches parent_term_id, so a child may name a parent that appears
	// later in the batch.
	managed := make(map[string]bool, len(inputs))
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			continue
		}
		if _, seen := result.IDsByName[name]; seen {
			continue
		}

		existing, err := s.repo.GetByName(ctx, name)
		switch {
		case errors.Is(err, ErrNotFound):
			term := &GlossaryTerm{
				Name:        name,
				Definition:  definitionOf(in),
				Description: descriptionOf(in),
				Metadata:    metadataOf(in),
				Tags:        in.Tags,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}
			if err := s.repo.Create(ctx, term, nil); err != nil {
				log.Error().Err(err).Str("source", source).Str("term", name).
					Msg("Failed to create glossary term")
				continue
			}
			result.IDsByName[name] = term.ID
			managed[name] = true
			result.Created++

			if s.searchObserver != nil {
				s.searchObserver.OnEntityChanged(ctx, "glossary", term.ID)
			}

		case err != nil:
			return nil, fmt.Errorf("looking up glossary term %q: %w", name, err)

		default:
			result.IDsByName[name] = existing.ID
			managed[name] = true

			updated := *existing
			updated.Definition = definitionOf(in)
			updated.Description = descriptionOf(in)
			updated.Metadata = metadataOf(in)
			updated.Tags = in.Tags

			if sameTerm(existing, &updated) {
				result.Unchanged++
				continue
			}

			updated.UpdatedAt = time.Now().UTC()
			if err := s.repo.Update(ctx, &updated, nil); err != nil {
				log.Error().Err(err).Str("source", source).Str("term", name).
					Msg("Failed to update glossary term")
				continue
			}
			result.Updated++

			if s.searchObserver != nil {
				s.searchObserver.OnEntityChanged(ctx, "glossary", existing.ID)
			}
		}
	}

	// Second pass: hierarchy, now that every name in the batch resolves
	// to an ID.
	for name, parentName := range parentAssignments(inputs) {
		if !managed[name] {
			continue
		}
		termID, ok := result.IDsByName[name]
		if !ok {
			continue
		}

		parentID, ok := result.IDsByName[parentName]
		if !ok {
			// The parent may already be in the catalog from an earlier
			// run, or may not exist at all.
			parent, err := s.repo.GetByName(ctx, parentName)
			if err != nil {
				log.Warn().Str("term", name).Str("parent", parentName).
					Msg("Parent glossary term not found, keeping term as a root")
				continue
			}
			parentID = parent.ID
			result.IDsByName[parentName] = parent.ID
		}

		if parentID == termID {
			continue
		}

		if err := s.repo.SetParent(ctx, termID, &parentID); err != nil {
			log.Error().Err(err).Str("term", name).Str("parent", parentName).
				Msg("Failed to set glossary term parent")
		}
	}

	return result, nil
}

// parentAssignments returns the parent name each term should end up
// under. Assignments that would make a term its own ancestor are dropped
// rather than written, so a malformed batch cannot produce a hierarchy
// the UI can never finish walking.
func parentAssignments(inputs []TermInput) map[string]string {
	parents := make(map[string]string, len(inputs))
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		parent := strings.TrimSpace(in.Parent)
		if name == "" || parent == "" || name == parent {
			continue
		}
		parents[name] = parent
	}

	// Every term whose chain of parents loops is dropped, not just the
	// one edge that closes the loop, so which term loses its place does
	// not come down to map order.
	looping := make(map[string]bool)
	for name := range parents {
		seen := map[string]bool{name: true}
		for at := parents[name]; at != ""; at = parents[at] {
			if seen[at] {
				looping[name] = true
				break
			}
			seen[at] = true
		}
	}
	for name := range looping {
		delete(parents, name)
	}

	return parents
}

// definitionOf falls back to the term's own name. The column is NOT NULL
// and a source system will happily hand over a term with no wording yet.
func definitionOf(in TermInput) string {
	if definition := strings.TrimSpace(in.Definition); definition != "" {
		return definition
	}
	return strings.TrimSpace(in.Name)
}

func descriptionOf(in TermInput) *string {
	description := strings.TrimSpace(in.Description)
	if description == "" {
		return nil
	}
	return &description
}

// metadataOf carries the source's own metadata through, with synonyms
// folded in because the table has no column for them.
func metadataOf(in TermInput) map[string]interface{} {
	metadata := make(map[string]interface{}, len(in.Metadata)+1)
	maps.Copy(metadata, in.Metadata)

	if len(in.Synonyms) > 0 {
		synonyms := make([]interface{}, 0, len(in.Synonyms))
		for _, synonym := range in.Synonyms {
			synonyms = append(synonyms, synonym)
		}
		metadata[synonymsKey] = synonyms
	}

	return metadata
}

// sameTerm reports whether a sync would leave the row exactly as it is,
// so an unchanged term costs no write and no search reindex.
func sameTerm(existing, updated *GlossaryTerm) bool {
	if existing.Definition != updated.Definition {
		return false
	}
	if !samePointer(existing.Description, updated.Description) {
		return false
	}
	if !slices.Equal(existing.Tags, updated.Tags) {
		return false
	}
	return reflect.DeepEqual(existing.Metadata, updated.Metadata)
}

func samePointer(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
