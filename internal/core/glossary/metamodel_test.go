package glossary

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

const termProfile = `formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: steward_area
    type: enum
    values: [finance, legal]
    required: true
    core: true
    storage: metadata.governance.steward_area
    appliesTo:
      kinds: [glossary_term]
    presentation:
      labelKey: example.steward_area.label
  - id: retention
    type: integer
    core: true
    storage: metadata.retention
    presentation:
      labelKey: example.retention.label
`

func TestTermsAreGovernedByTheProfile(t *testing.T) {
	registry, err := metamodel.Load(strings.NewReader(termProfile))
	if err != nil {
		t.Fatalf("a profile may scope fields to glossary_term: %v", err)
	}
	if ids := registry.Fields("glossary_term"); len(ids) != 1 || ids[0].ID != "steward_area" {
		t.Fatalf("glossary_term fields = %+v; asset-only fields must not apply", ids)
	}
	svc := NewService(newFakeRepo(), WithMetamodel(registry))
	ctx := context.Background()
	owners := []OwnerInput{{ID: "u1", Type: "user"}}
	var validation *metamodel.ValidationError

	_, err = svc.Create(ctx, CreateTermInput{Name: "Invoice", Definition: "A bill", Owners: owners, Metadata: map[string]interface{}{"governance": map[string]interface{}{"steward_area": "sales"}}})
	if !errors.As(err, &validation) {
		t.Fatalf("a value outside the enum must be refused, got %v", err)
	}
	// Required governed fields are reported as missing, never block a write (see #291).
	if _, err := svc.Create(ctx, CreateTermInput{Name: "Draft", Definition: "Not classified yet", Owners: owners}); err != nil {
		t.Fatalf("a missing required governed field must not block the write: %v", err)
	}
	term, err := svc.Create(ctx, CreateTermInput{Name: "Invoice", Definition: "A bill", Owners: owners, Metadata: map[string]interface{}{"governance": map[string]interface{}{"steward_area": "finance"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Update(ctx, term.ID, UpdateTermInput{Metadata: map[string]interface{}{"governance": map[string]interface{}{"steward_area": "hr"}}})
	if !errors.As(err, &validation) {
		t.Fatalf("an update is validated too, got %v", err)
	}
}

func TestTermsWithoutAProfileAreUnchanged(t *testing.T) {
	svc := NewService(newFakeRepo(), WithMetamodel(metamodel.Native()))
	if _, err := svc.Create(context.Background(), CreateTermInput{Name: "Invoice", Definition: "A bill", Owners: []OwnerInput{{ID: "u1", Type: "user"}}, Metadata: map[string]interface{}{"anything": 1}}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncKeepsGovernedValues(t *testing.T) {
	registry, err := metamodel.Load(strings.NewReader(termProfile))
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	svc := NewService(repo, WithMetamodel(registry))
	ctx := context.Background()
	if _, err := svc.SyncTerms(ctx, "erp", []TermInput{{Name: "Invoice", Definition: "A bill", Metadata: map[string]interface{}{"origin": "erp"}}}); err != nil {
		t.Fatal(err)
	}
	term := repo.byName(t, "Invoice")
	if _, err := svc.Update(ctx, term.ID, UpdateTermInput{Metadata: map[string]interface{}{"origin": "erp", "governance": map[string]interface{}{"steward_area": "legal"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncTerms(ctx, "erp", []TermInput{{Name: "Invoice", Definition: "A bill, reworded", Metadata: map[string]interface{}{"origin": "erp v2"}}}); err != nil {
		t.Fatal(err)
	}
	got := repo.byName(t, "Invoice").Metadata
	if v, _ := metamodel.ValueAt(got, "metadata.governance.steward_area"); v != "legal" {
		t.Fatalf("a sync wiped the curated governed value: %v", got)
	}
	if got["origin"] != "erp v2" {
		t.Fatalf("the source still owns its own metadata: %v", got)
	}
}
