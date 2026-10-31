package oauth2

import (
	"context"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func exerciseGrants(t *testing.T, repo GrantRepository, userID string) {
	t.Helper()
	ctx := context.Background()

	id1, err := repo.RecordGrant(ctx, userID, Grant{ClientID: "c1", ClientName: "Claude", Scopes: []string{"openid"}})
	if err != nil {
		t.Fatalf("RecordGrant: %v", err)
	}
	// Signing in again with the same client keeps the grant, so tokens from the first sign-in keep working.
	again, err := repo.RecordGrant(ctx, userID, Grant{ClientID: "c1", Scopes: []string{"openid"}})
	if err != nil {
		t.Fatalf("RecordGrant again: %v", err)
	}
	if again != id1 {
		t.Fatalf("re-authorising changed the grant id from %q to %q", id1, again)
	}

	grants, err := repo.ListGrants(ctx, userID)
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	if len(grants) != 1 || grants[0].ClientName != "Claude" || grants[0].ClientID != "c1" {
		t.Fatalf("grants = %+v", grants)
	}

	if ok, _ := repo.GrantValid(ctx, userID, id1); !ok {
		t.Fatal("live grant reported invalid")
	}
	if ok, _ := repo.GrantValid(ctx, "00000000-0000-0000-0000-0000000000ff", id1); ok {
		t.Fatal("grant valid for a different user")
	}

	found, err := repo.RevokeGrant(ctx, userID, "c1")
	if err != nil || !found {
		t.Fatalf("RevokeGrant: found=%v err=%v", found, err)
	}
	if ok, _ := repo.GrantValid(ctx, userID, id1); ok {
		t.Fatal("revoked grant still valid")
	}

	// Allowing the client again must not bring tokens from before the revoke back to life.
	id2, err := repo.RecordGrant(ctx, userID, Grant{ClientID: "c1", Scopes: []string{"openid"}})
	if err != nil {
		t.Fatalf("RecordGrant after revoke: %v", err)
	}
	if id2 == id1 {
		t.Fatal("grant id reused after revoke")
	}
	if ok, _ := repo.GrantValid(ctx, userID, id1); ok {
		t.Fatal("token from before the revoke is valid again")
	}

	if found, _ := repo.RevokeGrant(ctx, userID, "missing"); found {
		t.Fatal("revoking an unknown client reported found")
	}
}

func TestMemoryGrantRepository(t *testing.T) {
	exerciseGrants(t, NewMemoryGrantRepository(), "u1")
}

func TestPostgresGrantRepository(t *testing.T) {
	db := pgtest.TempDB(t)
	var userID string
	if err := db.QueryRow(context.Background(), `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	exerciseGrants(t, NewPostgresGrantRepository(db, 24*time.Hour), userID)
}

// A grant whose newest token has expired is no longer connected, so it drops off the list rather than piling up with every CLI login.
func TestPostgresGrantRepositoryHidesStaleGrants(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	var userID string
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresGrantRepository(db, time.Hour)
	if _, err := repo.RecordGrant(ctx, userID, Grant{ClientID: "old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE oauth_grants SET last_authorised_at = NOW() - INTERVAL '2 hours'`); err != nil {
		t.Fatal(err)
	}
	grants, err := repo.ListGrants(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("stale grant listed: %+v", grants)
	}
	if _, err := repo.RecordGrant(ctx, userID, Grant{ClientID: "new"}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM oauth_grants WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("stale grant not pruned on the next sign-in, %d rows", n)
	}
}
