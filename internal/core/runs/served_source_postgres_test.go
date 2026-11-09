package runs

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPostgresRepo returns a repository over a fresh migrated database of its own. Without that variable the test is skipped, so `go test ./...` stays offline.
func newPostgresRepo(t *testing.T) (*SchedulePostgresRepository, context.Context) {
	t.Helper()
	pool := pgtest.TempDB(t)
	return &SchedulePostgresRepository{db: pool}, context.Background()
}

func servedBy(principalType string, id string) *Schedule {
	return &Schedule{
		Name:         "served-" + uuid.NewString()[:8],
		PluginID:     "postgresql",
		Config:       map[string]interface{}{"host": "db", "password": "********"},
		ServedByType: &principalType,
		ServedByID:   &id,
	}
}

func TestClaimServedSources(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	owner, other := uuid.NewString(), uuid.NewString()

	t.Run("first claim takes a free name and is never scheduled", func(t *testing.T) {
		s := servedBy("service_account", owner)
		require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{s}))

		got, err := repo.GetScheduleByName(ctx, s.Name)
		require.NoError(t, err)
		assert.True(t, got.Served())
		assert.True(t, got.Queryable)
		assert.Equal(t, "serve", *got.ManagedBy)
		assert.Equal(t, owner, *got.ServedByID)

		due, err := repo.GetSchedulesDueForRun(ctx, 1000)
		require.NoError(t, err)
		for _, d := range due {
			assert.NotEqual(t, s.Name, d.Name, "the server must never run a served source's plugin")
		}
	})

	t.Run("owner can claim again", func(t *testing.T) {
		s := servedBy("service_account", owner)
		require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{s}))
		again := servedBy("service_account", owner)
		again.Name = s.Name
		again.Config = map[string]interface{}{"host": "db2"}
		require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{again}))
		assert.Equal(t, s.ID, again.ID)
	})

	t.Run("another principal cannot take a served source", func(t *testing.T) {
		s := servedBy("service_account", owner)
		require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{s}))

		thief := servedBy("service_account", other)
		thief.Name = s.Name
		assert.ErrorIs(t, repo.ClaimServedSources(ctx, []*Schedule{thief}), ErrSourceOwned)

		sameIDOtherType := servedBy("user", owner)
		sameIDOtherType.Name = s.Name
		assert.ErrorIs(t, repo.ClaimServedSources(ctx, []*Schedule{sameIDOtherType}), ErrSourceOwned)

		got, err := repo.GetScheduleByName(ctx, s.Name)
		require.NoError(t, err)
		assert.Equal(t, owner, *got.ServedByID)
	})

	t.Run("a pipeline Marmot runs cannot be claimed", func(t *testing.T) {
		existing := &Schedule{Name: "server-" + uuid.NewString()[:8], PluginID: "postgresql", Config: map[string]interface{}{}, Enabled: true}
		require.NoError(t, repo.CreateSchedule(ctx, existing))

		claim := servedBy("service_account", owner)
		claim.Name = existing.Name
		assert.ErrorIs(t, repo.ClaimServedSources(ctx, []*Schedule{claim}), ErrSourceOwned)

		got, err := repo.GetScheduleByName(ctx, existing.Name)
		require.NoError(t, err)
		assert.False(t, got.Served())
	})

	t.Run("only the operator may serve a Run the operator manages", func(t *testing.T) {
		managed := "operator"
		run := &Schedule{Name: "run-" + uuid.NewString()[:8], PluginID: "postgresql", Config: map[string]interface{}{}, Enabled: true, ManagedBy: &managed}
		require.NoError(t, repo.UpsertSchedule(ctx, run))

		claim := servedBy("service_account", owner)
		claim.Name = run.Name
		assert.ErrorIs(t, repo.ClaimServedSources(ctx, []*Schedule{claim}), ErrSourceOwned)

		operatorClaim := servedBy("operator", "00000000-0000-0000-0000-000000000001")
		operatorClaim.Name = run.Name
		require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{operatorClaim}))
		assert.Equal(t, "operator", *operatorClaim.ManagedBy)
	})
}

