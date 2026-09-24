package glossary

import (
	"context"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type noopRecorder struct{ metrics.Recorder }

func (noopRecorder) RecordDBQuery(context.Context, string, time.Duration, bool) {}

func TestImportIsAllOrNothing(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	var userID string
	if err := pool.QueryRow(ctx, "INSERT INTO users (username, name) VALUES ('ana', 'Ana') RETURNING id").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewPostgresRepository(pool, noopRecorder{}))
	owners := []OwnerInput{{ID: userID, Type: "user"}}

	_, err := svc.Import(ctx, []ImportTerm{
		{Create: CreateTermInput{Name: "Invoice", Definition: "A bill", Owners: owners}},
		{Create: CreateTermInput{Name: "Receipt", Definition: "Proof", Owners: owners}, ParentName: "Invoice"},
		{Create: CreateTermInput{Name: "Credit note", Definition: "", Owners: owners}},
	})
	if err == nil {
		t.Fatal("an invalid last row must fail the import")
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM glossary_terms").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("%d terms left behind by a failed import", count)
	}

	written, err := svc.Import(ctx, []ImportTerm{
		{Create: CreateTermInput{Name: "Receipt", Definition: "Proof", Owners: owners}, ParentName: "Invoice"},
		{Create: CreateTermInput{Name: "Invoice", Definition: "A bill", Owners: owners}},
	})
	if err != nil || len(written) != 2 {
		t.Fatalf("written = %v, err = %v", written, err)
	}
	byName, err := svc.ByNames(ctx, []string{"INVOICE"})
	if err != nil || len(byName["Invoice"]) != 1 {
		t.Fatalf("ByNames ignores case: %v, %v", byName, err)
	}
	receipt, err := svc.GetByName(ctx, "Receipt")
	if err != nil || receipt.ParentTermID == nil || len(receipt.Owners) != 1 {
		t.Fatalf("receipt = %+v, err = %v", receipt, err)
	}
}
