package auth

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type fakeRevocations struct {
	keys map[string]time.Time
	err  error
}

func (f *fakeRevocations) Revoke(_ context.Context, key []byte, until time.Time) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.keys[string(key)]; !ok {
		f.keys[string(key)] = until
	}
	return nil
}

func (f *fakeRevocations) Revoked(_ context.Context, key []byte) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.keys[string(key)]
	return ok, nil
}

// A token copied before sign-out must stop working although it has not expired, without taking the person's other sessions with it, and a failed lookup must refuse rather than accept.
func TestRevokeToken(t *testing.T) {
	ctx := context.Background()
	store := &fakeRevocations{keys: map[string]time.Time{}}
	svc := NewService(&mockRepo{}, nil, WithRevocationStore(store))
	u := &user.User{ID: "u1", Username: "jane"}

	token, err := svc.GenerateToken(ctx, u, nil)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	other, err := svc.GenerateToken(ctx, u, nil)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	claims, err := svc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatalf("ValidateToken before sign-out: %v", err)
	}

	if err := svc.RevokeToken(ctx, token); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if _, err := svc.ValidateToken(ctx, token); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("ValidateToken after sign-out: err = %v, want ErrTokenRevoked", err)
	}
	if _, err := svc.ValidateToken(ctx, other); err != nil {
		t.Fatalf("another token for the same person was refused: %v", err)
	}
	if err := svc.RevokeToken(ctx, token); err != nil {
		t.Fatalf("revoking twice: %v", err)
	}
	if len(store.keys) != 1 {
		t.Fatalf("stored %d keys, want 1", len(store.keys))
	}
	for _, until := range store.keys {
		if !until.Equal(claims.ExpiresAt.Time) {
			t.Fatalf("retained until %v, want the token's exp %v", until, claims.ExpiresAt.Time)
		}
	}

	if err := svc.RevokeToken(ctx, "not-a-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("RevokeToken of a made-up string: err = %v, want ErrInvalidToken", err)
	}
	if len(store.keys) != 1 {
		t.Fatal("a token that never verified was stored")
	}

	store.err = errors.New("database down")
	if _, err := svc.ValidateToken(ctx, other); !errors.Is(err, ErrTokenCheck) {
		t.Fatalf("ValidateToken while the store fails: err = %v, want ErrTokenCheck", err)
	}
	if err := svc.RevokeToken(ctx, other); err == nil || errors.Is(err, ErrInvalidToken) {
		t.Fatalf("RevokeToken while the store fails: err = %v, want a store error", err)
	}
}

// The store keys by jti, so tokens that share every other claim are still told apart, and a token without one falls back to the whole token.
func TestRevocationKey(t *testing.T) {
	a := revocationKey(&Claims{}, "token-a")
	b := revocationKey(&Claims{}, "token-b")
	if bytes.Equal(a, b) {
		t.Fatal("tokens without a jti share a key")
	}
	c := &Claims{}
	c.ID = "jti-1"
	if !bytes.Equal(revocationKey(c, "token-a"), revocationKey(c, "token-b")) {
		t.Fatal("the key does not follow the jti")
	}
	if len(a) != 32 {
		t.Fatalf("key is %d bytes, want a sha256", len(a))
	}
}

func TestPostgresRevocationStore(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	store := NewPostgresRevocationStore(db)

	live := []byte("live-key-live-key-live-key-live!")
	stale := []byte("stale-key-stale-key-stale-key-st")
	if err := store.Revoke(ctx, stale, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("Revoke stale: %v", err)
	}
	if revoked, err := store.Revoked(ctx, live); err != nil || revoked {
		t.Fatalf("Revoked before revoke = %v, %v", revoked, err)
	}
	if err := store.Revoke(ctx, live, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := store.Revoke(ctx, live, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("revoking twice: %v", err)
	}
	if revoked, err := store.Revoked(ctx, live); err != nil || !revoked {
		t.Fatalf("Revoked after revoke = %v, %v", revoked, err)
	}

	var rows int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM revoked_tokens`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("%d rows, want 1: the insert prunes rows past their retention", rows)
	}

	// The whole service against the real table.
	svc := NewService(&mockRepo{}, nil, WithRevocationStore(store))
	token, err := svc.GenerateToken(ctx, &user.User{ID: "u1", Username: "jane"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeToken(ctx, token); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if _, err := svc.ValidateToken(ctx, token); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("ValidateToken after sign-out: err = %v", err)
	}

	db.Close()
	if _, err := svc.ValidateToken(ctx, token); !errors.Is(err, ErrTokenCheck) {
		t.Fatalf("ValidateToken with the database gone: err = %v, want ErrTokenCheck", err)
	}
}
