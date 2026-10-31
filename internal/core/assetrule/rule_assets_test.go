package assetrule

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/enrichment"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type quietRecorder struct{}

func (quietRecorder) RecordSearchQuery(context.Context, string, string)               {}
func (quietRecorder) RecordAssetView(context.Context, string, string, string, string) {}
func (quietRecorder) RecordDBQuery(context.Context, string, time.Duration, bool)      {}
func (quietRecorder) WrapDBQuery(_ context.Context, _ string, fn func() error) error  { return fn() }
func (quietRecorder) RecordCustomMetrics(context.Context, []metrics.Metric) error     { return nil }

func TestRuleAssetsAndAuthors(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db, quietRecorder{})
	members := NewPostgresMembershipRepository(db, quietRecorder{})

	var alexID, samID, termID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('alex', 'Alex Taylor') RETURNING id::text`).Scan(&alexID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('sam', 'Sam Jones') RETURNING id::text`).Scan(&samID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO glossary_terms (name, definition) VALUES ('Personal data', 'PII') RETURNING id::text`).Scan(&termID))

	q := "tags:pii"
	now := time.Now().UTC()
	rule := &AssetRule{Name: "Personal data", RuleType: enrichment.RuleTypeQuery, QueryExpression: &q, TermIDs: []string{termID},
		Links: []ExternalLink{{Name: "Handling guide", URL: "https://example.com/pii"}}, IsEnabled: true, CreatedBy: &alexID, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.Create(ctx, rule))

	got, err := repo.Get(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alex Taylor", *got.CreatedByName)
	assert.Equal(t, "Alex Taylor", *got.UpdatedByName, "a rule nobody edited was last changed by its author")

	got.UpdatedBy = &samID
	require.NoError(t, repo.Update(ctx, got))
	got.UpdatedBy = nil
	require.NoError(t, repo.Update(ctx, got))
	got, err = repo.Get(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, "Sam Jones", *got.UpdatedByName, "an edit with no person behind it keeps the last editor")

	for _, term := range []string{"handling", "personal"} {
		found, err := repo.Search(ctx, SearchFilter{Query: term, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 1, found.Total, term)
	}

	_, err = db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by)
		VALUES ('a1', 'customers', 'mrn://table/pg/customers', 'table', '{postgresql}', 'test'),
		       ('a2', 'orders', 'mrn://table/pg/orders', 'table', '{postgresql}', 'test'),
		       ('a3', 'customer_events', 'mrn://topic/kafka/customer_events', 'topic', '{kafka}', 'test')`)
	require.NoError(t, err)
	require.NoError(t, members.CreateMemberships(ctx, rule.ID, []string{"a1", "a2", "a3"}))

	all, total, err := members.SearchMembershipAssets(ctx, rule.ID, "", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, all, 2)
	assert.Equal(t, "customer_events", all[0].Name)
	assert.Equal(t, []string{"kafka"}, all[0].Providers)

	customers, total, err := members.SearchMembershipAssets(ctx, rule.ID, "customer", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, customers, 2)
}
