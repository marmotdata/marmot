package plugins

import "testing"

// Both routes shipped without middleware and answered anonymously on every
// deployed instance: the plugin list is which systems a tenant has connected,
// and the credential status reports whether the server itself holds AWS
// credentials. Nothing here may go back to being reachable without a session.
func TestRoutesAreAuthenticated(t *testing.T) {
	// Routes only builds the middleware chain, so nil services are enough.
	routes := NewHandler(nil, nil, nil).Routes()
	if len(routes) == 0 {
		t.Fatal("no routes registered")
	}
	for _, route := range routes {
		if len(route.Middleware) == 0 {
			t.Errorf("%s %s has no middleware; every plugins route needs authentication",
				route.Method, route.Path)
		}
	}
}

// The credential status is about the server's own environment rather than the
// tenant's catalogue, so it sits behind the stronger of the two permissions.
func TestCredentialStatusIsNotReadableByViewers(t *testing.T) {
	var list, status int
	for _, route := range NewHandler(nil, nil, nil).Routes() {
		switch route.Path {
		case "/api/v1/plugins":
			list = len(route.Middleware)
		case "/api/v1/plugins/aws/credentials/status":
			status = len(route.Middleware)
		}
	}
	if list < 2 {
		t.Errorf("GET /api/v1/plugins: want auth and a permission check, got %d middleware", list)
	}
	if status < 2 {
		t.Errorf("GET /api/v1/plugins/aws/credentials/status: want auth and a permission check, got %d middleware", status)
	}
}
