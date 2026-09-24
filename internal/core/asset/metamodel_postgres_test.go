package asset

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type dbRecorder struct{ metrics.Recorder }

func (dbRecorder) RecordDBQuery(context.Context, string, time.Duration, bool) {}

func TestMetamodelPostgresWrites(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(pool, dbRecorder{})
	svc := NewService(repo, WithMetamodel(mustLoadProfile(t)))
	input := validCreate("postgres-metamodel")
	if err := pool.QueryRow(ctx, "SELECT id FROM users WHERE username = 'admin'").Scan(&input.CreatedBy); err != nil {
		t.Fatal(err)
	}
	input.Metadata["example"] = map[string]any{"retention": 30.0, "unknown": "keep"}
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.PatchFields(ctx, created.ID, created.Version, map[string]any{"retention": 90.0})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 2 || updated.Version != 2 || *stored.MRN != *created.MRN {
		t.Fatalf("persisted version/identity: %+v", stored)
	}
	if value, _ := metamodel.ValueAt(stored.Metadata, "metadata.example.unknown"); value != "keep" {
		t.Fatal("unknown metadata lost")
	}
	if _, err := svc.PatchFields(ctx, created.ID, 2, map[string]any{"retention": -1}); err == nil {
		t.Fatal("invalid value persisted")
	}
	if _, err := svc.PatchFields(ctx, created.ID, 1, map[string]any{"retention": 1}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale patch: %v", err)
	}

	// Both writers read version 2 before either issues its SQL UPDATE.
	other, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, a := range []*Asset{stored, other} {
		go func() { <-start; results <- repo.Update(ctx, a) }()
	}
	close(start)
	var successes, conflicts int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrVersionConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
	stored, err = repo.Get(ctx, created.ID)
	if err != nil || stored.Version != 3 {
		t.Fatalf("final version: %+v %v", stored, err)
	}
}
