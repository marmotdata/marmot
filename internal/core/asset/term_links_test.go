package asset

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

func TestTermLinksSayWhichRuleLinkedEachAsset(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db, quietRecorder{})

	var termID, ruleID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO glossary_terms (name, definition) VALUES ('GMV', 'Orders by value') RETURNING id::text`).Scan(&termID))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO asset_rules (name, rule_type, query_expression) VALUES ('Revenue definitions', 'query', 'type:table') RETURNING id::text`).Scan(&ruleID))
	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by)
		VALUES ('a1', 'orders', 'mrn://table/pg/orders', 'table', '{postgresql}', 'test'),
		       ('a2', 'orders_daily', 'mrn://table/pg/orders_daily', 'table', '{postgresql}', 'test'),
		       ('a3', 'charges', 'mrn://table/pg/charges', 'table', '{postgresql}', 'test')`)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO asset_terms (asset_id, glossary_term_id, source)
		VALUES ('a1', $1, 'user'), ('a3', $1, 'plugin:dbt'), ('a2', $1, 'rule:' || $2)`, termID, ruleID)
	require.NoError(t, err)

	groups, err := repo.GetTermLinkGroups(ctx, termID)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, TermLinkManual, groups[0].Source)
	assert.Equal(t, 2, groups[0].Count)
	assert.Equal(t, "rule", groups[1].Source)
	require.NotNil(t, groups[1].RuleName)
	assert.Equal(t, "Revenue definitions", *groups[1].RuleName)
	assert.Equal(t, ruleID, *groups[1].RuleID)

	manual, total, err := repo.GetAssetsByTermLink(ctx, termID, TermLinkManual, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, "charges", *manual[0].Name)

	ruled, total, err := repo.GetAssetsByTermLink(ctx, termID, "rule:"+ruleID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "a2", ruled[0].ID)

	links, err := repo.GetTermLinks(ctx, termID, []string{"a1", "a2"})
	require.NoError(t, err)
	byAsset := map[string]TermLink{}
	for _, l := range links {
		byAsset[l.AssetID] = l
	}
	assert.Equal(t, TermLinkManual, byAsset["a1"].Source)
	assert.Equal(t, "rule", byAsset["a2"].Source)
	assert.Equal(t, "Revenue definitions", *byAsset["a2"].RuleName)

	_, _, err = repo.GetAssetsByTermLink(ctx, termID, "nonsense", 10, 0)
	assert.Error(t, err)

	got, err := repo.GetByIDs(ctx, []string{"a3", "missing", "a1"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a3", got[0].ID)
	assert.Equal(t, "a1", got[1].ID)
}
