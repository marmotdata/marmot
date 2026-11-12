package crypto

import (
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
