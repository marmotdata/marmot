package serviceaccount

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

// The list and detail pages read usage straight off the account, so validating a key has to count the call and the account has to report it with its key count, creator and the agents that use it.
func TestServiceAccountUsage(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	svc := NewService(NewPostgresRepository(db))

	var userID string
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('john', 'John Smith') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	sa, err := svc.Create(ctx, CreateInput{Name: "svc-support"}, &userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sa.CreatedByName != "John Smith" || sa.APIKeyCount != 0 || sa.LastUsedAt != nil || len(sa.UsedBy) != 0 {
		t.Fatalf("fresh account = %+v", sa)
	}

	key, err := svc.CreateAPIKey(ctx, sa.ID, "inbox", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.ValidateAPIKey(ctx, key.Key); err != nil {
			t.Fatalf("ValidateAPIKey: %v", err)
		}
	}

	assetID := pgtest.SeedAsset(t, db)
	if _, err := db.Exec(ctx, `INSERT INTO agent_runs (id, agent_id, run_id, started_at, status, service_account_id) VALUES (gen_random_uuid(), $1, 'r1', NOW(), 'succeeded', $2)`, assetID, sa.ID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Get(ctx, sa.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.APIKeyCount != 1 || got.CallsLast7Days != 3 || got.LastUsedAt == nil {
		t.Fatalf("usage = keys %d calls %d last used %v", got.APIKeyCount, got.CallsLast7Days, got.LastUsedAt)
	}
	if len(got.UsedBy) != 1 || got.UsedBy[0].ID != assetID {
		t.Fatalf("used by = %+v", got.UsedBy)
	}

	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 || list[0].CallsLast7Days != 3 {
		t.Fatalf("List = %+v, %v", list, err)
	}

	rotated, err := svc.RotateAPIKey(ctx, sa.ID, key.ID)
	if err != nil {
		t.Fatalf("RotateAPIKey: %v", err)
	}
	if _, err := svc.ValidateAPIKey(ctx, key.Key); err == nil {
		t.Fatal("old secret still authenticates after rotation")
	}
	if _, err := svc.ValidateAPIKey(ctx, rotated.Key); err != nil {
		t.Fatalf("rotated key: %v", err)
	}
	if _, err := svc.RotateAPIKey(ctx, "00000000-0000-0000-0000-000000000000", key.ID); err != ErrKeyNotFound {
		t.Fatalf("rotating through another account: err = %v", err)
	}
}
