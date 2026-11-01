package dataquery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestSavedQueryInputValidation(t *testing.T) {
	ok := SavedQueryInput{Name: "  Weekly refunds ", SQL: "select 1", Source: " warehouse "}
	require.NoError(t, ok.normalise())
	assert.Equal(t, "Weekly refunds", ok.Name)
	assert.Equal(t, "warehouse", ok.Source)

	for name, in := range map[string]SavedQueryInput{
		"no name":   {SQL: "select 1"},
		"no sql":    {Name: "x", SQL: "  "},
		"long name": {Name: strings.Repeat("x", maxSavedQueryName+1), SQL: "select 1"},
		"long sql":  {Name: "x", SQL: strings.Repeat("x", maxSavedQueryStatement+1)},
	} {
		assert.ErrorIs(t, in.normalise(), ErrInvalidInput, name)
	}
}

func TestSavedQueryStoreIsScopedToOwner(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	store := NewSavedQueryStore(db)

	var jane, joe string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&jane))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('joe', 'Joe') RETURNING id`).Scan(&joe))

	first, err := store.Create(ctx, jane, SavedQueryInput{Name: "Refunds", SQL: "select * from refunds", Source: "warehouse"})
	require.NoError(t, err)
	assert.Equal(t, "warehouse", first.Source)
	assert.Nil(t, first.LastRunAt)
	second, err := store.Create(ctx, jane, SavedQueryInput{Name: "Orders", SQL: "select * from orders"})
	require.NoError(t, err)

	list, total, err := store.List(ctx, jane, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, list, 2)

	_, err = db.Exec(ctx, `UPDATE saved_queries SET updated_at = NOW() - interval '1 day'`)
	require.NoError(t, err)
	ran, err := store.MarkRun(ctx, jane, first.ID)
	require.NoError(t, err)
	require.NotNil(t, ran.LastRunAt)
	list, _, err = store.List(ctx, jane, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, first.ID, list[0].ID, "the query run last comes first")

	_, err = store.Get(ctx, joe, first.ID)
	assert.ErrorIs(t, err, ErrSavedQueryNotFound)
	_, err = store.Update(ctx, joe, first.ID, SavedQueryInput{Name: "Mine now", SQL: "select 1"})
	assert.ErrorIs(t, err, ErrSavedQueryNotFound)
	_, err = store.MarkRun(ctx, joe, first.ID)
	assert.ErrorIs(t, err, ErrSavedQueryNotFound)
	assert.ErrorIs(t, store.Delete(ctx, joe, first.ID), ErrSavedQueryNotFound)
	others, total, err := store.List(ctx, joe, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, others)
	assert.Zero(t, total)

	updated, err := store.Update(ctx, jane, second.ID, SavedQueryInput{Name: "Orders today", SQL: "select 2", Source: "shop"})
	require.NoError(t, err)
	assert.Equal(t, "Orders today", updated.Name)
	assert.Equal(t, "select 2", updated.SQL)
	assert.WithinDuration(t, time.Now(), updated.UpdatedAt, time.Minute)

	require.NoError(t, store.Delete(ctx, jane, second.ID))
	_, err = store.Get(ctx, jane, second.ID)
	assert.ErrorIs(t, err, ErrSavedQueryNotFound)
}

func TestAgentSummaries(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	var bot, quiet string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO service_accounts (name) VALUES ('support-bot') RETURNING id`).Scan(&bot))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO service_accounts (name) VALUES ('quiet-bot') RETURNING id`).Scan(&quiet))

	record := func(principalType, principalID, decision string, mrns []string, at time.Time) {
		t.Helper()
		status := StatusCompleted
		if decision == DecisionDenied {
			status = StatusDenied
		}
		q := &Query{PrincipalType: principalType, PrincipalID: principalID, AuditSubject: principalType + ":" + principalID, Client: "mcp",
			SourceName: "warehouse", Statement: "select 1", ReferencedMRNs: mrns, Decision: decision, Status: StatusRunning}
		require.NoError(t, repo.CreateQuery(ctx, q))
		_, err := db.Exec(ctx, `UPDATE data_queries SET started_at = $2 WHERE id = $1`, q.ID, at)
		require.NoError(t, err)
		require.NoError(t, repo.CompleteQuery(ctx, q.ID, QueryOutcome{Status: status}))
	}
	orders := []string{"mrn://table/postgres/orders"}
	refunds := []string{"mrn://table/postgres/refunds"}
	record("service_account", bot, DecisionAllowed, orders, now.Add(-time.Hour))
	record("service_account", bot, DecisionDenied, refunds, now.Add(-2*time.Hour))
	record("service_account", bot, DecisionAllowed, orders, now.Add(-3*24*time.Hour))
	record("service_account", quiet, DecisionAllowed, orders, now.Add(-10*24*time.Hour))
	record("user", "jane", DecisionAllowed, orders, now.Add(-time.Hour))

	agents, err := repo.AgentSummaries(ctx, AgentSummaryFilter{Now: now})
	require.NoError(t, err)
	require.Len(t, agents, 1, "users are not agents and the list only holds agents active this week")
	got := agents[0]
	assert.Equal(t, bot, got.PrincipalID)
	assert.Equal(t, "support-bot", got.Name)
	assert.EqualValues(t, 2, got.Queries24h)
	assert.EqualValues(t, 3, got.Queries7d)
	assert.EqualValues(t, 1, got.Refused24h)
	assert.EqualValues(t, 1, got.Refused7d)
	assert.EqualValues(t, 2, got.TablesRead7d)
	require.NotNil(t, got.LastActiveAt)
	assert.WithinDuration(t, now.Add(-time.Hour), *got.LastActiveAt, time.Second)

	// Asking for an agent quiet all week still returns it, with when it last queried
	one, err := repo.AgentSummaries(ctx, AgentSummaryFilter{Now: now, PrincipalID: quiet})
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, "quiet-bot", one[0].Name)
	assert.Zero(t, one[0].Queries7d)
	require.NotNil(t, one[0].LastActiveAt)
	assert.WithinDuration(t, now.Add(-10*24*time.Hour), *one[0].LastActiveAt, time.Second)

	none, err := repo.AgentSummaries(ctx, AgentSummaryFilter{Now: now, PrincipalID: "00000000-0000-0000-0000-000000000000"})
	require.NoError(t, err)
	assert.Empty(t, none)
}
