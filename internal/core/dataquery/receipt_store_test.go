package dataquery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestQueryLogKeepsOriginAndRunnerReceipt(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	origin := "task_run:6f1c"
	byTask := &Query{PrincipalType: "user", PrincipalID: "alice", AuditSubject: "user:alice", Client: "api",
		SourceName: "warehouse", Statement: "SELECT 1", Decision: DecisionAllowed, Status: StatusRunning, Origin: &origin}
	require.NoError(t, repo.CreateQuery(ctx, byTask))
	byHand := &Query{PrincipalType: "user", PrincipalID: "alice", AuditSubject: "user:alice", Client: "api",
		SourceName: "warehouse", Statement: "SELECT 2", Decision: DecisionAllowed, Status: StatusRunning}
	require.NoError(t, repo.CreateQuery(ctx, byHand))

	finished := time.Now().UTC().Truncate(time.Second)
	rows := int64(3)
	receipt := &RunnerReceipt{Status: ReceiptInvalid, Problem: "its signature does not verify", QueryID: byTask.ID, Rows: 3,
		FinishedAt: &finished, Signed: []byte{1, 2}, Signature: []byte{3}, Certificate: []byte{4}}
	require.NoError(t, repo.CompleteQuery(ctx, byTask.ID, QueryOutcome{Status: StatusCompleted, RowsReturned: &rows, Receipt: receipt}))
	require.NoError(t, repo.CompleteQuery(ctx, byHand.ID, QueryOutcome{Status: StatusCompleted, RowsReturned: &rows}))

	got, err := repo.GetQuery(ctx, byTask.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Origin)
	assert.Equal(t, origin, *got.Origin)
	assert.Equal(t, receipt, got.RunnerReceipt, "an invalid receipt is stored as it arrived, with why it failed")

	plain, err := repo.GetQuery(ctx, byHand.ID)
	require.NoError(t, err)
	assert.Nil(t, plain.Origin)
	assert.Nil(t, plain.RunnerReceipt)

	listed, total, err := repo.ListQueries(ctx, QueryFilter{Origin: origin, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, byTask.ID, listed[0].ID)
}
