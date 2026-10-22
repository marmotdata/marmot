package runs

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres"
)

// newPostgresRepo migrates the database named by MARMOT_TEST_POSTGRES_DSN and returns a repository over it. Without that variable the test is skipped, so `go test ./...` stays offline.
func newPostgresRepo(t *testing.T) (*SchedulePostgresRepository, context.Context) {
	t.Helper()
	dsn := os.Getenv("MARMOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MARMOT_TEST_POSTGRES_DSN to run the Postgres persistence tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, postgres.NewSetup(pool).Initialize(ctx))
	return &SchedulePostgresRepository{db: pool}, ctx
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