// An operator Run must never take over a pipeline someone created in Marmot, or one that serves queries, just by sharing its name.
func TestUpsertScheduleOnlyUpdatesItsOwnManagersSchedules(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	operator := "operator"

	t.Run("a pipeline created in Marmot is left alone", func(t *testing.T) {
		existing := &Schedule{Name: "ui-" + uuid.NewString()[:8], PluginID: "postgresql", Config: map[string]interface{}{"host": "prod-db"}, Enabled: true}
		require.NoError(t, repo.CreateSchedule(ctx, existing))

		run := &Schedule{Name: existing.Name, PluginID: "kafka", Config: map[string]interface{}{"host": "attacker"}, Enabled: true, ManagedBy: &operator}
		assert.ErrorIs(t, repo.UpsertSchedule(ctx, run), ErrScheduleNameExists)

		got, err := repo.GetScheduleByName(ctx, existing.Name)
		require.NoError(t, err)
		assert.Equal(t, "postgresql", got.PluginID)
		assert.Equal(t, "prod-db", got.Config["host"])
		assert.Nil(t, got.ManagedBy)
	})

	t.Run("a served source is left alone", func(t *testing.T) {
		served := servedBy("service_account", uuid.NewString())
		require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{served}))

		run := &Schedule{Name: served.Name, PluginID: "kafka", Config: map[string]interface{}{}, Enabled: true, ManagedBy: &operator}
		assert.ErrorIs(t, repo.UpsertSchedule(ctx, run), ErrScheduleNameExists)

		got, err := repo.GetScheduleByName(ctx, served.Name)
		require.NoError(t, err)
		assert.Equal(t, "serve", *got.ManagedBy)
	})

	t.Run("the operator creates and then updates its own", func(t *testing.T) {
		name := "run-" + uuid.NewString()[:8]
		first := &Schedule{Name: name, PluginID: "postgresql", Config: map[string]interface{}{"host": "a"}, Enabled: true, ManagedBy: &operator}
		require.NoError(t, repo.UpsertSchedule(ctx, first))

		second := &Schedule{Name: name, PluginID: "postgresql", Config: map[string]interface{}{"host": "b"}, Enabled: true, ManagedBy: &operator}
		require.NoError(t, repo.UpsertSchedule(ctx, second))
		assert.Equal(t, first.ID, second.ID)

		got, err := repo.GetScheduleByName(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "b", got.Config["host"])
	})

	t.Run("a schedule without a manager cannot be upserted", func(t *testing.T) {
		assert.Error(t, repo.UpsertSchedule(ctx, &Schedule{Name: "x-" + uuid.NewString()[:8], PluginID: "postgresql", Enabled: true}))
	})
}

// An enrolment claims all its sources or none: if one name belongs to someone else, the free names it listed stay free.
func TestClaimServedSourcesIsAllOrNothing(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	taken := servedBy("service_account", uuid.NewString())
	require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{taken}))

	claimant := uuid.NewString()
	free, alsoFree := servedBy("service_account", claimant), servedBy("service_account", claimant)
	conflict := servedBy("service_account", claimant)
	conflict.Name = taken.Name

	err := repo.ClaimServedSources(ctx, []*Schedule{free, conflict, alsoFree})
	require.ErrorIs(t, err, ErrSourceOwned)

	for _, name := range []string{free.Name, alsoFree.Name} {
		_, err := repo.GetScheduleByName(ctx, name)
		assert.ErrorIs(t, err, ErrScheduleNotFound, "%s must not be left claimed", name)
	}
}

