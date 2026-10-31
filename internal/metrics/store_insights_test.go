package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestZeroResultQueriesAreKeptApart(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	store := NewPostgresStore(db)
	now := time.Now().UTC()
	require.NoError(t, store.CreatePartition(ctx, now))

	search := func(name, query string) Metric {
		return Metric{Name: name, Type: Counter, Value: 1, Labels: map[string]string{"query_type": "full_text", "query": query}, Timestamp: now}
	}
	require.NoError(t, store.RecordMetrics(ctx, []Metric{
		search(MetricSearchQueries, "orders"),
		search(MetricSearchQueries, "orders"),
		search(MetricSearchQueriesEmpty, "gmv"),
		search(MetricSearchQueriesEmpty, "gmv"),
		search(MetricSearchQueriesEmpty, "chrun"),
	}))

	window := TimeRange{Start: now.Add(-time.Hour), End: now.Add(time.Hour)}
	top, err := store.GetTopQueries(ctx, window, 10)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, "orders", top[0].Query)
	assert.Nil(t, top[0].ResultCount)

	empty, err := store.GetZeroResultQueries(ctx, window, 10)
	require.NoError(t, err)
	require.Len(t, empty, 2)
	assert.Equal(t, "gmv", empty[0].Query)
	assert.EqualValues(t, 2, empty[0].Count)
	require.NotNil(t, empty[0].ResultCount)
	assert.Zero(t, *empty[0].ResultCount)
}

func TestAssetsOwned(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	store := NewPostgresStore(db)

	var userID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID))
	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by, is_stub) VALUES
		('a1', 'orders', 'mrn://table/postgres/orders', 'Table', '{postgres}', 'test', false),
		('a2', 'refunds', 'mrn://table/postgres/refunds', 'Table', '{postgres}', 'test', false),
		('a3', 'stub', 'mrn://table/postgres/stub', 'Table', '{postgres}', 'test', true)`)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO asset_owners (asset_id, user_id) VALUES ('a1', $1), ('a3', $1)`, userID)
	require.NoError(t, err)

	owned, total, err := store.GetAssetsOwned(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, owned)
	assert.EqualValues(t, 2, total, "stubs are not catalogued assets")
}
