package crypto

import (
	"crypto/sha256"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// HasAPIKeyScheme reports whether key carries a scheme prefix such as mrmt_u_ or mrmt_s_.
//
// The prefixes arrived after key_lookup, so a prefixed key always has a lookup
// value: one that misses the indexed lookup is invalid, and comparing it against
// every legacy key would let anyone spend a bcrypt per legacy key with a made-up one.
func HasAPIKeyScheme(key string) bool {
	return strings.HasPrefix(key, "mrmt_")
}

const maxVerifiedAPIKeys = 10_000

var (
	verifiedMu      sync.Mutex
	verifiedAPIKeys = make(map[[sha256.Size]byte]struct{})
)

// CompareAPIKey is bcrypt.CompareHashAndPassword for API keys, remembering pairs that already verified.
//
// API keys are presented on every request, and a bcrypt comparison costs tens of
// milliseconds of CPU. A successful comparison of the same key against the same
// stored hash cannot change, so it is remembered: the caller still loads the row
// on every request, which keeps deletion, expiry and rotation (a new hash) in
// effect immediately. Only a digest of hash and key is kept, never either one, and
// failures are not remembered so invalid keys cannot fill the set.
func CompareAPIKey(hash, key string) error {
	h := sha256.New()
	h.Write([]byte(hash))
	h.Write([]byte{0})
	h.Write([]byte(key))
	var id [sha256.Size]byte
	h.Sum(id[:0])

	verifiedMu.Lock()
	_, ok := verifiedAPIKeys[id]
	verifiedMu.Unlock()
	if ok {
		return nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)); err != nil {
		return err
	}

	verifiedMu.Lock()
	// Only holders of a valid key add entries, so the cap is reached only with more
	// live keys than this. Dropping one arbitrary entry (map order is randomised)
	// costs its holder one bcrypt, where clearing the set would cost every holder one.
	if len(verifiedAPIKeys) >= maxVerifiedAPIKeys {
		for evict := range verifiedAPIKeys {
			delete(verifiedAPIKeys, evict)
			break
		}
	}
	verifiedAPIKeys[id] = struct{}{}
	verifiedMu.Unlock()
	return nil
}