// A serving plugin ingests through the CLI before it claims its source, so the claim is what links the source to the tables that run catalogued; deleted ones stay out and runs of other pipelines are ignored.
func TestClaimServedSources_LinksTheLastRunsAssets(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	owner := uuid.NewString()
	s := servedBy("service_account", owner)
	s.PluginID = "duckdb"

	asset := func(name string) string {
		mrn := "mrn://table/duckdb/" + name + "-" + uuid.NewString()[:8]
		_, err := repo.db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by) VALUES ($1, $2, $3, 'Table', ARRAY['DuckDB'], 'test')`,
			uuid.NewString(), name, mrn)
		require.NoError(t, err)
		// The database stores the kinded form of whatever legacy MRN a writer sends.
		return strings.Replace(mrn, "mrn://", "mrn://asset/", 1)
	}
	run := func(pipeline string, checkpoints map[string]string) {
		var id string
		require.NoError(t, repo.db.QueryRow(ctx, `INSERT INTO runs (pipeline_name, source_name, run_id, status, completed_at) VALUES ($1, 'duckdb', $2, 'completed', NOW()) RETURNING id`,
			pipeline, uuid.NewString()).Scan(&id))
		for mrn, op := range checkpoints {
			_, err := repo.db.Exec(ctx, `INSERT INTO run_checkpoints (run_id, entity_type, entity_mrn, operation) VALUES ($1, 'asset', $2, $3)`, id, mrn, op)
			require.NoError(t, err)
		}
	}
	linked := func() []string {
		rows, err := repo.db.Query(ctx, `SELECT a.mrn FROM asset_schedules l JOIN assets a ON a.id = l.asset_id WHERE l.schedule_id = $1 ORDER BY a.mrn`, s.ID)
		require.NoError(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var mrn string
			require.NoError(t, rows.Scan(&mrn))
			out = append(out, mrn)
		}
		return out
	}

	orders, customers, gone, elsewhere := asset("orders"), asset("customers"), asset("gone"), asset("elsewhere")
	run(s.Name, map[string]string{orders: "created", customers: "unchanged", gone: StatusDeleted})
	run("another-pipeline", map[string]string{elsewhere: "created"})

	require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{s}))
	assert.ElementsMatch(t, []string{orders, customers}, linked())

	again := servedBy("service_account", owner)
	again.Name, again.PluginID = s.Name, s.PluginID
	require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{again}), "re-enrolment claims and links again without error")
	assert.ElementsMatch(t, []string{orders, customers}, linked())
}

// A served source shows whether its plugin is connected now and when it last ingested, which happens through the CLI rather than as a job Marmot runs.
func TestServedSourceReportsConnectionAndLastRun(t *testing.T) {
	repo, ctx := newPostgresRepo(t)
	owner := uuid.NewString()
	s := servedBy("service_account", owner)
	s.PluginID = "duckdb"
	require.NoError(t, repo.ClaimServedSources(ctx, []*Schedule{s}))

	got, err := repo.GetScheduleByName(ctx, s.Name)
	require.NoError(t, err)
	assert.False(t, got.ServingConnected, "nothing has connected yet")
	assert.Nil(t, got.LastRunAt)

	_, err = repo.db.Exec(ctx, `INSERT INTO runs (pipeline_name, source_name, run_id, status, completed_at) VALUES ($1, 'duckdb', $2, 'completed', NOW())`, s.Name, uuid.NewString())
	require.NoError(t, err)
	session := func(principalID string, heartbeat string) {
		var enrolment string
		require.NoError(t, repo.db.QueryRow(ctx, `
			INSERT INTO plugin_serve_enrolments (principal_type, principal_id, audit_subject, sources, public_key_sha256, credential_type, expires_at)
			VALUES ('service_account', $1, 'sa', $2, '\x00'::bytea, 'api_key', NOW() + INTERVAL '10 minutes') RETURNING id`,
			principalID, []string{s.Name}).Scan(&enrolment))
		_, err := repo.db.Exec(ctx, `INSERT INTO plugin_serve_sessions (enrolment_id, replica_id, replica_address, heartbeat_at) VALUES ($1, $2, '10.0.0.1:8443', NOW() - $3::interval)`,
			enrolment, uuid.NewString(), heartbeat)
		require.NoError(t, err)
	}

	session(uuid.NewString(), "0 seconds")
	got, err = repo.GetScheduleByName(ctx, s.Name)
	require.NoError(t, err)
	assert.False(t, got.ServingConnected, "another principal's session does not count")
	require.NotNil(t, got.LastRunAt)
	require.NotNil(t, got.LastRunStatus)
	assert.Equal(t, JobStatusSucceeded, *got.LastRunStatus)

	session(owner, "5 minutes")
	got, err = repo.GetScheduleByName(ctx, s.Name)
	require.NoError(t, err)
	assert.False(t, got.ServingConnected, "a session gone quiet does not count")

	session(owner, "5 seconds")
	got, err = repo.GetScheduleByName(ctx, s.Name)
	require.NoError(t, err)
	assert.True(t, got.ServingConnected)
}
