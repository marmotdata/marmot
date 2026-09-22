package search

import (
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

const facetProfile = `formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: classification
    type: enum
    core: true
    storage: metadata.example.classification
    values: [public, confidential]
    presentation:
      labelKey: example.classification.label
      facet: true
  - id: contains_pii
    type: boolean
    core: true
    nullable: true
    storage: metadata.example.contains_pii
    presentation:
      labelKey: example.contains_pii.label
      facet: true
  - id: retention
    type: integer
    core: true
    storage: metadata.example.retention
    presentation:
      labelKey: example.retention.label
`

func facetRegistry(t *testing.T) *metamodel.Registry {
	t.Helper()
	r, err := metamodel.Load(strings.NewReader(facetProfile))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestGovernedContainment(t *testing.T) {
	r := facetRegistry(t)
	classification, _ := r.Field("classification")
	pii, _ := r.Field("contains_pii")
	retention, _ := r.Field("retention")

	for name, tc := range map[string]struct {
		field metamodel.Field
		value string
		want  string
	}{
		"enum":         {classification, "public", `{"example":{"classification":"public"}}`},
		"boolean true": {pii, "true", `{"example":{"contains_pii":true}}`},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := governedContainment(tc.field, tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}

	for name, tc := range map[string]struct {
		field metamodel.Field
		value string
	}{
		"undeclared enum value": {classification, "restricted"},
		"non-boolean value":     {pii, "yes"},
		"non-facetable type":    {retention, "30"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := governedContainment(tc.field, tc.value); err == nil {
				t.Fatal("accepted an invalid facet value")
			}
		})
	}
}

func TestParseGovernedFilters(t *testing.T) {
	h := &Handler{metamodelRegistry: facetRegistry(t)}

	filters := h.parseGovernedFilters(map[string][]string{
		"governed.classification": {"public, confidential"},
		"governed.contains_pii":   {"true"},
		"governed.retention":      {"30"},  // not facetable, dropped
		"governed.unknown":        {"x"},   // unknown field, dropped
		"q":                       {"foo"}, // not a governed. key, ignored
	})

	if len(filters) != 2 {
		t.Fatalf("expected 2 governed filters, got %d: %v", len(filters), filters)
	}
	classification := filters["metadata.example.classification"]
	if len(classification) != 2 {
		t.Fatalf("expected 2 classification literals, got %v", classification)
	}
	pii := filters["metadata.example.contains_pii"]
	if len(pii) != 1 || pii[0] != `{"example":{"contains_pii":true}}` {
		t.Fatalf("unexpected contains_pii literal: %v", pii)
	}
}

func TestMetadataFacetSpecs(t *testing.T) {
	h := &Handler{metamodelRegistry: facetRegistry(t)}

	specs := h.metadataFacetSpecs()
	if len(specs) != 2 {
		t.Fatalf("expected 2 facetable specs, got %d: %+v", len(specs), specs)
	}
	byKey := make(map[string][]string)
	for _, spec := range specs {
		for _, v := range spec.Values {
			byKey[spec.Key] = append(byKey[spec.Key], v.Value)
		}
	}
	if len(byKey["metadata.example.classification"]) != 2 {
		t.Fatalf("expected 2 classification candidates, got %v", byKey["metadata.example.classification"])
	}
	if len(byKey["metadata.example.contains_pii"]) != 2 {
		t.Fatalf("expected true/false candidates, got %v", byKey["metadata.example.contains_pii"])
	}
}
