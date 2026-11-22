package common

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/marmotdata/marmot/pkg/config"
)

// PublicRootURL is the address clients reach Marmot at, for links that must be absolute, such as OAuth discovery. The configured root URL always wins; without one it comes from the request, through a reverse proxy's forwarded headers when present, so a server with no root URL still hands out usable addresses instead of bare paths. The second result reports whether it was derived, in which case the response must not be cached.
func PublicRootURL(r *http.Request, cfg *config.Config) (string, bool) {
	if cfg != nil && cfg.Server.RootURL != "" {
		return strings.TrimRight(cfg.Server.RootURL, "/"), false
	}
	if r == nil {
		return "", false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := strings.ToLower(firstHeaderValue(r, "X-Forwarded-Proto")); p == "http" || p == "https" {
		scheme = p
	}
	host := firstHeaderValue(r, "X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	// Only a bare host and port is accepted, so a crafted header can't smuggle a path, credentials or another scheme into the address.
	u, err := url.Parse(scheme + "://" + host)
	if host == "" || err != nil || u.Host != host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return scheme + "://" + host, true
}

// firstHeaderValue returns the first entry of a header a chain of proxies may have appended to.
func firstHeaderValue(r *http.Request, name string) string {
	v, _, _ := strings.Cut(r.Header.Get(name), ",")
	return strings.TrimSpace(v)
}
