package runs

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

// A plugin routinely emits an edge to an asset another source owns, and the
// server rejects an edge whose other end is not in the catalog yet. That is
// normal, not a defect: the two sources are ingested in whatever order the
// operator runs them. What matters is that the edge appears once both ends
// exist, without having to rename the pipeline.

type recorderStub struct{}

func (recorderStub) RecordSearchQuery(context.Context, string, string)               {}
func (recorderStub) RecordAssetView(context.Context, string, string, string, string) {}
func (recorderStub) RecordDBQuery(context.Context, string, time.Duration, bool)      {}

func (recorderStub) WrapDBQuery(_ context.Context, _ string, fn func() error) error {
	return fn()
}

func (recorderStub) RecordCustomMetrics(context.Context, []metrics.Metric) error { return nil }

func newTestService(t *testing.T, pool *pgxpool.Pool) (Service, lineage.Service) {
	t.Helper()

	recorder := recorderStub{}
	assetService := asset.NewService(asset.NewPostgresRepository(pool, recorder))
	lineageService := lineage.NewService(lineage.NewPostgresRepository(pool), assetService)

	return NewService(NewPostgresRepository(pool), assetService, lineageService, nil, recorder), lineageService
}

func tableInput(name string) CreateAssetInput {
	return CreateAssetInput{
		Name:      name,
		Type:      "Table",
		Providers: []string{"PostgreSQL"},
	}
}

// process runs one ingest of the given pipeline the way the CLI does: start a
// run, hand it what the plugin found, then complete it. Completing matters,
// because only a completed run becomes the checkpoint baseline the next run
// reads.
func process(t *testing.T, service Service, pipeline string, assets []CreateAssetInput, edges []LineageInput) *ProcessAssetsResponse {
	t.Helper()
	ctx := context.Background()

	run, err := service.StartRun(ctx, pipeline, "postgresql", "tester", plugin.RawPluginConfig{})
	require.NoError(t, err)

	response, err := service.ProcessEntities(ctx, run.RunID, assets, edges, nil, nil, nil, pipeline, "postgresql")
	require.NoError(t, err)

	require.NoError(t, service.CompleteRun(ctx, run.RunID, plugin.StatusCompleted, &plugin.RunSummary{}, ""))

	return response
}

func TestProcessEntities_WritesAnEdgeWhoseOtherEndArrivedAfterTheFirstRun(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, lineageService := newTestService(t, pool)

	edge := LineageInput{
		Source: "mrn://table/postgresql/customers",
		Target: "mrn://table/postgresql/orders",
		Type:   "FOREIGN_KEY",
	}

	// First run: only one end of the edge exists, so the edge is rejected.
	first := process(t, service, "orders-pipeline", []CreateAssetInput{tableInput("orders")}, []LineageInput{edge})
	require.Len(t, first.Lineage, 1)
	require.Equal(t, StatusFailed, first.Lineage[0].Status)

	exists, err := lineageService.EdgeExists(ctx, edge.Source, edge.Target)
	require.NoError(t, err)
	require.False(t, exists, "an edge to an asset that does not exist cannot be written")

	// Second run of the same pipeline, now that the other end is there.
	second := process(t, service, "orders-pipeline",
		[]CreateAssetInput{tableInput("orders"), tableInput("customers")}, []LineageInput{edge})
	require.Len(t, second.Lineage, 1)

	exists, err = lineageService.EdgeExists(ctx, edge.Source, edge.Target)
	require.NoError(t, err)
	assert.True(t, exists, "the edge has to be written once both of its ends exist")
}

func TestProcessEntities_WritesAnEdgeAgainAfterItIsDeletedOutOfBand(t *testing.T) {
	// The checkpoint records what a run did, not what the catalog holds. An
	// edge removed by hand, or lost with a restore, has to come back on the
	// next run of the pipeline that declared it.
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, lineageService := newTestService(t, pool)

	assets := []CreateAssetInput{tableInput("orders"), tableInput("customers")}
	edge := LineageInput{
		Source: "mrn://table/postgresql/customers",
		Target: "mrn://table/postgresql/orders",
		Type:   "FOREIGN_KEY",
	}

	process(t, service, "orders-pipeline", assets, []LineageInput{edge})

	edgeID, err := lineageService.CreateDirectLineage(ctx, edge.Source, edge.Target, edge.Type, "")
	require.NoError(t, err)
	require.NoError(t, lineageService.DeleteDirectLineage(ctx, edgeID))

	process(t, service, "orders-pipeline", assets, []LineageInput{edge})

	exists, err := lineageService.EdgeExists(ctx, edge.Source, edge.Target)
	require.NoError(t, err)
	assert.True(t, exists, "the run declares the edge, so it has to restore it")
}

func TestProcessEntities_ReportsAKnownEdgeAsUpdated(t *testing.T) {
	// Re-writing an edge every run must not make every run look like it
	// discovered something new.
	pool := pgtest.TempDB(t)
	service, _ := newTestService(t, pool)

	assets := []CreateAssetInput{tableInput("orders"), tableInput("customers")}
	edge := LineageInput{
		Source: "mrn://table/postgresql/customers",
		Target: "mrn://table/postgresql/orders",
		Type:   "FOREIGN_KEY",
	}

	first := process(t, service, "orders-pipeline", assets, []LineageInput{edge})
	require.Len(t, first.Lineage, 1)
	assert.Equal(t, StatusCreated, first.Lineage[0].Status)

	second := process(t, service, "orders-pipeline", assets, []LineageInput{edge})
	require.Len(t, second.Lineage, 1)
	assert.Equal(t, StatusUpdated, second.Lineage[0].Status)
}
