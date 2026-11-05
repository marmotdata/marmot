package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContentSecurityPolicyAllowsNoUnsafeScripts(t *testing.T) {
	csp := contentSecurityPolicy([]string{"'sha256-abc='", "'sha256-def='"})
	scriptSrc := ""
	for _, d := range strings.Split(csp, ";") {
		if d = strings.TrimSpace(d); strings.HasPrefix(d, "script-src ") {
			scriptSrc = d
		}
	}
	if scriptSrc != "script-src 'self' 'sha256-abc=' 'sha256-def='" {
		t.Fatalf("script-src = %q", scriptSrc)
	}
	for _, unsafe := range []string{"'unsafe-eval'", "'unsafe-hashes'", "*", "data:", "blob:"} {
		if strings.Contains(scriptSrc, unsafe) {
			t.Fatalf("script-src must not contain %s: %s", unsafe, scriptSrc)
		}
	}
	for _, want := range []string{"object-src 'none'", "base-uri 'self'", "worker-src 'self' blob:", "img-src 'self' data: blob:", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("policy is missing %q: %s", want, csp)
		}
	}
}

func TestSecurityHeadersSendTheStrictPolicy(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if csp := rec.Header().Get("Content-Security-Policy"); strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("unsafe script sources in %s", csp)
	}
}
