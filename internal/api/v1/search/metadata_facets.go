package search

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/internal/core/search"
)

const governedFilterPrefix = "governed."

// parseGovernedFilters reads governed.<field id>=value1,value2 query params, resolving each
// field against the metamodel registry. Unknown fields, non-facetable fields, and values that
// don't match the field's declared type are dropped rather than rejected: a stale client (an
// old profile cached in the browser) should degrade to no filter, not a 400.
func (h *Handler) parseGovernedFilters(queryValues map[string][]string) map[string][]string {
	filters := make(map[string][]string)
	for key, raw := range queryValues {
		id, ok := strings.CutPrefix(key, governedFilterPrefix)
		if !ok || len(raw) == 0 {
			continue
		}
		field, ok := h.metamodelRegistry.Field(id)
		if !ok || !field.Presentation.Facet || !slices.Contains(field.AppliesTo.EffectiveKinds(), "asset") {
			continue
		}
		var literals []string
		for _, group := range raw {
			for _, value := range strings.Split(group, ",") {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				if literal, err := governedContainment(field, value); err == nil {
					literals = append(literals, literal)
				}
			}
		}
		if len(literals) > 0 {
			filters[field.Storage] = literals
		}
	}
	return filters
}

// metadataFacetSpecs builds a facet spec for every field the profile marks facetable, so
// listing queries always report counts for the values Discover can filter by.
func (h *Handler) metadataFacetSpecs() []search.MetadataFacetSpec {
	var specs []search.MetadataFacetSpec
	for _, field := range h.metamodelRegistry.Fields("asset") {
		if !field.Presentation.Facet {
			continue
		}
		candidates := field.Values
		if field.Type == "boolean" {
			candidates = []string{"true", "false"}
		}
		values := make([]search.MetadataFacetValue, 0, len(candidates))
		for _, value := range candidates {
			if literal, err := governedContainment(field, value); err == nil {
				values = append(values, search.MetadataFacetValue{Value: value, Literal: literal})
			}
		}
		if len(values) > 0 {
			specs = append(specs, search.MetadataFacetSpec{Key: field.Storage, Values: values})
		}
	}
	return specs
}

// governedContainment builds the JSONB containment fragment that selects assets whose
// governed field holds value, e.g. `{"example":{"classification":"public"}}`.
func governedContainment(field metamodel.Field, value string) (string, error) {
	segments := strings.Split(strings.TrimPrefix(field.Storage, "metadata."), ".")
	if len(segments) == 0 || segments[0] == "" {
		return "", fmt.Errorf("field %q has no metadata binding", field.ID)
	}
	var leaf any = value
	switch field.Type {
	case "boolean":
		switch value {
		case "true":
			leaf = true
		case "false":
			leaf = false
		default:
			return "", fmt.Errorf("invalid boolean facet value %q", value)
		}
	case "enum":
		if !slices.Contains(field.Values, value) {
			return "", fmt.Errorf("value %q is not a declared member of %q", value, field.ID)
		}
	default:
		return "", fmt.Errorf("field %q is not facetable", field.ID)
	}
	var doc any = leaf
	for i := len(segments) - 1; i >= 0; i-- {
		doc = map[string]any{segments[i]: doc}
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
