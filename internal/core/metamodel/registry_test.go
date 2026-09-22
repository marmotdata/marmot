package metamodel

import (
	"errors"
	"strings"
	"testing"
)

const exampleProfile = `formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: retention
    type: integer
    core: true
    required: true
    storage: metadata.example.retention
    validation:
      minimum: 1
    presentation:
      labelKey: example.retention.label
`

func TestProfileExtendsNativeWithoutChangingIdentity(t *testing.T) {
	r, err := Load(strings.NewReader(exampleProfile))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Field("tags"); !ok {
		t.Fatal("native tags missing")
	}
	if f, ok := r.Field("name"); !ok || f.Storage != "marmot.name" || !f.Required {
		t.Fatal("native name contract changed")
	}
	if !r.Enabled() || len(r.Schema().Fields) != len(Native().Schema().Fields)+1 {
		t.Fatal("profile was not composed")
	}
	if err := r.Validate(map[string]any{"name": "table", "retention": 30.0}, true); err != nil {
		t.Fatal(err)
	}
	for _, values := range []map[string]any{{"name": "table"}, {"name": "table", "retention": 0.0}, {"name": "table", "retention": "30"}} {
		var invalid *ValidationError
		if err := r.Validate(values, true); !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "retention" {
			t.Fatalf("expected retention violation: %v", err)
		}
	}
	snapshot := r.Schema()
	snapshot.Fields[0].Required = false
	if !r.Schema().Fields[0].Required {
		t.Fatal("schema snapshot mutated registry")
	}
	r2, _ := Load(strings.NewReader(exampleProfile))
	if r.Schema().Hash != r2.Schema().Hash {
		t.Fatal("unstable schema hash")
	}
}

func TestRejectInvalidDefinitions(t *testing.T) {
	for name, document := range map[string]string{
		"unknown property":        exampleProfile + "unexpected: true\n",
		"shadow native ownership": strings.ReplaceAll(exampleProfile, "retention", "owners"),
		"shadow native identity":  strings.ReplaceAll(exampleProfile, "retention", "mrn"),
		"duplicate YAML key":      exampleProfile + "id: replacement\n",
		"unknown type":            strings.Replace(exampleProfile, "type: integer", "type: executable", 1),
		"invalid binding":         strings.Replace(exampleProfile, "metadata.example.retention", "marmot.mrn", 1),
		"prototype binding":       strings.Replace(exampleProfile, "metadata.example.retention", "metadata.constructor.retention", 1),
		"required nullable":       strings.Replace(exampleProfile, "required: true", "required: true\n    nullable: true", 1),
		"format version":          strings.Replace(exampleProfile, "formatVersion: 1", "formatVersion: 2", 1),
		"native override":         strings.ReplaceAll(strings.ReplaceAll(exampleProfile, "retention", "name"), "metadata.example.name", "marmot.name"),
		"duplicate id":            exampleProfile + "  - id: retention\n    type: string\n    storage: metadata.example.other\n    core: true\n",
		"irrelevant constraint":   strings.Replace(exampleProfile, "minimum: 1", "minLength: 1", 1),
		"unsupported reference":   strings.Replace(exampleProfile, "type: integer", "type: reference", 1),
		"nullable native tags":    "formatVersion: 1\nid: example\nversion: 1\ndefaultLocale: en\nfields:\n  - id: tags\n    type: list\n    itemType: string\n    core: true\n    nullable: true\n    storage: marmot.tags\n",
		"facet on integer field":  strings.Replace(exampleProfile, "labelKey: example.retention.label", "labelKey: example.retention.label\n      facet: true", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(document)); err == nil {
				t.Fatal("accepted invalid profile")
			}
		})
	}
	if _, err := Load(strings.NewReader(strings.Repeat("x", MaxProfileBytes+1))); err == nil {
		t.Fatal("accepted oversized profile")
	}
}

func TestOptionalCoreAndGovernedApplicability(t *testing.T) {
	profile := strings.Replace(exampleProfile, "required: true", "required: false", 1)
	r, err := Load(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(map[string]any{"name": "table"}, true); err != nil {
		t.Fatal(err)
	}
	profile = strings.Replace(exampleProfile, "required: true", "required: true\n    appliesTo: governed_assets", 1)
	r, err = Load(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(map[string]any{"name": "stub"}, false); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(map[string]any{"name": "table"}, true); err == nil {
		t.Fatal("required field skipped for governed asset")
	}
}

func TestSupportedValues(t *testing.T) {
	for _, tc := range []struct {
		field     Field
		good, bad any
	}{
		{Field{Type: "boolean"}, false, "false"},
		{Field{Type: "integer"}, 3.0, 3.5},
		{Field{Type: "date"}, "2026-09-17", "2026-02-30"},
		{Field{Type: "enum", Values: []string{"internal", "public"}}, "internal", "translated value"},
		{Field{Type: "list", ItemType: "integer"}, []any{1.0, 2.0}, []any{1.0, "2"}},
	} {
		t.Run(tc.field.Type, func(t *testing.T) {
			if code := validateValue(tc.field, tc.good); code != "" {
				t.Fatalf("valid value rejected: %s", code)
			}
			if code := validateValue(tc.field, tc.bad); code == "" {
				t.Fatal("invalid value accepted")
			}
		})
	}
}

func TestFacetPresentation(t *testing.T) {
	profile := `formatVersion: 1
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
`
	r, err := Load(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	f, ok := r.Field("classification")
	if !ok || !f.Presentation.Facet {
		t.Fatal("facet flag not carried into the registered field")
	}
}

func TestMessagesCatalog(t *testing.T) {
	profile := exampleProfile + "messages:\n  en:\n    example.retention.label: Retention (days)\n  es:\n    example.retention.label: Retención (días)\n"
	r, err := Load(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	messages := r.Schema().Messages
	if messages["en"]["example.retention.label"] != "Retention (days)" || messages["es"]["example.retention.label"] != "Retención (días)" {
		t.Fatalf("messages not carried into schema: %v", messages)
	}
	r2, _ := Load(strings.NewReader(exampleProfile))
	if r.Schema().Hash == r2.Schema().Hash {
		t.Fatal("messages did not change the schema hash")
	}
}

func TestRejectInvalidMessages(t *testing.T) {
	for name, document := range map[string]string{
		"invalid locale": exampleProfile + "messages:\n  \"en us\":\n    k: v\n",
		"invalid key":    exampleProfile + "messages:\n  en:\n    \"bad key\": v\n",
		"empty value":    exampleProfile + "messages:\n  en:\n    k: \"\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(document)); err == nil {
				t.Fatal("accepted invalid messages")
			}
		})
	}
}
