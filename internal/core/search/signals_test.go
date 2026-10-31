package search

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type quietRecorder struct{}

func (quietRecorder) RecordSearchQuery(context.Context, string, string)               {}
func (quietRecorder) RecordAssetView(context.Context, string, string, string, string) {}
func (quietRecorder) RecordDBQuery(context.Context, string, time.Duration, bool)      {}
func (quietRecorder) WrapDBQuery(_ context.Context, _ string, fn func() error) error  { return fn() }
func (quietRecorder) RecordCustomMetrics(context.Context, []metrics.Metric) error     { return nil }

func names(results []*Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Name
	}
	return out
}

func TestSearchSignalsSortAndOwners(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()

	var userID, teamID, termID, productID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane Doe') RETURNING id::text`).Scan(&userID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO teams (name) VALUES ('Finance') RETURNING id::text`).Scan(&teamID))
	_, err := db.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, source) VALUES ($1, $2, 'owner', 'manual')`, teamID, userID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO assets (id, name, mrn, type, providers, tags, metadata, schema, created_by, updated_at) VALUES
			('a1', 'orders', 'mrn://table/pg/orders', 'table', '{postgresql}', '{pii,finance}', '{"row_count": 1200}', '{"columns": "[{\"name\":\"id\"},{\"name\":\"total\"}]"}', 'test', now() - interval '3 days'),
			('a2', 'customers', 'mrn://table/pg/customers', 'table', '{postgresql}', '{pii}', '{}', '{}', 'test', now() - interval '1 day'),
			('a3', 'billing_events', 'mrn://topic/kafka/billing_events', 'topic', '{kafka}', '{}', '{}', '{}', 'test', now())`)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO asset_owners (asset_id, user_id) VALUES ('a1', $1)`, userID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO asset_owners (asset_id, team_id) VALUES ('a1', $1), ('a3', $1)`, teamID)
	require.NoError(t, err)

	require.NoError(t, db.QueryRow(ctx, `INSERT INTO glossary_terms (name, definition) VALUES ('Order value', 'What an order is worth') RETURNING id::text`).Scan(&termID))
	_, err = db.Exec(ctx, `INSERT INTO asset_terms (asset_id, glossary_term_id) VALUES ('a1', $1), ('a2', $1)`, termID)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO data_products (name) VALUES ('Orders product') RETURNING id::text`).Scan(&productID))
	_, err = db.Exec(ctx, `INSERT INTO data_product_memberships (data_product_id, asset_id, source) VALUES ($1, 'a1', 'manual')`, productID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO data_product_owners (data_product_id, team_id) VALUES ($1, $2)`, productID, teamID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO metrics_timeseries (metric_name, metric_type, labels, hour, day, total_sum) VALUES
			('asset_views_total', 'counter', '{"asset_id":"a2"}', date_trunc('hour', now()), CURRENT_DATE, 90),
			('asset_views_total', 'counter', '{"asset_id":"a3"}', date_trunc('hour', now()), CURRENT_DATE, 4)`)
	require.NoError(t, err)

	svc := NewService(NewPostgresRepository(db, quietRecorder{}))
	assetsOnly := []ResultType{ResultTypeAsset}

	t.Run("sorts listings", func(t *testing.T) {
		resp, err := svc.Search(ctx, Filter{Types: assetsOnly, Sort: SortName})
		require.NoError(t, err)
		assert.Equal(t, []string{"billing_events", "customers", "orders"}, names(resp.Results))

		resp, err = svc.Search(ctx, Filter{Types: assetsOnly, Sort: SortPopular})
		require.NoError(t, err)
		assert.Equal(t, []string{"customers", "billing_events", "orders"}, names(resp.Results))

		resp, err = svc.Search(ctx, Filter{Types: assetsOnly, Sort: SortUpdated})
		require.NoError(t, err)
		assert.Equal(t, []string{"billing_events", "customers", "orders"}, names(resp.Results))
	})

	t.Run("sorts text searches", func(t *testing.T) {
		for _, q := range []string{"or", "orders", "orders customers", "@tag = \"pii\""} {
			_, err := svc.Search(ctx, Filter{Query: q, Sort: SortPopular})
			require.NoError(t, err, q)
		}
		resp, err := svc.Search(ctx, Filter{Query: `@tag = "PII"`, Sort: SortName})
		require.NoError(t, err)
		assert.Equal(t, []string{"customers", "orders"}, names(resp.Results))
		resp, err = svc.Search(ctx, Filter{Query: `@tag = "fin*"`})
		require.NoError(t, err)
		assert.Equal(t, []string{"orders"}, names(resp.Results))
	})

	t.Run("filters and facets by owner", func(t *testing.T) {
		resp, err := svc.Search(ctx, Filter{Owners: []string{teamID}, Sort: SortName})
		require.NoError(t, err)
		assert.Equal(t, []string{"billing_events", "orders", "Orders product"}, names(resp.Results))

		resp, err = svc.Search(ctx, Filter{})
		require.NoError(t, err)
		require.NotEmpty(t, resp.Facets.Owners)
		assert.Equal(t, OwnerFacetValue{Value: teamID, Label: "Finance", Kind: "team", Count: 3}, resp.Facets.Owners[0])
		assert.Equal(t, OwnerFacetValue{Value: userID, Label: "Jane Doe", Kind: "user", Count: 1}, resp.Facets.Owners[1])

		resp, err = svc.Search(ctx, Filter{Types: assetsOnly, Tags: []string{"pii"}})
		require.NoError(t, err)
		assert.Len(t, resp.Facets.Owners, 2)
	})

	t.Run("enriches results", func(t *testing.T) {
		resp, err := svc.Search(ctx, Filter{})
		require.NoError(t, err)
		byName := map[string]*Result{}
		for _, r := range resp.Results {
			byName[r.Name] = r
		}

		orders := byName["orders"]
		require.NotNil(t, orders)
		require.Len(t, orders.Owners, 2)
		assert.Equal(t, ResultOwner{ID: userID, Name: "Jane Doe", Kind: "user"}, orders.Owners[0])
		assert.Equal(t, "team", orders.Owners[1].Kind)
		require.NotNil(t, orders.RowCount)
		assert.Equal(t, int64(1200), *orders.RowCount)
		require.NotNil(t, orders.ColumnCount)
		assert.Equal(t, 2, *orders.ColumnCount)
		require.NotNil(t, orders.Popularity)
		assert.Equal(t, 0, *orders.Popularity)
		require.NotNil(t, orders.Stale)
		assert.False(t, *orders.Stale)
		assert.Equal(t, 4, *byName["customers"].Popularity)

		term := byName["Order value"]
		require.NotNil(t, term)
		assert.Equal(t, 2, *term.LinkedAssetCount)

		product := byName["Orders product"]
		require.NotNil(t, product)
		assert.Equal(t, 1, *product.AssetCount)
		require.Len(t, product.Owners, 1)

		team := byName["Finance"]
		require.NotNil(t, team)
		assert.Equal(t, 2, *team.AssetCount)
		assert.Equal(t, 1, *team.MemberCount)
	})
}
