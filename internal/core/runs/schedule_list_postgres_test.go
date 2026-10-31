package runs

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchSchedulesFiltersAndCounts(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	svc := NewScheduleService(repo)

	var teamID string
	require.NoError(t, repo.db.QueryRow(ctx, `INSERT INTO teams (name) VALUES ('Data Platform') RETURNING id::text`).Scan(&teamID))

	description := "Catalogs the read replica of the checkout database."
	team := ScheduleOwnerTeam
	orders, err := svc.CreateScheduleWithDetails(ctx, "postgres-orders", "postgresql", map[string]interface{}{}, "", true, false, false, nil,
		ScheduleDetails{Description: &description, OwnerType: &team, OwnerID: &teamID})
	require.NoError(t, err)
	assert.Equal(t, description, *orders.Description)
	assert.Equal(t, "team", *orders.OwnerType)
	assert.Equal(t, "Data Platform", *orders.OwnerName)

	events, err := svc.CreateSchedule(ctx, "kafka-events", "kafka", map[string]interface{}{}, "", true, false, false, nil)
	require.NoError(t, err)
	_, err = svc.CreateSchedule(ctx, "postgres-billing", "postgresql", map[string]interface{}{}, "", true, false, false, nil)
	require.NoError(t, err)

	failed, err := svc.CreateJobRun(ctx, &orders.ID, "scheduler")
	require.NoError(t, err)
	msg := "connection refused"
	require.NoError(t, svc.CompleteJobRun(ctx, failed.ID, false, &msg, 0, 0, 0, 0, 0))
	_, err = svc.CreateJobRun(ctx, &events.ID, "scheduler")
	require.NoError(t, err)

	// The orders pipeline's last successful plugin run cataloged two assets, one of which a later deletion removed from the catalog.
	var pluginRun string
	require.NoError(t, repo.db.QueryRow(ctx, `
		INSERT INTO runs (pipeline_name, source_name, run_id, status, completed_at)
		VALUES ('postgres-orders', 'postgresql', 'r1', 'completed', NOW()) RETURNING id::text`).Scan(&pluginRun))
	for _, mrn := range []string{"mrn://table/postgresql/orders", "mrn://table/postgresql/gone"} {
		_, err = repo.db.Exec(ctx, `INSERT INTO run_checkpoints (run_id, entity_type, entity_mrn, operation) VALUES ($1, 'asset', $2, 'created')`, pluginRun, mrn)
		require.NoError(t, err)
	}
	_, err = repo.db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, created_by) VALUES ($1, 'orders', 'mrn://table/postgresql/orders', 'Table', 'test')`, uuid.NewString())
	require.NoError(t, err)

	names := func(filter ScheduleFilter) []string {
		t.Helper()
		filter.Limit = 50
		list, _, err := svc.SearchSchedules(ctx, filter)
		require.NoError(t, err)
		out := []string{}
		for _, s := range list {
			out = append(out, s.Name)
		}
		return out
	}

	assert.Equal(t, []string{"postgres-billing", "postgres-orders"}, names(ScheduleFilter{Search: "postgres"}))
	assert.Equal(t, []string{"kafka-events"}, names(ScheduleFilter{Search: "KAFKA"}))
	assert.Equal(t, []string{"postgres-orders"}, names(ScheduleFilter{Status: JobStatusFailed}))
	assert.Equal(t, []string{"kafka-events"}, names(ScheduleFilter{Status: ScheduleStatusRunning}))
	assert.Equal(t, []string{"postgres-billing"}, names(ScheduleFilter{Status: ScheduleStatusNever}))
	assert.Empty(t, names(ScheduleFilter{Status: JobStatusSucceeded}))

	_, total, err := svc.SearchSchedules(ctx, ScheduleFilter{Status: JobStatusFailed, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	_, _, err = svc.SearchSchedules(ctx, ScheduleFilter{Status: "bogus", Limit: 50})
	assert.ErrorIs(t, err, ErrInvalidJobStatus)

	got, err := svc.GetSchedule(ctx, orders.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.AssetCount)
	assert.Equal(t, JobStatusFailed, *got.LastRunStatus)

	counts, err := svc.CountSchedulesByPlugin(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"postgresql": 2, "kafka": 1}, counts)
}

func TestScheduleDetailsKeepAndClear(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	svc := NewScheduleService(repo)

	description := "Topics from the events cluster."
	created, err := svc.CreateScheduleWithDetails(ctx, "kafka-events", "kafka", map[string]interface{}{}, "", true, false, false, nil,
		ScheduleDetails{Description: &description})
	require.NoError(t, err)

	// An update that does not mention the details keeps them.
	updated, err := svc.UpdateSchedule(ctx, created.ID, "kafka-events", "kafka", map[string]interface{}{}, "", true, false, false)
	require.NoError(t, err)
	require.NotNil(t, updated.Description)
	assert.Equal(t, description, *updated.Description)

	empty := ""
	cleared, err := svc.UpdateScheduleWithDetails(ctx, created.ID, "kafka-events", "kafka", map[string]interface{}{}, "", true, false, false,
		ScheduleDetails{Description: &empty})
	require.NoError(t, err)
	assert.Nil(t, cleared.Description)

	user := ScheduleOwnerUser
	missing := uuid.NewString()
	_, err = svc.UpdateScheduleWithDetails(ctx, created.ID, "kafka-events", "kafka", map[string]interface{}{}, "", true, false, false,
		ScheduleDetails{OwnerType: &user, OwnerID: &missing})
	assert.ErrorIs(t, err, ErrScheduleOwnerNotFound)
}

func TestJobRunPhasesAndClaim(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	svc := NewScheduleService(repo)

	schedule, err := svc.CreateSchedule(ctx, "kafka-events", "kafka", map[string]interface{}{}, "", true, false, false, nil)
	require.NoError(t, err)
	run, err := svc.CreateJobRun(ctx, &schedule.ID, "scheduler")
	require.NoError(t, err)

	_, err = svc.ClaimJobRun(ctx, run.ID, "worker-1")
	require.NoError(t, err)
	_, err = svc.ClaimJobRun(ctx, run.ID, "worker-2")
	assert.ErrorIs(t, err, ErrJobRunNotClaimable, "a second replica must not run the same job")

	require.NoError(t, svc.MarkJobRunPhase(ctx, run.ID, JobPhaseConnecting))
	first, err := svc.GetJobRun(ctx, run.ID)
	require.NoError(t, err)
	require.NotNil(t, first.ConnectingAt)
	assert.Nil(t, first.ExtractingAt)

	require.NoError(t, svc.MarkJobRunPhase(ctx, run.ID, JobPhaseConnecting))
	again, err := svc.GetJobRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, first.ConnectingAt, again.ConnectingAt, "a phase keeps the time it was first reached")

	require.NoError(t, svc.MarkJobRunPhase(ctx, run.ID, JobPhaseExtracting))
	require.NoError(t, svc.MarkJobRunPhase(ctx, run.ID, JobPhaseWriting))
	got, err := svc.GetJobRun(ctx, run.ID)
	require.NoError(t, err)
	assert.NotNil(t, got.ExtractingAt)
	assert.NotNil(t, got.WritingAt)
	assert.Equal(t, "worker-1", *got.ClaimedBy)

	assert.Error(t, svc.MarkJobRunPhase(ctx, run.ID, "dancing"))
	assert.ErrorIs(t, svc.MarkJobRunPhase(ctx, uuid.NewString(), JobPhaseWriting), ErrJobRunNotFound)
}
