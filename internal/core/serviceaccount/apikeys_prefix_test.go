package serviceaccount

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

func (s *stubCreateKeyRepo) CountAPIKeys(_ context.Context, _ string) (int, error) { return 0, nil }

func (s *stubCreateKeyRepo) CreateAPIKey(_ context.Context, _ string, apiKey *APIKey, keyHash string) error {
	s.created = apiKey
	s.keyHash = keyHash
	return nil
}

// Service-account keys get their own prefix so a leaked key can be told apart
// from a user key at a glance. See the user package's twin test for why old
// unprefixed keys keep validating.
func TestCreateAPIKey_PrefixedAndRoundTrips(t *testing.T) {
	repo := &stubCreateKeyRepo{}
	created, err := NewService(repo).CreateAPIKey(context.Background(), "sa1", "ci", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	if !strings.HasPrefix(created.Key, APIKeyPrefix) {
		t.Fatalf("key %q does not start with %q", created.Key, APIKeyPrefix)
	}
	if len(created.Key) != len(APIKeyPrefix)+44 {
		t.Fatalf("key length %d, want prefix plus 44 base64 chars", len(created.Key))
	}
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
