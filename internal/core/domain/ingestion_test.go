package domain_test

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestIngestedAssetsInheritTheScheduleDomain(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	repo := domain.NewPostgresRepository(pool)
	svc := domain.NewService(repo)
	observer := domain.NewIngestionObserver(repo)

	finance := mustCreate(t, svc, "Finance", nil)
	var schedule string
	if err := pool.QueryRow(ctx, `
		INSERT INTO ingestion_schedules (name, plugin_id, cron_expression)
		VALUES ('finance-warehouse', 'postgresql', '0 * * * *') RETURNING id`).Scan(&schedule); err != nil {
		t.Fatal(err)
	}
	if err := svc.Assign(ctx, domain.KindIngestionSchedule, []string{schedule}, finance.ID); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"pipeline with a domain", domain.WithPipeline(ctx, "finance-warehouse"), finance.ID},
		{"pipeline without a schedule", domain.WithPipeline(ctx, "adhoc-cli-run"), domain.UnassignedID},
		{"no pipeline in context", ctx, domain.UnassignedID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := pgtest.SeedAsset(t, pool)
			observer.OnAssetCreated(tc.ctx, &asset.Asset{ID: id})
			if got, _ := svc.DomainOf(ctx, domain.KindAsset, id); got != tc.want {
				t.Fatalf("domain = %q, want %q", got, tc.want)
			}
		})
	}
}
