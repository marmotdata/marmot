package user

import (
	"context"
	"testing"
	"time"
)

// Only UpdateUser and the Get pair are reachable from these paths, so the rest
// of Repository is embedded rather than stubbed out method by method.
type stubSessionsRepo struct {
	Repository
	updates map[string]interface{}
}

func (s *stubSessionsRepo) UpdateUser(_ context.Context, _ string, updates map[string]interface{}) error {
	s.updates = updates
	return nil
}

func (s *stubSessionsRepo) GetUser(_ context.Context, id string) (*User, error) {
	return &User{ID: id, Username: "alice", Active: true}, nil
}

func (s *stubSessionsRepo) GetUserIdentities(_ context.Context, _ string) ([]*UserIdentity, error) {
	return nil, nil
}

func TestInvalidateSessions_SetsCutoff(t *testing.T) {
	repo := &stubSessionsRepo{}
	if err := NewService(repo).InvalidateSessions(context.Background(), "u1"); err != nil {
		t.Fatalf("InvalidateSessions: %v", err)
	}

	cutoff, ok := repo.updates["sessions_invalidated_at"].(time.Time)
	if !ok {
		t.Fatalf("updates = %v, want a sessions_invalidated_at time", repo.updates)
	}
	if time.Since(cutoff) > time.Minute {
		t.Fatalf("cutoff %v is not recent", cutoff)
	}
}

// Changing a password usually means the old one is no longer trusted, so the
// change signs out every existing session as well.
func TestUpdatePassword_InvalidatesSessions(t *testing.T) {
	repo := &stubSessionsRepo{}
	if _, err := NewService(repo).UpdatePassword(context.Background(), "u1", "brand-new-password"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	if _, ok := repo.updates["sessions_invalidated_at"].(time.Time); !ok {
		t.Fatalf("updates = %v, want sessions_invalidated_at set", repo.updates)
	}
	if _, ok := repo.updates["password_hash"]; !ok {
		t.Fatal("password_hash missing from the same update")
	}
}
