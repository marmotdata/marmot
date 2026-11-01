package index

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// trustedKeysJSON holds the public keys the plugin index may be signed with. Two can be listed while the signing key is rotated.
//
//go:embed trusted_keys.json
var trustedKeysJSON []byte

// PublicKey is a key the index may be signed with.
type PublicKey struct {
	ID  string            `json:"key_id"`
	Key ed25519.PublicKey `json:"public_key"`
}

// Signature is the detached signature published beside the index, over the index's exact bytes.
type Signature struct {
	KeyID     string `json:"key_id"`
	Signature []byte `json:"signature"`
}

// ErrUntrusted means the index was not signed by a trusted key.
var ErrUntrusted = errors.New("plugin index signature is not from a trusted key")

// TrustedKeys returns the keys built into this binary.
func TrustedKeys() ([]PublicKey, error) {
	var raw []struct {
		ID  string `json:"key_id"`
		Key string `json:"public_key"`
	}
	if err := json.Unmarshal(trustedKeysJSON, &raw); err != nil {
		return nil, fmt.Errorf("parsing trusted plugin index keys: %w", err)
	}
	keys := make([]PublicKey, 0, len(raw))
	for _, k := range raw {
		key, err := base64.StdEncoding.DecodeString(k.Key)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("trusted plugin index key %s is not an ed25519 public key", k.ID)
		}
		keys = append(keys, PublicKey{ID: k.ID, Key: key})
	}
	return keys, nil
}

// Verify checks the signature over data against keys and only then parses the index.
func Verify(data, signature []byte, keys []PublicKey) (*Index, error) {
	var sig Signature
	if err := json.Unmarshal(signature, &sig); err != nil {
		return nil, fmt.Errorf("parsing plugin index signature: %w", err)
	}
	trusted := false
	for _, k := range keys {
		if k.ID == sig.KeyID && ed25519.Verify(k.Key, data, sig.Signature) {
			trusted = true
			break
		}
	}
	if !trusted {
		return nil, ErrUntrusted
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing plugin index: %w", err)
	}
	if idx.Version <= 0 {
		return nil, fmt.Errorf("plugin index has no version")
	}
	if idx.Expires.IsZero() {
		return nil, fmt.Errorf("plugin index has no expiry")
	}
	return &idx, nil
}

// Sign produces the detached signature for an index, for the tool that publishes it.
func Sign(data []byte, keyID string, key ed25519.PrivateKey) ([]byte, error) {
	return json.Marshal(Signature{KeyID: keyID, Signature: ed25519.Sign(key, data)})
}
