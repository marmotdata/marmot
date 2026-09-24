package memory

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func seedProduct(t *testing.T, pool *pgxpool.Pool, name string) Entity {
	t.Helper()
	var id string
	if err := pool.QueryRow(t.Context(), `INSERT INTO data_products (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seeding data product: %v", err)
	}
	return Entity{Type: EntityDataProduct, ID: id}
}

var (
	agent  = Author{Type: "service_account", ID: "sa-1", Name: "orders-agent"}
	person = Author{Type: "user", ID: "user-1", Name: "Ada"}
)

func TestRememberAddsAndUpdateRecordsTheEditor(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	product := seedProduct(t, pool, "orders")

	first, err := svc.Remember(ctx, product, RememberInput{Content: "grain is one row per order", SessionID: "run-1", Author: agent})
	if err != nil {
		t.Fatalf("remember: %v", err)
	}
	if _, err := svc.Remember(ctx, product, RememberInput{Content: "grain is one row per order", SessionID: "run-2", Author: agent}); err != nil {
		t.Fatalf("remember again: %v", err)
	}
	all, err := svc.List(ctx, product, ListFilter{})
	if err != nil || all.Total != 2 {
		t.Fatalf("every remember adds an entry: %+v, %v", all, err)
	}

	edited, err := svc.Update(ctx, product, first.ID, UpdateInput{Content: "grain is one row per order line", SessionID: "review", Author: person})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if edited.CreatedBy.ID != agent.ID || edited.SessionID != "run-1" ||
		edited.UpdatedBy.ID != person.ID || edited.UpdatedSessionID != "review" || !edited.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("update should keep the creator and record the editor: %+v", edited)
	}

	recent, err := svc.List(ctx, product, ListFilter{Limit: 1})
	if err != nil || recent.Memories[0].ID != first.ID {
		t.Errorf("the most recently changed entry comes first: %+v, %v", recent, err)
	}
	session, err := svc.List(ctx, product, ListFilter{Filter: Filter{SessionID: "review"}})
	if err != nil || session.Total != 1 {
		t.Errorf("a session filter matches entries edited in it: %+v, %v", session, err)
	}
}

func TestSearchMatchesWordsWithinOneEntity(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	product := seedProduct(t, pool, "orders")

	other := seedProduct(t, pool, "payments")

	if _, err := svc.Remember(ctx, product, RememberInput{Content: "partitions older than 90 days are archived", Author: agent}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Remember(ctx, other, RememberInput{Content: "partitions are never archived", Author: agent}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Search(ctx, product, SearchQuery{Query: "archived partitions"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Memories) != 1 || result.Memories[0].EntityID != product.ID {
		t.Errorf("want the one match on this entity, got %+v", result.Memories)
	}
}

func TestEntriesAreScopedToTheirEntity(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	orders := seedProduct(t, pool, "orders")
	payments := seedProduct(t, pool, "payments")

	m, err := svc.Remember(ctx, orders, RememberInput{Content: "note", Author: agent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, payments, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get through another entity: %v", err)
	}
	if _, err := svc.Update(ctx, payments, m.ID, UpdateInput{Content: "changed", Author: agent}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update through another entity: %v", err)
	}
	if err := svc.Forget(ctx, payments, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("forget through another entity: %v", err)
	}
	if _, err := svc.Remember(ctx, Entity{Type: EntityDataProduct, ID: "00000000-0000-0000-0000-000000000000"}, RememberInput{Content: "x", Author: agent}); !errors.Is(err, ErrNotFound) {
		t.Errorf("remember on a missing entity: %v", err)
	}
}

func seedAsset(t *testing.T, pool *pgxpool.Pool, name string) Entity {
	t.Helper()
	id := "asset-" + name
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO assets (id, name, mrn, type, created_by) VALUES ($1, $2, $3, 'Table', 'test')`,
		id, name, "postgres://db/public/"+name); err != nil {
		t.Fatalf("seeding asset: %v", err)
	}
	return Entity{Type: EntityAsset, ID: id}
}

