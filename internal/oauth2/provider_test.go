package oauth2

import (
	"testing"

	"github.com/ory/fosite"
)

// An authorization code is spent when the PKCE row is consumed, so PKCE has to
// be mandatory. With EnforcePKCE off, a token request carrying no code_verifier
// skips that path and the code would replay until it expired.
func TestProvider_EnforcesPKCE(t *testing.T) {
	p := NewProvider([]byte("test-secret-key-for-pkce-pinned!"), NewMemoryRepository())

	cfg, ok := p.OAuth2Provider.(*fosite.Fosite).Config.(*fosite.Config)
	if !ok {
		t.Fatalf("unexpected config type %T", p.OAuth2Provider)
	}
	if !cfg.EnforcePKCE {
		t.Fatal("EnforcePKCE must stay on: it is what makes an authorization code single use")
	}
	if cfg.EnablePKCEPlainChallengeMethod {
		t.Fatal("the plain PKCE challenge method must stay off")
	}
}
