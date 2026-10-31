package recent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestRecordAndList(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	store := NewStore(db)

	var userID, otherID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('joe', 'Joe') RETURNING id`).Scan(&otherID))
	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by) VALUES
		('a1', 'orders', 'mrn://table/postgres/orders', 'Table', '{postgres}', 'test'),
		('a2', 'customers', 'mrn://table/snowflake/customers', 'Table', '{snowflake}', 'test')`)
	require.NoError(t, err)
	var termID, savedID, othersSaved string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO glossary_terms (name, definition) VALUES ('GMV', 'Gross merchandise value') RETURNING id`).Scan(&termID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO saved_queries (user_id, name, statement) VALUES ($1, 'Weekly refunds', 'select 1') RETURNING id`, userID).Scan(&savedID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO saved_queries (user_id, name, statement) VALUES ($1, 'Not yours', 'select 2') RETURNING id`, otherID).Scan(&othersSaved))

	id, err := store.ResolveAsset(ctx, "mrn://table/postgres/orders")
	require.NoError(t, err)
	assert.Equal(t, "a1", id)
	_, err = store.ResolveAsset(ctx, "mrn://table/postgres/missing")
	assert.ErrorIs(t, err, ErrItemNotFound)

	require.NoError(t, store.Record(ctx, userID, KindAsset, "a1"))
	require.NoError(t, store.Record(ctx, userID, KindGlossaryTerm, termID))
	require.NoError(t, store.Record(ctx, userID, KindSavedQuery, savedID))
	require.NoError(t, store.Record(ctx, userID, KindAsset, "a2"))

	assert.ErrorIs(t, store.Record(ctx, userID, KindSavedQuery, othersSaved), ErrItemNotFound, "someone else's saved query cannot be recorded")
	assert.ErrorIs(t, store.Record(ctx, userID, KindAsset, "nope"), ErrItemNotFound)
	assert.ErrorIs(t, store.Record(ctx, userID, "table", "a1"), ErrInvalidKind)

	// Spread the views out so the order does not depend on statements landing in the same instant
	for i, item := range []string{"a1", termID, savedID, "a2"} {
		_, err := db.Exec(ctx, `UPDATE recent_views SET viewed_at = NOW() - make_interval(mins => $2) WHERE item_id = $1`, item, 10-i)
		require.NoError(t, err)
	}

	all := []string{KindAsset, KindGlossaryTerm, KindDataProduct, KindSavedQuery}
	items, err := store.List(ctx, userID, all, 10)
	require.NoError(t, err)
	require.Len(t, items, 4)
	assert.Equal(t, "customers", items[0].Name)
	assert.Equal(t, "snowflake", items[0].Provider)
	assert.Equal(t, "mrn://table/snowflake/customers", items[0].MRN)
	assert.Equal(t, KindSavedQuery, items[1].Kind)
	assert.Equal(t, "Weekly refunds", items[1].Name)
	assert.Equal(t, "GMV", items[2].Name)
	assert.Equal(t, "orders", items[3].Name)

	// A repeat view inside the throttle window leaves the stored time alone; the spreading above moved a2 outside the window, so bring it back inside first
	_, err = db.Exec(ctx, `UPDATE recent_views SET viewed_at = NOW() - INTERVAL '10 seconds' WHERE item_id = 'a2'`)
	require.NoError(t, err)
	var before time.Time
	require.NoError(t, db.QueryRow(ctx, `SELECT viewed_at FROM recent_views WHERE item_id = 'a2'`).Scan(&before))
	require.NoError(t, store.Record(ctx, userID, KindAsset, "a2"))
	var after time.Time
	require.NoError(t, db.QueryRow(ctx, `SELECT viewed_at FROM recent_views WHERE item_id = 'a2'`).Scan(&after))
	assert.Equal(t, before, after)

	// Kinds the caller cannot see and items since deleted are left out
	items, err = store.List(ctx, userID, []string{KindSavedQuery}, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	_, err = db.Exec(ctx, `UPDATE glossary_terms SET deleted_at = NOW() WHERE id = $1`, termID)
	require.NoError(t, err)
	items, err = store.List(ctx, userID, all, 10)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	other, err := store.List(ctx, otherID, all, 10)
	require.NoError(t, err)
	assert.Empty(t, other)
}

func TestRecordKeepsLatest(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	store := NewStore(db)

	var userID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID))
	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by)
		SELECT 'a' || i, 'asset_' || i, 'mrn://table/postgres/asset_' || i, 'Table', '{postgres}', 'test' FROM generate_series(1, $1) AS i`, keepPerUser+5)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO recent_views (user_id, kind, item_id, viewed_at)
		SELECT $1, 'asset', 'a' || i, NOW() - make_interval(mins => 1000 - i) FROM generate_series(1, $2) AS i`, userID, keepPerUser+4)
	require.NoError(t, err)

	require.NoError(t, store.Record(ctx, userID, KindAsset, fmt.Sprintf("a%d", keepPerUser+5)))

	var count int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM recent_views WHERE user_id = $1`, userID).Scan(&count))
	assert.Equal(t, keepPerUser, count)
	var oldest bool
	require.NoError(t, db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM recent_views WHERE item_id = 'a1')`).Scan(&oldest))
	assert.False(t, oldest, "the oldest views are trimmed")
}

func TestValidKind(t *testing.T) {
	assert.True(t, ValidKind(KindDataProduct))
	assert.False(t, ValidKind("Asset"))
	assert.False(t, ValidKind(""))
}
