package common

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/role"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

const (
	validServiceAccountKey = "sa-valid-key"
	validUserKey           = "user-valid-key"
)

// mockServiceAccountService embeds the interface so only ValidateAPIKey, the one method WithAuth calls, needs implementing.
type mockServiceAccountService struct {
	serviceaccount.Service
}

// Two roles with permissions, so the role and permission projection WithAuth
// performs is asserted rather than only the principal type.
func (m *mockServiceAccountService) ValidateAPIKey(_ context.Context, apiKey string) (*serviceaccount.ServiceAccount, error) {
	if apiKey == validServiceAccountKey {
		return &serviceaccount.ServiceAccount{
			ID:     "sa1",
			Name:   "ci-bot",
			Active: true,
			Roles: []*role.Role{
				{Name: "viewer", Permissions: []role.Permission{{ResourceType: "assets", Action: "view"}}},
				{Name: "ingester", Permissions: []role.Permission{{ResourceType: "ingestion", Action: "view"}}},
			},
		}, nil
	}
	return nil, errors.New("invalid service account key")
}

func serviceAccountUserService() *mockUserService {
	return &mockUserService{
		validateAPIKeyFn: func(_ context.Context, key string) (*user.User, error) {
			if key == validUserKey {
				return &user.User{ID: "user1", Username: "alice", Active: true}, nil
			}
			return nil, user.ErrInvalidAPIKey
		},
	}
}

// serve runs a request through WithAuth and reports the status plus whatever
// principal and user the handler saw.
func serve(t *testing.T, req *http.Request, withSA bool) (int, auth.Principal, *user.User) {
	t.Helper()

	old := globalServiceAccountService
	t.Cleanup(func() { globalServiceAccountService = old })
	if withSA {
		globalServiceAccountService = &mockServiceAccountService{}
	} else {
		globalServiceAccountService = nil
	}

	var principal auth.Principal
	var usr *user.User
	handler := WithAuth(serviceAccountUserService(), &mockAuthService{}, &config.Config{})(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := r.Context().Value(PrincipalContextKey).(auth.Principal); ok {
			principal = p
		}
		if u, ok := r.Context().Value(UserContextKey).(*user.User); ok {
			usr = u
		}
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec.Code, principal, usr
}

func bearerRequest(key string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	return req
}

func apiKeyRequest(key string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	req.Header.Set("X-API-Key", key)
	return req
}

// A service-account key is not a user key, so before the bearer fallback existed
// user.ErrInvalidAPIKey reached the 401 and the two carriers disagreed about the
// same key. Both carriers are asserted here so they cannot drift apart again.
func TestWithAuth_ServiceAccountKeyByCarrier(t *testing.T) {
	carriers := map[string]func(string) *http.Request{
		"Authorization: Bearer": bearerRequest,
		"X-API-Key":             apiKeyRequest,
	}

	for carrier, newRequest := range carriers {
		t.Run(carrier, func(t *testing.T) {
			t.Run("valid service account key is accepted", func(t *testing.T) {
				status, principal, usr := serve(t, newRequest(validServiceAccountKey), true)

				if status != http.StatusOK {
					t.Fatalf("status = %d, want %d", status, http.StatusOK)
				}
				if principal == nil {
					t.Fatal("no principal in context")
				}
				if principal.Type() != auth.PrincipalTypeServiceAccount {
					t.Fatalf("principal type = %q, want %q", principal.Type(), auth.PrincipalTypeServiceAccount)
				}
				if principal.ID() != "sa1" || principal.DisplayName() != "ci-bot" {
					t.Fatalf("principal = %q/%q, want sa1/ci-bot", principal.ID(), principal.DisplayName())
				}
				// RequirePermission authorises a service account off these, so an
				// empty projection would authenticate the key and then deny it.
				if !principal.HasPermission("assets", "view") || !principal.HasPermission("ingestion", "view") {
					t.Fatalf("permissions = %v, want assets:view and ingestion:view", principal.Permissions())
				}
				if principal.HasPermission("assets", "manage") {
					t.Fatal("principal holds a permission its roles do not grant")
				}
				if len(principal.Roles()) != 2 {
					t.Fatalf("roles = %v, want viewer and ingester", principal.Roles())
				}
				// AsUser is nil for a service account, which is what keeps
				// passwordChangeGate from gating a principal that has no password.
				if usr != nil {
					t.Fatalf("user in context = %+v, want none", usr)
				}
			})

			t.Run("unknown service account key is refused", func(t *testing.T) {
				status, _, _ := serve(t, newRequest("sa-unknown-key"), true)
				if status != http.StatusUnauthorized {
					t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
				}
			})

			t.Run("no service registered refuses the key", func(t *testing.T) {
				status, _, _ := serve(t, newRequest(validServiceAccountKey), false)
				if status != http.StatusUnauthorized {
					t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
				}
			})

			t.Run("user key still authenticates as the user", func(t *testing.T) {
				status, principal, usr := serve(t, newRequest(validUserKey), true)

				if status != http.StatusOK {
					t.Fatalf("status = %d, want %d", status, http.StatusOK)
				}
				if principal == nil || principal.Type() != auth.PrincipalTypeUser {
					t.Fatalf("principal = %+v, want a user principal", principal)
				}
				if usr == nil || usr.ID != "user1" {
					t.Fatalf("user in context = %+v, want user1", usr)
				}
			})
		})
	}
}