func TestAssetMemoryIsSeparateAndDeletedWithTheAsset(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	table := seedAsset(t, pool, "orders")
	product := seedProduct(t, pool, "orders")

	for _, e := range []Entity{table, product} {
		if _, err := svc.Remember(ctx, e, RememberInput{Content: "one row per " + string(e.Type), Author: agent}); err != nil {
			t.Fatalf("remember on %s: %v", e.Type, err)
		}
	}
	onAsset, err := svc.List(ctx, table, ListFilter{})
	if err != nil || onAsset.Total != 1 {
		t.Fatalf("asset memory: %+v, %v", onAsset, err)
	}
	if got := onAsset.Memories[0]; got.EntityType != EntityAsset || got.EntityID != table.ID || got.Content != "one row per asset" {
		t.Errorf("asset memory: %+v", got)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, table.ID); err != nil {
		t.Fatal(err)
	}
	if left, err := svc.List(ctx, table, ListFilter{}); err != nil || left.Total != 0 {
		t.Errorf("after deleting the asset: %+v, %v", left, err)
	}
	if left, err := svc.List(ctx, product, ListFilter{}); err != nil || left.Total != 1 {
		t.Errorf("product memory should survive: %+v, %v", left, err)
	}
}

func TestListSorts(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	product := seedProduct(t, pool, "orders")

	older, err := svc.Remember(ctx, product, RememberInput{Content: "written first", Author: agent})
	if err != nil {
		t.Fatal(err)
	}
	newer, err := svc.Remember(ctx, product, RememberInput{Content: "written second", Author: agent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, product, older.ID, UpdateInput{Content: "written first, edited last", Author: person}); err != nil {
		t.Fatal(err)
	}

	changed, err := svc.List(ctx, product, ListFilter{})
	if err != nil || changed.Memories[0].ID != older.ID {
		t.Errorf("by default the most recently changed comes first: %+v, %v", changed, err)
	}
	created, err := svc.List(ctx, product, ListFilter{Sort: SortCreated})
	if err != nil || created.Memories[0].ID != newer.ID {
		t.Errorf("sorted by creation the newest comes first: %+v, %v", created, err)
	}
	if _, err := svc.List(ctx, product, ListFilter{Sort: "size"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown sort: %v", err)
	}
}

func TestSearchAllReadsEveryEntity(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	table := seedAsset(t, pool, "orders")
	product := seedProduct(t, pool, "orders")

	for _, e := range []Entity{table, product} {
		if _, err := svc.Remember(ctx, e, RememberInput{Content: "partitions are archived after 90 days", Author: agent}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Remember(ctx, product, RememberInput{Content: "on-call is #orders", Author: agent}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.SearchAll(ctx, SearchQuery{Query: "archived partitions"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	got := map[EntityType]bool{}
	for _, m := range result.Memories {
		got[m.EntityType] = true
	}
	if len(result.Memories) != 2 || !got[EntityAsset] || !got[EntityDataProduct] {
		t.Errorf("want the match on the asset and on the product, got %+v", result.Memories)
	}
}

func TestRankingByUse(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	svc := NewService(NewPostgresRepository(pool))
	product := seedProduct(t, pool, "orders")

	remember := func(content string) *Memory {
		t.Helper()
		m, err := svc.Remember(ctx, product, RememberInput{Content: content, Author: agent})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	stale := remember("stale note nobody reads")
	popular := remember("grain is one row per order line")
	fresh := remember("fresh note written today")
	// Both older ones were written 60 days ago, four half-lives back.
	if _, err := pool.Exec(ctx, `UPDATE memories SET used_at = NOW() - interval '60 days' WHERE id = ANY($1::uuid[])`,
		[]string{stale.ID, popular.ID}); err != nil {
		t.Fatal(err)
	}

	// A search that does not count use leaves the count alone.
	if _, err := svc.Search(ctx, product, SearchQuery{Query: "grain"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		result, err := svc.Search(ctx, product, SearchQuery{Query: "grain", CountUse: true})
		if err != nil || len(result.Memories) != 1 {
			t.Fatalf("search: %+v, %v", result, err)
		}
	}
	got, err := svc.Get(ctx, product, popular.ID)
	if err != nil || got.FoundCount != 2 || got.LastFoundAt == nil {
		t.Errorf("searches should be counted: %+v, %v", got, err)
	}

	order := func(sort Sort) []string {
		t.Helper()
		list, err := svc.List(ctx, product, ListFilter{Sort: sort})
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(list.Memories))
		for i, m := range list.Memories {
			ids[i] = m.ID
		}
		return ids
	}
	if got, want := order(SortUsed), []string{popular.ID, fresh.ID, stale.ID}; !equal(got, want) {
		t.Errorf("most used: got %v, want recently found, then new, then stale", got)
	}

	// Editing counts as a use.
	if _, err := svc.Update(ctx, product, stale.ID, UpdateInput{Content: "stale note, now corrected", Author: person}); err != nil {
		t.Fatal(err)
	}
	if got := order(SortUsed); got[len(got)-1] == stale.ID {
		t.Errorf("an edited memory should no longer rank last: %v", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
