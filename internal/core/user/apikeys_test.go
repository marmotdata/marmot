package user

import (
	"context"
	"errors"
	"testing"
)

// ValidateAPIKey reaches only these four methods, so the rest of Repository is
// embedded rather than stubbed out method by method.
type stubAPIKeyRepo struct {
	Repository
	user        *User
	lastUsedFor []string
}

func (s *stubAPIKeyRepo) GetAPIKeyByHash(_ context.Context, key string) (*APIKey, error) {
	if key != "known-key" {
		return nil, ErrUserNotFound
	}
	return &APIKey{ID: "key1", UserID: "user1"}, nil
}

func (s *stubAPIKeyRepo) GetUser(_ context.Context, _ string) (*User, error) {
	return s.user, nil
}

func (s *stubAPIKeyRepo) GetUserIdentities(_ context.Context, _ string) ([]*UserIdentity, error) {
	return nil, nil
}

func (s *stubAPIKeyRepo) UpdateAPIKeyLastUsed(_ context.Context, id string) error {
	s.lastUsedFor = append(s.lastUsedFor, id)
	return nil
}

// Deactivating a user leaves their keys in place, so the key has to be refused
// at validation or it outlives the account it belongs to.
func TestValidateAPIKey_InactiveUser(t *testing.T) {
	tests := []struct {
		name         string
		user         *User
		key          string
		wantErr      error
		wantUser     bool
		wantLastUsed bool
	}{
		{
			name:         "active user authenticates and the key counts as used",
			user:         &User{ID: "user1", Username: "alice", Active: true},
			key:          "known-key",
			wantUser:     true,
			wantLastUsed: true,
		},
		{
			name:    "deactivated user is refused",
			user:    &User{ID: "user1", Username: "alice", Active: false},
			key:     "known-key",
			wantErr: ErrUserInactive,
		},
		{
			name:    "unknown key is still an invalid key, not an inactive user",
			user:    &User{ID: "user1", Username: "alice", Active: true},
			key:     "no-such-key",
			wantErr: ErrInvalidAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubAPIKeyRepo{user: tt.user}
			got, err := NewService(repo).ValidateAPIKey(context.Background(), tt.key)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("user = %+v, want nil on refusal", got)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantUser && (got == nil || got.ID != "user1") {
				t.Fatalf("user = %+v, want user1", got)
			}

			// A refused key must not be credited with a use, or the UI reports a
			// key as recently used when nothing it was presented for succeeded.
			if gotLastUsed := len(repo.lastUsedFor) > 0; gotLastUsed != tt.wantLastUsed {
				t.Fatalf("last_used updated = %v, want %v", gotLastUsed, tt.wantLastUsed)
			}
		})
	}
}
