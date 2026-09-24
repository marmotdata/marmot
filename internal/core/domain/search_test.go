package domain_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type noopRecorder struct{ metrics.Recorder }

func (noopRecorder) RecordDBQuery(context.Context, string, time.Duration, bool) {}

func TestSearchByDomainSubtree(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	svc := domain.NewService(domain.NewPostgresRepository(pool))
	repo := search.NewPostgresRepository(pool, noopRecorder{})
	repo.SetDomainResolver(domain.SearchResolver(svc))

	finance := mustCreate(t, svc, "Finance", nil)
	payments := mustCreate(t, svc, "Payments", finance)
	legal := mustCreate(t, svc, "Legal", nil)

	inPayments, inLegal, unassigned := pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool)
	for id, d := range map[string]string{inPayments: payments.ID, inLegal: legal.ID} {
		if err := svc.Assign(ctx, domain.KindAsset, []string{id}, d); err != nil {
			t.Fatal(err)
		}
	}
	var term string
	if err := pool.QueryRow(ctx, "INSERT INTO glossary_terms (name, definition) VALUES ('Invoice', 'A bill') RETURNING id").Scan(&term); err != nil {
		t.Fatal(err)
	}
	if err := svc.Assign(ctx, domain.KindGlossaryTerm, []string{term}, finance.ID); err != nil {
		t.Fatal(err)
	}

	ids := func(query string) []string {
		t.Helper()
		results, total, _, err := repo.Search(ctx, search.Filter{Query: query, Limit: 50})
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		out := make([]string, 0, len(results))
		for _, r := range results {
			out = append(out, r.ID)
		}
		if total != 0 && total != len(out) {
			t.Fatalf("search %q: total %d for %d results", query, total, len(out))
		}
		sort.Strings(out)
		return out
	}
	want := func(values ...string) []string {
		sort.Strings(values)
		return values
	}
	equal := func(t *testing.T, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}

	t.Run("a domain matches its subtree across kinds", func(t *testing.T) {
		equal(t, ids("@domain:"+finance.ID), want(inPayments, term))
	})
	t.Run("a leaf matches only itself", func(t *testing.T) {
		equal(t, ids("@domain:"+payments.ID), want(inPayments))
	})
	t.Run("several domains are ORed", func(t *testing.T) {
		equal(t, ids("@domain:"+payments.ID+" @domain:"+legal.ID), want(inPayments, inLegal))
	})
	t.Run("unassigned matches entities without a row", func(t *testing.T) {
		got := ids("@domain:" + domain.UnassignedID + " @kind:asset")
		equal(t, got, want(unassigned))
	})
	t.Run("an unknown domain matches nothing", func(t *testing.T) {
		equal(t, ids("@domain:00000000-0000-4000-8000-00000000ffff"), nil)
		equal(t, ids("@domain:Nowhere"), nil)
	})
	t.Run("a name matches its subtree, ignoring case", func(t *testing.T) {
		equal(t, ids("@domain:finance"), want(inPayments, term))
		equal(t, ids(`@kind:asset AND @domain = "Finance"`), want(inPayments))
	})
	t.Run("a name path is followed from a root", func(t *testing.T) {
		equal(t, ids(`@domain:"Finance/Payments"`), want(inPayments))
		equal(t, ids(`@domain:"Legal/Payments"`), nil)
	})
	t.Run("unknown references are ignored next to known ones", func(t *testing.T) {
		equal(t, ids("@domain:Legal @domain:Nowhere"), want(inLegal))
	})
	t.Run("NOT excludes a subtree", func(t *testing.T) {
		equal(t, ids("@kind:asset NOT @domain:Legal"), want(inPayments, unassigned))
		equal(t, ids(`@domain:Finance NOT @domain:"Finance/Payments"`), want(term))
		equal(t, ids("NOT @domain:Unassigned @kind:asset"), want(inPayments, inLegal))
	})
	t.Run("excluding an unknown domain excludes nothing", func(t *testing.T) {
		equal(t, ids("@kind:asset NOT @domain:Nowhere"), want(inPayments, inLegal, unassigned))
	})
	t.Run("a repeated name matches every domain that has it", func(t *testing.T) {
		archive := mustCreate(t, svc, "Archive", finance)
		legalArchive := mustCreate(t, svc, "Archive", legal)
		a, b := pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool)
		for id, d := range map[string]string{a: archive.ID, b: legalArchive.ID} {
			if err := svc.Assign(ctx, domain.KindAsset, []string{id}, d); err != nil {
				t.Fatal(err)
			}
		}
		equal(t, ids("@domain:archive"), want(a, b))
		equal(t, ids(`@domain:"Legal/Archive"`), want(b))
	})
	t.Run("Unassigned resolves by name and by its translated name", func(t *testing.T) {
		equal(t, ids("@domain:Unassigned @kind:asset"), want(unassigned))
		equal(t, ids(`@domain:"sin asignar" @kind:asset`), want(unassigned))
	})
	t.Run("a real domain named like an alias wins", func(t *testing.T) {
		real := mustCreate(t, svc, "Sin asignar", nil)
		asset := pgtest.SeedAsset(t, pool)
		if err := svc.Assign(ctx, domain.KindAsset, []string{asset}, real.ID); err != nil {
			t.Fatal(err)
		}
		equal(t, ids(`@domain:"Sin asignar" @kind:asset`), want(asset))
	})
}
