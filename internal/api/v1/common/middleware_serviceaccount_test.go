package common

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/role"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type mockServiceAccountService struct{}

func (m *mockServiceAccountService) Create(_ context.Context, _ serviceaccount.CreateInput, _ *string) (*serviceaccount.ServiceAccount, error) {
	return nil, nil
}

func (m *mockServiceAccountService) Get(_ context.Context, _ string) (*serviceaccount.ServiceAccount, error) {
	return nil, nil
}

func (m *mockServiceAccountService) List(_ context.Context) ([]*serviceaccount.ServiceAccount, error) {
	return nil, nil
}

func (m *mockServiceAccountService) Update(_ context.Context, _ string, _ serviceaccount.UpdateInput) (*serviceaccount.ServiceAccount, error) {
	return nil, nil
}

func (m *mockServiceAccountService) Delete(_ context.Context, _ string) error { return nil }

func (m *mockServiceAccountService) CreateAPIKey(_ context.Context, _ string, _ string, _ *time.Duration) (*serviceaccount.APIKey, error) {
	return nil, nil
}

func (m *mockServiceAccountService) ListAPIKeys(_ context.Context, _ string) ([]*serviceaccount.APIKey, error) {
	return nil, nil
}

func (m *mockServiceAccountService) DeleteAPIKey(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockServiceAccountService) ValidateAPIKey(_ context.Context, apiKey string) (*serviceaccount.ServiceAccount, error) {
	if apiKey == "sa-valid-key" {
		return &serviceaccount.ServiceAccount{
			ID:     "sa1",
			Name:   "ci-bot",
			Active: true,
			Roles:  []*role.Role{{Name: "viewer"}},
		}, nil
	}
	return nil, errors.New("invalid service account key")
}

func TestWithAuth_BearerServiceAccountKey(t *testing.T) {
	old := globalServiceAccountService
	defer func() { globalServiceAccountService = old }()

	userSvc := &mockUserService{
		validateAPIKeyFn: func(_ context.Context, key string) (*user.User, error) {
			if key == "user-valid-key" {
				return &user.User{ID: "user1", Username: "alice", Active: true}, nil
			}
			return nil, user.ErrInvalidAPIKey
		},
	}
	authSvc := &mockAuthService{}
	cfg := &config.Config{}

	tests := []struct {
		name       string
		bearerKey  string
		withSA     bool
		wantStatus int
	}{
		{name: "service account key on bearer path", bearerKey: "sa-valid-key", withSA: true, wantStatus: http.StatusOK},
		{name: "service account key with no service registered", bearerKey: "sa-valid-key", withSA: false, wantStatus: http.StatusUnauthorized},
		{name: "unknown service account key", bearerKey: "sa-unknown-key", withSA: true, wantStatus: http.StatusUnauthorized},
		{name: "valid user key on bearer path", bearerKey: "user-valid-key", withSA: true, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.withSA {
				globalServiceAccountService = &mockServiceAccountService{}
			} else {
				globalServiceAccountService = nil
			}

			var captured auth.Principal
			handler := WithAuth(userSvc, authSvc, cfg)(func(w http.ResponseWriter, r *http.Request) {
				if p, ok := r.Context().Value(PrincipalContextKey).(auth.Principal); ok {
					captured = p
				}
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
			req.Header.Set("Authorization", "Bearer "+tt.bearerKey)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}

			if tt.wantStatus == http.StatusOK {
				if captured == nil {
					t.Fatal("expected principal in context, got nil")
				}
				switch tt.bearerKey {
				case "sa-valid-key":
					if captured.Type() != auth.PrincipalTypeServiceAccount {
						t.Fatalf("expected principal type %q, got %q", auth.PrincipalTypeServiceAccount, captured.Type())
					}
				case "user-valid-key":
					if captured.Type() != auth.PrincipalTypeUser {
						t.Fatalf("expected principal type %q, got %q", auth.PrincipalTypeUser, captured.Type())
					}
				}
			}
		})
	}
}
