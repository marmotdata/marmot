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
