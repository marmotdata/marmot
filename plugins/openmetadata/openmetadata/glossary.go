package openmetadata

import (
	"context"
	"fmt"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// The business glossary: the vocabulary someone wrote by hand in
// OpenMetadata, which is the part of a catalog that cannot be read back
// out of the systems themselves.
//
// A term is identified by its OpenMetadata fully qualified name rather
// than its own name, so two glossaries can each hold a Customer without
// becoming one term in Marmot.
//
// The glossary is imported as a term too, and its top level terms sit
// under it. Marmot has a single tree of terms rather than a set of named
// glossaries, so without that root the vocabularies would all land side
// by side at the top of the tree with nothing saying which is which.

const (
	glossaryFields     = "owners,tags"
	glossaryTermFields = "parent,glossary,synonyms,owners,tags,relatedTerms"
)

func (c *collector) discoverGlossary(ctx context.Context, client *client) error {
	glossaries, supported, err := listOptional[glossary](ctx, client, "/v1/glossaries", glossaryFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing glossaries: %w", err)
	}
	if !supported {
		log.Debug().Msg("OpenMetadata does not expose a glossary, skipping")
		return nil
	}

	roots := 0
	for _, g := range glossaries {
		if !c.wantedTerm(g.entityBase) {
			continue
		}

		c.terms = append(c.terms, pluginsdk.GlossaryTerm{
			Name:       g.FullyQualifiedName,
			Definition: termDefinition(g.entityBase),
			Tags:       tagNames(g.Tags),
			Metadata:   c.termMetadata(g.entityBase, ""),
		})
		roots++
	}

	terms, _, err := listOptional[glossaryTerm](ctx, client, "/v1/glossaryTerms", glossaryTermFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing glossary terms: %w", err)
	}

	discovered := 0
	for _, t := range terms {
		if !c.wantedTerm(t.entityBase) {
			continue
		}

		c.terms = append(c.terms, pluginsdk.GlossaryTerm{
			Name:       t.FullyQualifiedName,
			Definition: termDefinition(t.entityBase),
			Parent:     parentOf(t),
			Synonyms:   t.Synonyms,
			Tags:       tagNames(t.Tags),
			Metadata:   c.termMetadata(t.entityBase, glossaryOf(t)),
		})
		discovered++
	}

	log.Debug().Int("glossaries", roots).Int("terms", discovered).Msg("Discovered glossary terms")
	return nil
}

// wantedTerm reports whether a glossary entity is in scope. A glossary
// belongs to no service, so the service filters that scope an import
// cannot apply to it: honouring them would drop the whole vocabulary
// from any run that names the services it wants.
func (c *collector) wantedTerm(base entityBase) bool {
	if base.FullyQualifiedName == "" {
		return false
	}
	return !base.Deleted || c.config.IncludeDeleted
}

// parentOf is the term a term sits under. One with no parent term sits
// under its glossary, which was imported as a term of its own.
func parentOf(t glossaryTerm) string {
	if t.Parent != nil && t.Parent.FullyQualifiedName != "" {
		return t.Parent.FullyQualifiedName
	}
	return glossaryOf(t)
}

// glossaryOf is the vocabulary a term belongs to. A glossary's fully
// qualified name is its name, so a reference that carries only one of
// the two is still enough to place the term.
func glossaryOf(t glossaryTerm) string {
	if t.Glossary.FullyQualifiedName != "" {
		return t.Glossary.FullyQualifiedName
	}
	return t.Glossary.Name
}

// termDefinition is what the term means. Marmot requires every term to
// have a definition and OpenMetadata does not, so a term nobody wrote
// anything about is defined by its own name rather than dropped.
func termDefinition(base entityBase) string {
	if definition := strings.TrimSpace(base.Description); definition != "" {
		return definition
	}
	if name := strings.TrimSpace(base.Name); name != "" {
		return name
	}
	return base.FullyQualifiedName
}

// termMetadata records where a term came from, in the same shape assets
// carry, plus the API address of the term itself.
func (c *collector) termMetadata(base entityBase, glossaryName string) map[string]interface{} {
	metadata := map[string]interface{}{}
	c.stampProvenance(base, metadata)

	if om, ok := metadata["openmetadata"].(map[string]interface{}); ok {
		putIf(om, "href", base.Href)
		// stampProvenance addresses an entity by its kind, which it only
		// knows for assets. Glossaries and their terms share one route.
		putIf(om, "url", c.entityURL(base, "glossary"))
	}
	putIf(metadata, "glossary", glossaryName)

	return metadata
}
