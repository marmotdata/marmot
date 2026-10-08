package user

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type stubCreateKeyRepo struct {
	Repository
	created *APIKey
	keyHash string
}

func (s *stubCreateKeyRepo) CreateAPIKey(_ context.Context, apiKey *APIKey, keyHash string) error {
	s.created = apiKey
	s.keyHash = keyHash
	return nil
}

// New keys carry a recognisable prefix so secret scanners can match them and a
// key in a support ticket can be identified. The prefix is part of the string
// that gets hashed, so the key handed to the caller is exactly the key that
// validates; unprefixed keys issued before this keep working because
// validation only ever hashes whatever string is presented.
func TestCreateAPIKey_PrefixedAndRoundTrips(t *testing.T) {
	repo := &stubCreateKeyRepo{}
	created, err := NewService(repo).CreateAPIKey(context.Background(), "u1", "ci", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	if !strings.HasPrefix(created.Key, UserAPIKeyPrefix) {
		t.Fatalf("key %q does not start with %q", created.Key, UserAPIKeyPrefix)
	}
	if len(created.Key) != len(UserAPIKeyPrefix)+44 {
		t.Fatalf("key length %d, want prefix plus 44 base64 chars", len(created.Key))
	}
	// bcrypt only reads 72 bytes, so the whole prefixed key must stay under that
	// or two keys sharing the first 72 bytes would verify as each other.
	if len(created.Key) > 72 {
		t.Fatalf("key length %d exceeds bcrypt's 72 byte limit", len(created.Key))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.keyHash), []byte(created.Key)); err != nil {
		t.Fatal("stored hash does not verify the full prefixed key")
	}
	if repo.created.Key != created.Key {
		t.Fatal("repository saw a different key than the caller was handed")
	}
}
