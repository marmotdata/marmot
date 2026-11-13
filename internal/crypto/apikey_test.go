package crypto

import (
	"crypto/sha256"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func hashKey(t testing.TB, key string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestCompareAPIKey(t *testing.T) {
	key := "mrmt_u_test-key"
	hash := hashKey(t, key)

	for i := range 2 {
		if err := CompareAPIKey(hash, key); err != nil {
			t.Fatalf("attempt %d: valid key refused: %v", i, err)
		}
	}

	if err := CompareAPIKey(hash, key+"x"); err == nil {
		t.Fatal("wrong key accepted against a remembered hash")
	}

	// Rotation stores a new hash; the old key must not ride on the remembered pair.
	if err := CompareAPIKey(hashKey(t, "mrmt_u_rotated"), key); err == nil {
		t.Fatal("old key accepted against the rotated hash")
	}
}

func TestCompareAPIKey_FailuresAreNotRemembered(t *testing.T) {
	hash := hashKey(t, "mrmt_u_other")
	verifiedMu.Lock()
	before := len(verifiedAPIKeys)
	verifiedMu.Unlock()

	_ = CompareAPIKey(hash, "mrmt_u_wrong")

	verifiedMu.Lock()
	defer verifiedMu.Unlock()
	if len(verifiedAPIKeys) != before {
		t.Fatal("a failed comparison was remembered")
	}
}

func TestCompareAPIKey_Concurrent(t *testing.T) {
	key := "mrmt_s_concurrent"
	hash := hashKey(t, key)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if err := CompareAPIKey(hash, key); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func BenchmarkCompareAPIKey(b *testing.B) {
	key := "mrmt_u_bench"
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("bcrypt", func(b *testing.B) {
		for b.Loop() {
			_ = bcrypt.CompareHashAndPassword(hash, []byte(key))
		}
	})
	b.Run("remembered", func(b *testing.B) {
		_ = CompareAPIKey(string(hash), key)
		for b.Loop() {
			_ = CompareAPIKey(string(hash), key)
		}
	})
}

func TestHasAPIKeyScheme(t *testing.T) {
	for key, want := range map[string]bool{
		"mrmt_u_abc":       true,
		"mrmt_s_abc":       true,
		"bGVnYWN5LWtleQ==": false,
		"":                 false,
	} {
		if got := HasAPIKeyScheme(key); got != want {
			t.Errorf("HasAPIKeyScheme(%q) = %v, want %v", key, got, want)
		}
	}
}

// At the cap one entry makes room for the new one, so the other remembered keys stay remembered.
func TestCompareAPIKey_EvictsOneAtCap(t *testing.T) {
	verifiedMu.Lock()
	saved := verifiedAPIKeys
	verifiedAPIKeys = make(map[[sha256.Size]byte]struct{}, maxVerifiedAPIKeys)
	for i := range maxVerifiedAPIKeys {
		verifiedAPIKeys[sha256.Sum256([]byte{byte(i), byte(i >> 8)})] = struct{}{}
	}
	verifiedMu.Unlock()
	t.Cleanup(func() {
		verifiedMu.Lock()
		verifiedAPIKeys = saved
		verifiedMu.Unlock()
	})

	if err := CompareAPIKey(hashKey(t, "mrmt_u_at-cap"), "mrmt_u_at-cap"); err != nil {
		t.Fatal(err)
	}

	verifiedMu.Lock()
	defer verifiedMu.Unlock()
	if n := len(verifiedAPIKeys); n != maxVerifiedAPIKeys {
		t.Fatalf("remembered %d keys at the cap, want %d", n, maxVerifiedAPIKeys)
	}
}
