package common

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/pkg/config"
)

func TestPublicRootURL(t *testing.T) {
	configured := &config.Config{}
	configured.Server.RootURL = "https://marmot.example.com/"

	tests := []struct {
		name        string
		cfg         *config.Config
		host        string
		headers     map[string]string
		tls         bool
		want        string
		wantDerived bool
	}{
		{name: "configured wins", cfg: configured, host: "localhost:8080", headers: map[string]string{"X-Forwarded-Host": "other.example"}, want: "https://marmot.example.com"},
		{name: "request host", cfg: &config.Config{}, host: "localhost:8080", want: "http://localhost:8080", wantDerived: true},
		{name: "tls", cfg: &config.Config{}, host: "marmot.internal", tls: true, want: "https://marmot.internal", wantDerived: true},
		{name: "forwarded by a proxy", cfg: &config.Config{}, host: "10.0.0.5:8080", headers: map[string]string{"X-Forwarded-Host": "marmot.example.com, proxy.internal", "X-Forwarded-Proto": "https"}, want: "https://marmot.example.com", wantDerived: true},
		{name: "odd proto ignored", cfg: &config.Config{}, host: "localhost:8080", headers: map[string]string{"X-Forwarded-Proto": "javascript"}, want: "http://localhost:8080", wantDerived: true},
		{name: "path in host refused", cfg: &config.Config{}, host: "localhost:8080", headers: map[string]string{"X-Forwarded-Host": "evil.example/x"}, want: ""},
		{name: "credentials in host refused", cfg: &config.Config{}, host: "localhost:8080", headers: map[string]string{"X-Forwarded-Host": "user@evil.example"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
			r.Host = tt.host
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			got, derived := PublicRootURL(r, tt.cfg)
			if got != tt.want || derived != tt.wantDerived {
				t.Fatalf("PublicRootURL = %q, %v; want %q, %v", got, derived, tt.want, tt.wantDerived)
			}
		})
	}
}
