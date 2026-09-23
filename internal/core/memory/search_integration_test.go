package memory

import (
	"context"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type noRecorder struct{}

func (noRecorder) RecordSearchQuery(context.Context, string, string)               {}
func (noRecorder) RecordAssetView(context.Context, string, string, string, string) {}
func (noRecorder) RecordDBQuery(context.Context, string, time.Duration, bool)      {}
func (noRecorder) WrapDBQuery(_ context.Context, _ string, fn func() error) error  { return fn() }
func (noRecorder) RecordCustomMetrics(context.Context, []metrics.Metric) error     { return nil }

func TestMemoryMakesEntitiesFindableInSearch(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	finder := search.NewPostgresRepository(pool, noRecorder{})

	find := func(q string) []string {
		t.Helper()
		results, _, _, err := finder.Search(ctx, search.Filter{Query: q, Limit: 20})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		return ids
	}

	payments := seedAsset(t, pool, "payments")
	seedAsset(t, pool, "orders")
	product := seedProduct(t, pool, "Marketing")

	m, err := svc.Remember(ctx, payments, RememberInput{Content: "Refund spike from the SUMMER25 promo code", Author: agent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Remember(ctx, product, RememberInput{Content: "SUMMER25 is excluded from the Q3 ROI report", Author: agent}); err != nil {
		t.Fatal(err)
	}

	if got := find("summer25"); !contains(got, payments.ID) || !contains(got, product.ID) || len(got) != 2 {
		t.Errorf("one word: want the payments asset and the product, got %v", got)
	}
	if got := find("summer25 promo"); len(got) != 1 || got[0] != payments.ID {
		t.Errorf("several words match within one memory text: want the payments asset, got %v", got)
	}

	// An entity named after the word ranks above one that only mentions it.
	named := seedAsset(t, pool, "summer25_campaigns")
	if got := find("summer25"); len(got) == 0 || got[0] != named.ID {
		t.Errorf("a name match should rank first, got %v", got)
	}

	if _, err := svc.Update(ctx, payments, m.ID, UpdateInput{Content: "Refund volume back to normal", Author: person}); err != nil {
		t.Fatal(err)
	}
	if got := find("summer25"); contains(got, payments.ID) {
		t.Errorf("an edited memory should stop matching, got %v", got)
	}
	if got := find("refund volume"); !contains(got, payments.ID) {
		t.Errorf("the edited text should match, got %v", got)
	}
	if err := svc.Forget(ctx, payments, m.ID); err != nil {
		t.Fatal(err)
	}
	if got := find("refund volume"); contains(got, payments.ID) {
		t.Errorf("a forgotten memory should stop matching, got %v", got)
	}
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestSearchIndexKeepsUpWithMemory(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	finder := search.NewPostgresRepository(pool, noRecorder{})
	find := func(q string) []string {
		t.Helper()
		results, _, _, err := finder.Search(ctx, search.Filter{Query: q, Limit: 20})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		return ids
	}

	t.Run("a stub that becomes an asset keeps its memory findable", func(t *testing.T) {
		stub := seedAsset(t, pool, "stubby")
		if _, err := pool.Exec(ctx, `UPDATE assets SET is_stub = TRUE WHERE id = $1`, stub.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Remember(ctx, stub, RememberInput{Content: "giraffeword lives here", Author: agent}); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE assets SET is_stub = FALSE WHERE id = $1`, stub.ID); err != nil {
			t.Fatal(err)
		}
		if got := find("giraffeword"); !contains(got, stub.ID) {
			t.Errorf("memory lost when the stub became an asset: %v", got)
		}
	})

	t.Run("concurrent writers both end up searchable", func(t *testing.T) {
		table := seedAsset(t, pool, "busy")
		insert := `INSERT INTO memories (asset_id, content, created_by_type, created_by_id, created_by_name,
			updated_by_type, updated_by_id, updated_by_name) VALUES ($1, $2, 'user', 'u', 'u', 'user', 'u', 'u')`
		first, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := first.Exec(ctx, insert, table.ID, "alphaword"); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := pool.Exec(ctx, insert, table.ID, "betaword")
			done <- err
		}()
		time.Sleep(200 * time.Millisecond) // let the second writer block on the lock
		if err := first.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		for _, word := range []string{"alphaword", "betaword"} {
			if got := find(word); !contains(got, table.ID) {
				t.Errorf("%s lost to the concurrent write: %v", word, got)
			}
		}
	})

	t.Run("an entity's own text outranks a memory-only match", func(t *testing.T) {
		described := seedAsset(t, pool, "described")
		if _, err := pool.Exec(ctx, `UPDATE assets SET description = 'okapi and some other text then stripes' WHERE id = $1`, described.ID); err != nil {
			t.Fatal(err)
		}
		remembered := seedAsset(t, pool, "remembered")
		if _, err := svc.Remember(ctx, remembered, RememberInput{Content: "okapi stripes okapi stripes", Author: agent}); err != nil {
			t.Fatal(err)
		}
		got := find("okapi stripes")
		if len(got) < 2 || got[0] != described.ID {
			t.Errorf("own text should rank first: %v", got)
		}
	})
}
