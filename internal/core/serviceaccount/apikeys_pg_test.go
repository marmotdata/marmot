package serviceaccount

import (
	"context"
	"errors"
	"testing"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"golang.org/x/crypto/bcrypt"
)

// Keys issued before key_lookup existed have no scheme prefix and are found by scanning. A prefixed key always has a lookup value, so it must never be searched for among them.
func TestGetAPIKeyByHash_LegacyScanOnlyForUnprefixedKeys(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	var saID string
	if err := db.QueryRow(ctx, `INSERT INTO service_accounts (name) VALUES ('svc-legacy') RETURNING id`).Scan(&saID); err != nil {
		t.Fatal(err)
	}
	insertLegacy := func(name, key string) {
		t.Helper()
		hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO service_account_api_keys (service_account_id, name, key_hash) VALUES ($1, $2, $3)`, saID, name, string(hash)); err != nil {
			t.Fatal(err)
		}
	}
	// The second row cannot exist in practice; it is how the test sees that the scan did not run.
	insertLegacy("legacy", "bGVnYWN5LWtleQ==")
	insertLegacy("impossible", APIKeyPrefix+"without-lookup")

	if found, err := repo.GetAPIKeyByHash(ctx, "bGVnYWN5LWtleQ=="); err != nil || found.Name != "legacy" {
		t.Fatalf("legacy key: found=%v err=%v", found, err)
	}
	if _, err := repo.GetAPIKeyByHash(ctx, APIKeyPrefix+"without-lookup"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("prefixed key was compared against legacy keys: err = %v", err)
	}
}
