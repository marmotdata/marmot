package user

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Only UpdateUser and the Get pair are reachable from these paths, so the rest
// of Repository is embedded rather than stubbed out method by method.
type stubSessionsRepo struct {
	Repository
	updates            map[string]interface{}
	mustChangePassword bool
	password           string
}

func (s *stubSessionsRepo) UpdateUser(_ context.Context, _ string, updates map[string]interface{}) error {
	s.updates = updates
	return nil
}

func (s *stubSessionsRepo) GetUser(_ context.Context, id string) (*User, error) {
	return &User{ID: id, Username: "alice", Active: true, MustChangePassword: s.mustChangePassword}, nil
}

func (s *stubSessionsRepo) ValidatePassword(_ context.Context, _ string, password string) error {
	if password != s.password {
		return ErrInvalidPassword
	}
	return nil
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
	repo := &stubSessionsRepo{password: "old-password"}
	if _, err := NewService(repo).UpdatePassword(context.Background(), "u1", "old-password", "brand-new-password"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	if _, ok := repo.updates["sessions_invalidated_at"].(time.Time); !ok {
		t.Fatalf("updates = %v, want sessions_invalidated_at set", repo.updates)
	}
	if _, ok := repo.updates["password_hash"]; !ok {
		t.Fatal("password_hash missing from the same update")
	}
}

// A voluntary change has to prove the current password, so a stolen session cannot lock the owner out.
func TestUpdatePassword_RequiresCurrentPassword(t *testing.T) {
	for _, tc := range []struct {
		name    string
		current string
	}{
		{name: "missing", current: ""},
		{name: "wrong", current: "not-the-password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubSessionsRepo{password: "old-password"}
			_, err := NewService(repo).UpdatePassword(context.Background(), "u1", tc.current, "brand-new-password")

			var invalid *InvalidInputError
			if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "current_password" {
				t.Fatalf("err = %v, want a current_password field error", err)
			}
			if repo.updates != nil {
				t.Fatalf("updates = %v, want nothing written", repo.updates)
			}
		})
	}
}

// The forced first change follows a sign-in with the password just checked, so it does not ask for it again.
func TestUpdatePassword_ForcedChangeSkipsCurrentPassword(t *testing.T) {
	repo := &stubSessionsRepo{mustChangePassword: true, password: "admin"}
	if _, err := NewService(repo).UpdatePassword(context.Background(), "u1", "", "brand-new-password"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	if repo.updates["must_change_password"] != false {
		t.Fatalf("updates = %v, want must_change_password cleared", repo.updates)
	}
}

// A password set by an administrator is temporary, so the member has to choose their own and old sessions end.
func TestUpdate_PasswordResetForcesChange(t *testing.T) {
	repo := &stubSessionsRepo{}
	password := "temporary-password"
	if _, err := NewService(repo).Update(context.Background(), "u1", UpdateUserInput{Password: &password}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updates["must_change_password"] != true {
		t.Fatalf("updates = %v, want must_change_password set", repo.updates)
	}
	if _, ok := repo.updates["sessions_invalidated_at"].(time.Time); !ok {
		t.Fatalf("updates = %v, want sessions_invalidated_at set", repo.updates)
	}
}
