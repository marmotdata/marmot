package dataproduct

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

func productNames(r *ListResult) []string {
	out := []string{}
	for _, p := range r.DataProducts {
		out = append(out, p.Name)
	}
	return out
}

func TestBrowsingProductsOnTheServer(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db, quietRecorder{})
	svc := NewService(repo)

	var janeID, kenjiID, teamID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane Doe') RETURNING id::text`).Scan(&janeID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('kenji', 'Kenji Sato') RETURNING id::text`).Scan(&kenjiID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO teams (name) VALUES ('Analytics Engineering') RETURNING id::text`).Scan(&teamID))
	_, err := db.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, source) VALUES ($1, $2, 'member', 'manual')`, teamID, janeID)
	require.NoError(t, err)

	commerce, err := svc.Create(ctx, CreateInput{Name: "Commerce core", Tags: []string{"commerce"}, Owners: []OwnerInput{{ID: teamID, Type: "team"}}})
	require.NoError(t, err)
	support, err := svc.Create(ctx, CreateInput{Name: "Support operations", Tags: []string{"support"}, Owners: []OwnerInput{{ID: kenjiID, Type: "user"}}})
	require.NoError(t, err)
	_, err = svc.Create(ctx, CreateInput{Name: "Web analytics", Tags: []string{"web", "marketing"}, Owners: []OwnerInput{{ID: kenjiID, Type: "user"}}})
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by)
		VALUES ('a1', 'orders', 'mrn://table/pg/orders', 'table', '{postgresql}', 'test'),
		       ('a2', 'orders_daily', 'mrn://table/pg/orders_daily', 'table', '{postgresql}', 'test'),
		       ('a3', 'shop.orders.v2', 'mrn://topic/kafka/shop.orders.v2', 'topic', '{kafka}', 'test'),
		       ('a4', 'tickets', 'mrn://table/pg/tickets', 'table', '{postgresql}', 'test')`)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO asset_owners (asset_id, team_id) VALUES ('a2', $1)`, teamID)
	require.NoError(t, err)
	require.NoError(t, repo.AddAssets(ctx, commerce.ID, []string{"a1"}, ""))
	require.NoError(t, repo.AddAssets(ctx, support.ID, []string{"a4"}, ""))

	q := "type:table"
	ruleA, err := repo.CreateRule(ctx, commerce.ID, &RuleInput{Name: "Order marts", RuleType: RuleTypeQuery, QueryExpression: &q, IsEnabled: true})
	require.NoError(t, err)
	ruleB, err := repo.CreateRule(ctx, commerce.ID, &RuleInput{Name: "Order events", RuleType: RuleTypeQuery, QueryExpression: &q, IsEnabled: true})
	require.NoError(t, err)
	assert.Equal(t, 0, ruleA.Position)
	assert.Equal(t, 1, ruleB.Position)
	_, err = db.Exec(ctx, `INSERT INTO data_product_memberships (data_product_id, asset_id, source, rule_id)
		VALUES ($1, 'a2', 'rule', $2), ($1, 'a3', 'rule', $3)`, commerce.ID, ruleA.ID, ruleB.ID)
	require.NoError(t, err)

	// Commerce assets are viewed, support's are not
	_, err = db.Exec(ctx, `INSERT INTO metrics_timeseries (metric_name, metric_type, labels, hour, day, total_sum)
		VALUES ('asset_views_total', 'counter', '{"asset_id":"a2"}', date_trunc('hour', now()), CURRENT_DATE, 40),
		       ('asset_views_total', 'counter', '{"asset_id":"a1"}', date_trunc('hour', now()), CURRENT_DATE, 3)`)
	require.NoError(t, err)

	mine, err := svc.ListFiltered(ctx, ListFilter{Mine: MineOnly, UserID: janeID})
	require.NoError(t, err)
	assert.Equal(t, []string{"Commerce core"}, productNames(mine), "products of the user's teams count as theirs")

	rest, err := svc.ListFiltered(ctx, ListFilter{Mine: MineExclude, UserID: janeID})
	require.NoError(t, err)
	assert.Equal(t, []string{"Support operations", "Web analytics"}, productNames(rest))

	tagged, err := svc.ListFiltered(ctx, ListFilter{Tags: []string{"marketing", "support"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Support operations", "Web analytics"}, productNames(tagged))

	owned, err := svc.ListFiltered(ctx, ListFilter{OwnerIDs: []string{teamID}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Commerce core"}, productNames(owned))

	searched, err := svc.ListFiltered(ctx, ListFilter{Query: "web"})
	require.NoError(t, err)
	assert.Equal(t, []string{"Web analytics"}, productNames(searched))

	popular, err := svc.ListFiltered(ctx, ListFilter{Sort: SortPopular})
	require.NoError(t, err)
	assert.Equal(t, "Commerce core", popular.DataProducts[0].Name)
	assert.Equal(t, 4, popular.DataProducts[0].Popularity)
	assert.Equal(t, 0, popular.DataProducts[1].Popularity)

	_, err = svc.ListFiltered(ctx, ListFilter{Sort: "random"})
	assert.ErrorIs(t, err, ErrInvalidInput)

	got, err := svc.Get(ctx, commerce.ID)
	require.NoError(t, err)
	assert.Equal(t, []TypeCount{{Type: "table", Count: 2}, {Type: "topic", Count: 1}}, got.AssetTypes)
	require.Len(t, got.Rules, 2)
	assert.Equal(t, 1, got.Rules[0].MatchedAssetCount)

	members, err := svc.GetMembers(ctx, commerce.ID, MemberFilter{Source: SourceRule, RuleID: ruleA.ID})
	require.NoError(t, err)
	require.Equal(t, 1, members.Total)
	m := members.Members[0]
	assert.Equal(t, "orders_daily", m.Name)
	assert.Equal(t, "Order marts", *m.RuleName)
	assert.Equal(t, []string{"Analytics Engineering"}, m.OwnerNames)
	assert.Equal(t, 4, m.Popularity)

	found, err := svc.GetMembers(ctx, commerce.ID, MemberFilter{Query: "shop"})
	require.NoError(t, err)
	require.Equal(t, 1, found.Total)
	assert.Equal(t, "a3", found.Members[0].AssetID)

	byPopularity, err := svc.GetMembers(ctx, commerce.ID, MemberFilter{Sort: SortPopular})
	require.NoError(t, err)
	assert.Equal(t, []string{"orders_daily", "orders", "shop.orders.v2"}, []string{byPopularity.Members[0].Name, byPopularity.Members[1].Name, byPopularity.Members[2].Name})

	reordered, err := svc.ReorderRules(ctx, commerce.ID, []string{ruleB.ID, ruleA.ID})
	require.NoError(t, err)
	assert.Equal(t, ruleB.ID, reordered[0].ID)
	assert.Equal(t, 0, reordered[0].Position)

	_, err = svc.ReorderRules(ctx, commerce.ID, []string{ruleB.ID})
	assert.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.ReorderRules(ctx, commerce.ID, []string{ruleB.ID, ruleB.ID})
	assert.ErrorIs(t, err, ErrInvalidInput)

	facets, err := svc.GetFacets(ctx)
	require.NoError(t, err)
	assert.Len(t, facets.Owners, 2)
	assert.Equal(t, "Analytics Engineering", facets.Owners[0].Name)
	assert.Equal(t, 2, facets.Owners[1].Count)
	assert.Len(t, facets.Tags, 4)
}
