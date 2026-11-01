package assetdetail

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestFieldAnnotationsAndCounts(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	exec := func(q string) {
		t.Helper()
		if _, err := db.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO assets (id, name, mrn, type, providers, created_by) VALUES
		('a1', 'orders', 'mrn://table/postgresql/orders', 'Table', '{postgresql}', 't'),
		('a2', 'orders_daily', 'mrn://table/dbt/orders_daily', 'Table', '{dbt}', 't')`)
	exec(`INSERT INTO glossary_terms (id, name, definition) VALUES ('11111111-1111-1111-1111-111111111111', 'Gross merchandise value', 'x')`)
	exec(`INSERT INTO lineage_events (event_id, event_time, event_type, event_data) VALUES ('33333333-3333-3333-3333-333333333333', now(), 'DIRECT', '{}')`)
	exec(`INSERT INTO lineage_edges (source_mrn, target_mrn, event_id, type) VALUES ('mrn://table/postgresql/orders', 'mrn://table/dbt/orders_daily', '33333333-3333-3333-3333-333333333333', 'DIRECT')`)

	s := NewStore(db)
	yes, no := true, false
	desc, empty := "Contact email", ""
	terms := []string{"11111111-1111-1111-1111-111111111111"}

	if err := s.SetFieldAnnotation(ctx, "a1", FieldAnnotationInput{Schema: "columns", Path: "customer.email", PII: &yes, Description: &desc, TermIDs: &terms}, "jane"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldAnnotation(ctx, "a1", FieldAnnotationInput{Schema: "columns", Path: "order_id", Certified: &yes}, "jane"); err != nil {
		t.Fatal(err)
	}
	unknown := []string{"22222222-2222-2222-2222-222222222222"}
	if err := s.SetFieldAnnotation(ctx, "a1", FieldAnnotationInput{Path: "x", TermIDs: &unknown}, "jane"); err == nil {
		t.Fatal("an unknown glossary term was accepted")
	}

	got, err := s.ListFieldAnnotations(ctx, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "customer.email" || !got[0].PII || got[0].Description == nil || *got[0].Description != desc ||
		len(got[0].Terms) != 1 || got[0].Terms[0].Name != "Gross merchandise value" || !got[1].Certified || got[1].PII {
		t.Fatalf("unexpected annotations %+v", got)
	}

	// Setting one marker keeps the rest.
	if err := s.SetFieldAnnotation(ctx, "a1", FieldAnnotationInput{Schema: "columns", Path: "customer.email", Certified: &yes}, "sam"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListFieldAnnotations(ctx, "a1")
	if !got[0].PII || !got[0].Certified || got[0].UpdatedBy != "sam" {
		t.Fatalf("a partial update lost markers: %+v", got[0])
	}

	// Clearing every marker removes the field.
	if err := s.SetFieldAnnotation(ctx, "a1", FieldAnnotationInput{Schema: "columns", Path: "customer.email", PII: &no, Certified: &no, Description: &empty, TermIDs: &[]string{}}, "sam"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFieldAnnotation(ctx, "a1", "columns", "order_id"); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.ListFieldAnnotations(ctx, "a1"); len(got) != 0 {
		t.Fatalf("expected no annotations, got %+v", got)
	}

	c, err := s.TabCounts(ctx, "a1", "mrn://table/postgresql/orders", false, QueryScope{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Lineage != 1 || c.Runs != 0 || c.Queries != nil {
		t.Fatalf("unexpected counts %+v", c)
	}
	c, err = s.TabCounts(ctx, "a2", "mrn://table/dbt/orders_daily", true, QueryScope{PrincipalType: "user", PrincipalID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Lineage != 1 || c.Queries == nil || *c.Queries != 0 {
		t.Fatalf("unexpected counts %+v", c)
	}
}
