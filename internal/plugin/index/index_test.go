package index

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func testPlugin() Plugin {
	return Plugin{
		Versions: []Version{
			{Version: "0.3.0", Digest: "sha256:a"},
			{Version: "0.3.1", Digest: "sha256:b"},
			{Version: "0.3.2", Digest: "sha256:c", Yanked: true},
			{Version: "0.4.0", Digest: "sha256:d", MarmotVersion: ">=0.20.0"},
			{Version: "0.3.3", Digest: "sha256:e"},
		},
		Advisories: []Advisory{{ID: "MPA-1", Severity: "high", Affected: "<0.3.1", Fixed: "0.3.1"}},
	}
}

func TestInstallableSkipsYankedAndIncompatible(t *testing.T) {
	got := testPlugin().Installable("0.15.0")
	want := []string{"0.3.3", "0.3.1", "0.3.0"}
	if len(got) != len(want) {
		t.Fatalf("Installable = %v, want %v", got, want)
	}
	for i, v := range got {
		if v.Version != want[i] {
			t.Errorf("Installable[%d] = %s, want %s", i, v.Version, want[i])
		}
	}
}

func TestUpgradeChoices(t *testing.T) {
	p := testPlugin()
	if a := p.Affecting("0.3.0"); len(a) != 1 {
		t.Errorf("Affecting(0.3.0) = %v, want MPA-1", a)
	}
	if a := p.Affecting("0.3.1"); len(a) != 0 {
		t.Errorf("Affecting(0.3.1) = %v, want none", a)
	}
	if v, ok := p.SecurityFix("0.3.0", "0.15.0"); !ok || v.Version != "0.3.1" {
		t.Errorf("SecurityFix = %v %v, want 0.3.1", v.Version, ok)
	}
	if v, ok := p.LatestPatch("0.3.0", "0.15.0"); !ok || v.Version != "0.3.3" {
		t.Errorf("LatestPatch = %v %v, want 0.3.3", v.Version, ok)
	}
	if v, ok := p.Latest("0.15.0"); !ok || v.Version != "0.3.3" {
		t.Errorf("Latest on 0.15 = %v %v, want 0.3.3", v.Version, ok)
	}
	if v, ok := p.Latest("0.20.0"); !ok || v.Version != "0.4.0" {
		t.Errorf("Latest on 0.20 = %v %v, want 0.4.0", v.Version, ok)
	}
}

func TestVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	keys := []PublicKey{{ID: "k1", Key: pub}}
	data := []byte(`{"version": 7, "expires": "2999-01-01T00:00:00Z", "plugins": {}}`)

	sig, err := Sign(data, "k1", priv)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Verify(data, sig, keys)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if idx.Version != 7 {
		t.Errorf("Version = %d, want 7", idx.Version)
	}

	tampered := []byte(`{"version": 8, "expires": "2999-01-01T00:00:00Z", "plugins": {}}`)
	if _, err := Verify(tampered, sig, keys); !errors.Is(err, ErrUntrusted) {
		t.Errorf("tampered index: err = %v, want ErrUntrusted", err)
	}

	_, other, _ := ed25519.GenerateKey(rand.Reader)
	foreign, _ := Sign(data, "k1", other)
	if _, err := Verify(data, foreign, keys); !errors.Is(err, ErrUntrusted) {
		t.Errorf("foreign key: err = %v, want ErrUntrusted", err)
	}
}

func TestTrustedKeysParse(t *testing.T) {
	keys, err := TrustedKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("no trusted keys built in")
	}
}

func TestVerifyRefusesIndexWithoutExpiry(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	data := []byte(`{"version": 7, "plugins": {}}`)
	sig, _ := Sign(data, "k1", priv)
	if _, err := Verify(data, sig, []PublicKey{{ID: "k1", Key: pub}}); err == nil {
		t.Error("an index without an expiry was accepted")
	}
}
