package domain_test

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestImportFromMetadata(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	svc := domain.NewService(domain.NewPostgresRepository(pool))
	finance := mustCreate(t, svc, "Finance", nil)
	legal := mustCreate(t, svc, "Legal", nil)

	seed := func(value string) string {
		t.Helper()
		id := pgtest.SeedAsset(t, pool)
		if _, err := pool.Exec(ctx, `UPDATE assets SET metadata = jsonb_build_object('dgu', jsonb_build_object('domain', $2::text)) WHERE id = $1`, id, value); err != nil {
			t.Fatal(err)
		}
		return id
	}
	fin1, fin2, other := seed("Finanzas"), seed("Finanzas"), seed("Otro")
	pinned := seed("Finanzas")
	if err := svc.Assign(ctx, domain.KindAsset, []string{pinned}, legal.ID); err != nil {
		t.Fatal(err)
	}
	var term, deletedTerm string
	for _, row := range []struct {
		dest    *string
		deleted bool
	}{{&term, false}, {&deletedTerm, true}} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO glossary_terms (name, definition, metadata, deleted_at)
			VALUES (gen_random_uuid()::text, 'd', '{"dgu":{"domain":"Legal"}}', CASE WHEN $1 THEN now() END)
			RETURNING id`, row.deleted).Scan(row.dest); err != nil {
			t.Fatal(err)
		}
	}
	in := domain.ImportInput{
		Source:  "metadata.dgu.domain",
		Mapping: map[string]string{"Finanzas": finance.ID, "Legal": legal.ID},
	}

	report, err := svc.Import(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if report.Applied || report.AlreadyAssigned != 1 {
		t.Fatalf("dry run report = %+v", report)
	}
	wantMapped := []domain.ImportValue{
		{Kind: domain.KindAsset, Value: "Finanzas", Count: 2, DomainID: finance.ID},
		{Kind: domain.KindGlossaryTerm, Value: "Legal", Count: 1, DomainID: legal.ID},
	}
	if len(report.Mapped) != len(wantMapped) || report.Mapped[0] != wantMapped[0] || report.Mapped[1] != wantMapped[1] {
		t.Fatalf("mapped = %+v", report.Mapped)
	}
	if len(report.Unmapped) != 1 || report.Unmapped[0].Value != "Otro" {
		t.Fatalf("unmapped = %+v", report.Unmapped)
	}
	if got, _ := svc.DomainOf(ctx, domain.KindAsset, fin1); got != domain.UnassignedID {
		t.Fatal("a dry run wrote a membership")
	}

	in.Apply = true
	if report, err = svc.Import(ctx, in); err != nil || !report.Applied {
		t.Fatalf("apply: %+v, %v", report, err)
	}
	for id, want := range map[string]string{fin1: finance.ID, fin2: finance.ID, other: domain.UnassignedID, pinned: legal.ID} {
		if got, _ := svc.DomainOf(ctx, domain.KindAsset, id); got != want {
			t.Fatalf("asset %s in %q, want %q", id, got, want)
		}
	}
	if got, _ := svc.DomainOf(ctx, domain.KindGlossaryTerm, term); got != legal.ID {
		t.Fatalf("term in %q", got)
	}
	if got, _ := svc.DomainOf(ctx, domain.KindGlossaryTerm, deletedTerm); got != domain.UnassignedID {
		t.Fatal("a deleted term was imported")
	}

	_, err = svc.Import(ctx, domain.ImportInput{Source: "metadata.dgu.domain", Mapping: map[string]string{"x": "00000000-0000-4000-8000-00000000ffff"}})
	wantErr(t, err, domain.ErrNotFound)
	for _, source := range []string{"dgu.domain", "metadata", "metadata.dgu;drop", "metadata.a.b.c.d.e.f.g.h.i"} {
		_, err = svc.Import(ctx, domain.ImportInput{Source: source, Mapping: in.Mapping})
		wantErr(t, err, domain.ErrInvalidInput)
	}
}
