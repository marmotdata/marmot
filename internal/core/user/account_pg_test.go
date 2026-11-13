package user

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"golang.org/x/crypto/bcrypt"
)

// Rotating keeps the key's identity but not its secret: the old secret must stop at once, the new one must work, and the stored prefix must follow the new secret.
func TestRotateAPIKey(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)
	svc := NewService(repo)

	var userID, otherID string
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('john', 'John') RETURNING id`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}

	created, err := svc.CreateAPIKey(ctx, userID, "ci", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if !strings.HasPrefix(created.Key, created.Prefix) || len(created.Prefix) != len(UserAPIKeyPrefix)+visibleKeyChars {
		t.Fatalf("prefix %q does not start key %q", created.Prefix, created.Key)
	}

	// Verify once first so the old secret is remembered, and rotation must still end it.
	if _, err := repo.GetAPIKeyByHash(ctx, created.Key); err != nil {
		t.Fatalf("new key: %v", err)
	}

	if _, err := svc.RotateAPIKey(ctx, otherID, created.ID); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("rotating someone else's key: err = %v", err)
	}

	rotated, err := svc.RotateAPIKey(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("RotateAPIKey: %v", err)
	}
	if rotated.ID != created.ID || rotated.Name != "ci" || rotated.Key == created.Key {
		t.Fatalf("rotated = %+v", rotated)
	}
	if _, err := repo.GetAPIKeyByHash(ctx, created.Key); err == nil {
		t.Fatal("old secret still authenticates")
	}
	if found, err := repo.GetAPIKeyByHash(ctx, rotated.Key); err != nil || found.ID != created.ID {
		t.Fatalf("new secret: found=%v err=%v", found, err)
	}

	keys, err := svc.ListAPIKeys(ctx, userID)
	if err != nil || len(keys) != 1 {
		t.Fatalf("ListAPIKeys: %v %v", keys, err)
	}
	if keys[0].Prefix != rotated.Prefix || keys[0].Key != "" {
		t.Fatalf("listed key %+v leaks the secret or lost the prefix", keys[0])
	}
}

func TestProfileJobTitleAndTeams(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	svc := NewService(NewPostgresRepository(db))

	var userID, teamID string
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO teams (name) VALUES ('Analytics Engineering') RETURNING id`).Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, source, sso_provider) VALUES ($1, $2, 'member', 'sso', 'okta')`, teamID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sso_team_mappings (provider, sso_group_name, team_id, member_role) VALUES ('okta', 'analytics', $1, 'member')`, teamID); err != nil {
		t.Fatal(err)
	}

	if err := svc.UpdateJobTitle(ctx, userID, "  Staff analytics engineer "); err != nil {
		t.Fatalf("UpdateJobTitle: %v", err)
	}
	if err := svc.UpdateJobTitle(ctx, userID, strings.Repeat("x", MaxJobTitleLength+1)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("over-long title: err = %v", err)
	}

	profile, err := svc.GetProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.JobTitle != "Staff analytics engineer" {
		t.Fatalf("job title %q", profile.JobTitle)
	}
	if len(profile.Teams) != 1 || profile.Teams[0].Name != "Analytics Engineering" || profile.Teams[0].SSOGroup != "analytics" {
		t.Fatalf("teams = %+v", profile.Teams)
	}

	if err := svc.UpdateJobTitle(ctx, userID, ""); err != nil {
		t.Fatalf("clearing job title: %v", err)
	}
	profile, _ = svc.GetProfile(ctx, userID)
	if profile.JobTitle != "" {
		t.Fatalf("job title not cleared: %q", profile.JobTitle)
	}
}

// Keys issued before key_lookup existed have no scheme prefix and are found by scanning. A prefixed key always has a lookup value, so it must never be searched for among them.
func TestGetAPIKeyByHash_LegacyScanOnlyForUnprefixedKeys(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	var userID string
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('jane', 'Jane') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	insertLegacy := func(name, key string) {
		t.Helper()
		hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO api_keys (user_id, name, key_hash) VALUES ($1, $2, $3)`, userID, name, string(hash)); err != nil {
			t.Fatal(err)
		}
	}
	// The second row cannot exist in practice; it is how the test sees that the scan did not run.
	insertLegacy("legacy", "bGVnYWN5LWtleQ==")
	insertLegacy("impossible", UserAPIKeyPrefix+"without-lookup")

	if found, err := repo.GetAPIKeyByHash(ctx, "bGVnYWN5LWtleQ=="); err != nil || found.Name != "legacy" {
		t.Fatalf("legacy key: found=%v err=%v", found, err)
	}
	if _, err := repo.GetAPIKeyByHash(ctx, UserAPIKeyPrefix+"without-lookup"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("prefixed key was compared against legacy keys: err = %v", err)
	}
}
