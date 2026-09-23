package dataproduct

import (
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

const productProfile = `formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: cost_center
    type: string
    core: true
    required: true
    storage: metadata.example.cost_center
    appliesTo:
      kinds: [data_product]
    presentation:
      labelKey: example.cost_center.label
  - id: retention
    type: integer
    core: true
    storage: metadata.example.retention
    appliesTo:
      kinds: [asset]
    presentation:
      labelKey: example.retention.label
`

func mustLoadProductProfile(t *testing.T) *metamodel.Registry {
	t.Helper()
	r, err := metamodel.Load(strings.NewReader(productProfile))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestValidateMetamodelNilOrDisabledIsNoop(t *testing.T) {
	if err := validateMetamodel(nil, map[string]any{"anything": "goes"}); err != nil {
		t.Fatalf("nil registry must not validate: %v", err)
	}
	if err := validateMetamodel(metamodel.Native(), map[string]any{"anything": "goes"}); err != nil {
		t.Fatalf("disabled registry must not validate: %v", err)
	}
}

func TestValidateMetamodelChecksDataProductFieldsOnly(t *testing.T) {
	registry := mustLoadProductProfile(t)

	// Absent required governed field is completeness, not validity: never blocks.
	if err := validateMetamodel(registry, map[string]any{}); err != nil {
		t.Fatalf("a missing required field must not block: %v", err)
	}

	// A present but wrong-typed value still fails.
	if err := validateMetamodel(registry, map[string]any{"example": map[string]any{"cost_center": 3}}); err == nil {
		t.Fatal("wrong type accepted")
	}

	// A valid value passes.
	if err := validateMetamodel(registry, map[string]any{"example": map[string]any{"cost_center": "CC-1"}}); err != nil {
		t.Fatalf("valid value rejected: %v", err)
	}

	// A field scoped to asset only must never surface as a data_product violation, even with a
	// bad value: it isn't part of Fields("data_product") at all.
	if err := validateMetamodel(registry, map[string]any{"example": map[string]any{"cost_center": "CC-1", "retention": "not-a-number"}}); err != nil {
		t.Fatalf("an asset-only field must not affect data_product validity: %v", err)
	}
}

func TestValidateMetamodelReturnsValidationError(t *testing.T) {
	registry := mustLoadProductProfile(t)
	err := validateMetamodel(registry, map[string]any{"example": map[string]any{"cost_center": 3}})
	var invalid *metamodel.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "cost_center" {
		t.Fatalf("expected a cost_center validation error: %v", err)
	}
}
