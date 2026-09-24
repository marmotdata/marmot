package domain_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

// ingest records assets as written by one scheduled run of the schedule, the
// way the scheduler and the runs service leave them.
func ingest(t *testing.T, pool *pgxpool.Pool, scheduleID string, assetIDs ...string) {
	t.Helper()
	ctx := context.Background()
	var runID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO runs (pipeline_name, source_name, run_id, status, created_by)
		SELECT name, 'postgresql', gen_random_uuid()::text, 'completed', 'test' FROM ingestion_schedules WHERE id = $1
		RETURNING id`, scheduleID).Scan(&runID); err != nil {
		t.Fatalf("inserting run: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ingestion_job_runs (schedule_id, status, plugin_run_id) VALUES ($1, 'succeeded', $2)`,
		scheduleID, runID); err != nil {
		t.Fatalf("inserting job run: %v", err)
	}
	for _, id := range assetIDs {
		if _, err := pool.Exec(ctx, `
			INSERT INTO run_checkpoints (run_id, entity_type, entity_mrn, operation)
			SELECT $1, 'asset', mrn, 'created' FROM assets WHERE id = $2`, runID, id); err != nil {
			t.Fatalf("inserting checkpoint: %v", err)
		}
	}
}

func TestPipelineDomainChange(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	svc := domain.NewService(domain.NewPostgresRepository(pool))
	finance := mustCreate(t, svc, "Finance", nil)
	legal := mustCreate(t, svc, "Legal", nil)
	risk := mustCreate(t, svc, "Risk", nil)

	var schedule string
	if err := pool.QueryRow(ctx, `
		INSERT INTO ingestion_schedules (name, plugin_id, cron_expression)
		VALUES ('warehouse', 'postgresql', '0 * * * *') RETURNING id`).Scan(&schedule); err != nil {
		t.Fatal(err)
	}
	stays, pinned, other := pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool)
	ingest(t, pool, schedule, stays, pinned)
	if err := svc.Assign(ctx, domain.KindAsset, []string{stays, pinned, other}, finance.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AssignPipeline(ctx, schedule, finance.ID, false); err != nil {
		t.Fatal(err)
	}
	// A steward moved one of the pipeline's assets elsewhere.
	if err := svc.Assign(ctx, domain.KindAsset, []string{pinned}, risk.ID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.PipelineAssignment(ctx, schedule)
	if err != nil || got.DomainID != finance.ID || got.AssetsInDomain != 1 {
		t.Fatalf("assignment = %+v, %v; want Finance with 1 asset (not the pinned one, not another pipeline's)", got, err)
	}

	t.Run("without moving assets only the pipeline changes", func(t *testing.T) {
		res, err := svc.AssignPipeline(ctx, schedule, legal.ID, false)
		if err != nil || res.MovedAssets != 0 {
			t.Fatalf("result = %+v, %v", res, err)
		}
		if d, _ := svc.DomainOf(ctx, domain.KindAsset, stays); d != finance.ID {
			t.Fatalf("asset moved without being asked: %s", d)
		}
		if _, err := svc.AssignPipeline(ctx, schedule, finance.ID, false); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("moving assets takes only those still in the old domain", func(t *testing.T) {
		res, err := svc.AssignPipeline(ctx, schedule, legal.ID, true)
		if err != nil || res.MovedAssets != 1 {
			t.Fatalf("result = %+v, %v", res, err)
		}
		for id, want := range map[string]string{stays: legal.ID, pinned: risk.ID, other: finance.ID} {
			if d, _ := svc.DomainOf(ctx, domain.KindAsset, id); d != want {
				t.Fatalf("asset %s in %s, want %s", id, d, want)
			}
		}
		if d, _ := svc.DomainOf(ctx, domain.KindIngestionSchedule, schedule); d != legal.ID {
			t.Fatalf("pipeline in %s", d)
		}
	})

	t.Run("moving to Unassigned clears the memberships", func(t *testing.T) {
		res, err := svc.AssignPipeline(ctx, schedule, domain.UnassignedID, true)
		if err != nil || res.MovedAssets != 1 {
			t.Fatalf("result = %+v, %v", res, err)
		}
		if d, _ := svc.DomainOf(ctx, domain.KindAsset, stays); d != domain.UnassignedID {
			t.Fatalf("asset in %s", d)
		}
	})

	t.Run("unknown pipeline or domain", func(t *testing.T) {
		_, err := svc.PipelineAssignment(ctx, "00000000-0000-4000-8000-00000000ffff")
		wantErr(t, err, domain.ErrEntityNotFound)
		_, err = svc.AssignPipeline(ctx, "00000000-0000-4000-8000-00000000ffff", legal.ID, false)
		wantErr(t, err, domain.ErrEntityNotFound)
		_, err = svc.AssignPipeline(ctx, schedule, "00000000-0000-4000-8000-00000000ffff", false)
		wantErr(t, err, domain.ErrNotFound)
	})
}
