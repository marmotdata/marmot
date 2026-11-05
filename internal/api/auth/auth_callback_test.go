package auth

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/staticfiles"
	"github.com/marmotdata/marmot/pkg/config"
)

func TestAuthCallbackPageScriptMatchesItsPolicyHash(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.RootURL = `https://marmot.example/"><script>alert(1)</script>`
	h := &Handler{config: cfg}
	rec := httptest.NewRecorder()
	h.handleAuthCallback(rec, httptest.NewRequest("GET", "/auth/callback", nil))

	body := rec.Body.String()
	hashes := staticfiles.HashInlineScripts([]byte(body))
	if len(hashes) != 1 {
		t.Fatalf("expected exactly one inline script, got %d: %s", len(hashes), body)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src "+hashes[0]+";") {
		t.Fatalf("served script hash %s is not the one the policy allows: %s", hashes[0], csp)
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("policy must not allow unsafe sources: %s", csp)
	}
	// The root URL lands in an attribute, escaped, never as markup or script.
	if strings.Contains(body, "<script>alert(1)") || !strings.Contains(body, `data-root="https://marmot.example/&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;"`) {
		t.Fatalf("root URL was not escaped into data-root: %s", body)
	}
}
