// Command plugin-index manages the key the plugin index is signed with. The index itself is built and signed in the registry repo (marmotdata/registry), from its plugins.yaml.
//
//	plugin-index keygen -id marmot-plugins-2026
//	PLUGIN_INDEX_SIGNING_KEY=... plugin-index sign -id marmot-plugins-2026 -in index.json -o index.json.sig
//	plugin-index verify -in index.json -sig index.json.sig
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/marmotdata/marmot/internal/plugin/index"
)

// signingKeyEnv holds the base64 ed25519 private key the index is signed with.
const signingKeyEnv = "PLUGIN_INDEX_SIGNING_KEY"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: plugin-index keygen|sign|verify [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	case "sign":
		err = sign(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "plugin-index:", err)
		os.Exit(1)
	}
}

func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	id := fs.String("id", "", "key id, e.g. marmot-plugins-2026")
	_ = fs.Parse(args)
	if *id == "" {
		return fmt.Errorf("-id is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Printf("Add to internal/plugin/index/trusted_keys.json:\n  {\"key_id\": %q, \"public_key\": %q}\n\n", *id, base64.StdEncoding.EncodeToString(pub))
	fmt.Printf("Store as the %s secret (keep it nowhere else):\n%s\n", signingKeyEnv, base64.StdEncoding.EncodeToString(priv))
	return nil
}

func sign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	id := fs.String("id", "", "id of the signing key, as listed in trusted_keys.json")
	in := fs.String("in", "index.json", "index to sign")
	out := fs.String("o", "index.json.sig", "signature output file")
	_ = fs.Parse(args)
	if *id == "" {
		return fmt.Errorf("-id is required")
	}

	raw, err := base64.StdEncoding.DecodeString(os.Getenv(signingKeyEnv))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return fmt.Errorf("%s must hold a base64 ed25519 private key", signingKeyEnv)
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sig, err := index.Sign(data, *id, ed25519.PrivateKey(raw))
	if err != nil {
		return err
	}

	// Check the signature against the keys this tree trusts, so a key mix-up fails here rather than on every Marmot.
	keys, err := index.TrustedKeys()
	if err != nil {
		return err
	}
	if _, err := index.Verify(data, sig, keys); err != nil {
		return fmt.Errorf("signed index does not verify against trusted_keys.json: %w", err)
	}
	return os.WriteFile(*out, sig, 0o600) //nolint:gosec // G703: the path is the operator's own -o flag
}

// verify checks an index and its signature the way Marmot does, against the keys this tree trusts or a given public key.
func verify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	in := fs.String("in", "index.json", "index to verify")
	sigPath := fs.String("sig", "index.json.sig", "detached signature")
	pub := fs.String("pub", "", "base64 public key to verify against instead of trusted_keys.json, with -id")
	id := fs.String("id", "", "key id for -pub")
	_ = fs.Parse(args)

	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(*sigPath)
	if err != nil {
		return err
	}
	keys, err := index.TrustedKeys()
	if err != nil {
		return err
	}
	if *pub != "" {
		key, err := base64.StdEncoding.DecodeString(*pub)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return fmt.Errorf("-pub must be a base64 ed25519 public key")
		}
		keys = []index.PublicKey{{ID: *id, Key: key}}
	}
	idx, err := index.Verify(data, sig, keys)
	if err != nil {
		return err
	}
	advisories := 0
	for _, p := range idx.Plugins {
		advisories += len(p.Advisories)
	}
	fmt.Printf("index %d verified: %d plugins, %d advisories, expires %s\n", idx.Version, len(idx.Plugins), advisories, idx.Expires.Format(time.RFC3339))
	return nil
}
