package dataquery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestListQueriesByTimeAndSearch(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	record := func(subject, statement string, mrns []string, startedAt time.Time) {
		t.Helper()
		q := &Query{PrincipalType: "user", PrincipalID: subject, AuditSubject: "user:" + subject, Client: "api",
			SourceName: "warehouse", Statement: statement, ReferencedMRNs: mrns, Decision: "allowed", Status: "completed"}
		require.NoError(t, repo.CreateQuery(ctx, q))
		_, err := db.Exec(ctx, `UPDATE data_queries SET started_at = $2 WHERE id = $1`, q.ID, startedAt)
		require.NoError(t, err)
	}
	now := time.Now().UTC()
	record("alice", "SELECT * FROM orders", []string{"mrn://table/postgres/shop.public.orders"}, now.Add(-time.Hour))
	record("bob", "SELECT 1", nil, now.Add(-3*24*time.Hour))
	record("carol", "SELECT * FROM refunds_100%", nil, now.Add(-10*24*time.Hour))

	names := func(f QueryFilter) []string {
		t.Helper()
		f.Limit = 50
		got, total, err := repo.ListQueries(ctx, f)
		require.NoError(t, err)
		out := make([]string, len(got))
		for i, q := range got {
			out[i] = q.PrincipalID
		}
		assert.Equal(t, total, len(got))
		return out
	}

	assert.Equal(t, []string{"alice", "bob"}, names(QueryFilter{From: now.Add(-7 * 24 * time.Hour)}))
	assert.Equal(t, []string{"bob"}, names(QueryFilter{From: now.Add(-7 * 24 * time.Hour), To: now.Add(-2 * time.Hour)}))

	assert.Equal(t, []string{"alice"}, names(QueryFilter{Search: "ALICE"}), "matches the caller ignoring case")
	assert.Equal(t, []string{"alice"}, names(QueryFilter{Search: "shop.public.orders"}), "matches a referenced table")
	assert.Equal(t, []string{"bob"}, names(QueryFilter{Search: "select 1"}), "matches the statement")
	// A wildcard in the search is matched literally rather than as a pattern
	assert.Equal(t, []string{"carol"}, names(QueryFilter{Search: "100%"}))
	assert.Empty(t, names(QueryFilter{Search: "_00"}))
}

func TestListQueriesByOutcomeCallerAndID(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	record := func(subject, decision, status, reason string) *Query {
		t.Helper()
		q := &Query{PrincipalType: "user", PrincipalID: subject + "-id", AuditSubject: "user:" + subject, Client: "editor",
			SourceName: "warehouse", Statement: "SELECT 1", Decision: decision, Status: status, OutcomeReason: reason}
		require.NoError(t, repo.CreateQuery(ctx, q))
		return q
	}
	allowed := record("alice", DecisionAllowed, StatusCompleted, ReasonRole)
	record("bob", DecisionDenied, StatusDenied, ReasonPermission)
	record("carol", DecisionAllowed, StatusFailed, ReasonTimeout)
	record("dan", DecisionAllowed, StatusCancelled, ReasonCancelled)
	// A row recorded before reasons were stored still filters by a derived one.
	legacy := record("erin", DecisionAllowed, StatusFailed, "")

	subjects := func(f QueryFilter) []string {
		t.Helper()
		f.Limit = 50
		got, total, err := repo.ListQueries(ctx, f)
		require.NoError(t, err)
		assert.Equal(t, total, len(got))
		out := []string{}
		for _, q := range got {
			out = append(out, q.AuditSubject)
		}
		return out
	}

	assert.Equal(t, []string{"user:alice"}, subjects(QueryFilter{Outcome: OutcomeAllowed}))
	assert.Equal(t, []string{"user:bob"}, subjects(QueryFilter{Outcome: OutcomeDenied, Reason: ReasonPermission}))
	assert.ElementsMatch(t, []string{"user:carol", "user:erin"}, subjects(QueryFilter{Outcome: OutcomeFailed}))
	assert.Equal(t, []string{"user:erin"}, subjects(QueryFilter{Reason: ReasonSourceError}))
	assert.Equal(t, []string{"user:dan"}, subjects(QueryFilter{Outcome: OutcomeCancelled}))
	assert.Equal(t, []string{"user:alice"}, subjects(QueryFilter{Caller: "user:alice"}), "by audit subject")
	assert.Equal(t, []string{"user:alice"}, subjects(QueryFilter{Caller: "user:alice-id"}), "by principal id")
	assert.Equal(t, []string{"user:alice"}, subjects(QueryFilter{ID: allowed.ID}))

	got, err := repo.GetQuery(ctx, legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, ReasonSourceError, got.OutcomeReason)

	var exported int
	require.NoError(t, repo.EachQuery(ctx, QueryFilter{Outcome: OutcomeFailed, Limit: 1}, func(*Query) error {
		exported++
		return nil
	}))
	assert.Equal(t, 2, exported, "an export ignores the page size")
}

func TestAccessRequestsAndTableFacts(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	req := &AccessRequest{PrincipalType: "user", PrincipalID: "u1", AuditSubject: "user:alice", SourceName: "warehouse", Permission: "assets:query", Needed: "assets:query on warehouse"}
	require.NoError(t, repo.CreateAccessRequest(ctx, req))
	assert.NotEmpty(t, req.ID)

	found, err := repo.RecentAccessRequest(ctx, "user", "u1", "warehouse", "assets:query on warehouse", time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, req.ID, found.ID)
	none, err := repo.RecentAccessRequest(ctx, "user", "u1", "other", "assets:query on other", time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Nil(t, none)

	facts, err := repo.TableFacts(ctx, []string{"no-such-asset"})
	require.NoError(t, err)
	assert.Empty(t, facts)
}

func TestRowCountOf(t *testing.T) {
	n := rowCountOf(map[string]any{"num_rows": "418230"})
	require.NotNil(t, n)
	assert.Equal(t, int64(418230), *n)
	assert.Nil(t, rowCountOf(map[string]any{"row_count": "lots"}))
	assert.Nil(t, rowCountOf(nil))
}
