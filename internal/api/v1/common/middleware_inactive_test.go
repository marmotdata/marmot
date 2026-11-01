package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

// Deactivating a user is a routine admin action, and their JWT stops working
// when it expires. Their API key has to stop at the same moment rather than
// keeping full access, and both carriers have to agree on that.
func TestWithAuth_InactiveUserAPIKey(t *testing.T) {
	carriers := map[string]func(string) *http.Request{
		"Authorization: Bearer": bearerRequest,
		"X-API-Key":             apiKeyRequest,
	}

	for carrier, newRequest := range carriers {
		t.Run(carrier, func(t *testing.T) {
			userSvc := &mockUserService{
				validateAPIKeyFn: func(_ context.Context, _ string) (*user.User, error) {
					return nil, user.ErrUserInactive
				},
			}

			reached := false
			handler := WithAuth(userSvc, &mockAuthService{}, &config.Config{})(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			handler(rec, newRequest("deactivated-users-key"))

			if reached {
				t.Fatal("handler ran for a deactivated user")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}

			// The JWT path already answers this way, so a client cannot tell the
			// two apart and retry the other carrier expecting a different result.
			var body struct {
				Error string `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response: %v", err)
			}
			if body.Error != "User account is inactive" {
				t.Fatalf("error = %q, want %q", body.Error, "User account is inactive")
			}
		})
	}
}

// An inactive user's key is a user key, so it must not be retried as a
// service-account key on the way to the refusal.
func TestWithAuth_InactiveUserIsNotRetriedAsServiceAccount(t *testing.T) {
	old := globalServiceAccountService
	t.Cleanup(func() { globalServiceAccountService = old })
	globalServiceAccountService = &mockServiceAccountService{}

	userSvc := &mockUserService{
		validateAPIKeyFn: func(_ context.Context, _ string) (*user.User, error) {
			return nil, user.ErrUserInactive
		},
	}
	handler := WithAuth(userSvc, &mockAuthService{}, &config.Config{})(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// validServiceAccountKey is what the mock service accepts, so if the inactive
	// refusal fell through to it the request would wrongly succeed as the bot.
	rec := httptest.NewRecorder()
	handler(rec, bearerRequest(validServiceAccountKey))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// revokingAuthService answers ValidateToken with a fixed error, standing in for a signed-out token or a failed sign-out lookup.
type revokingAuthService struct {
	mockAuthService
	err error
}

func (m *revokingAuthService) ValidateToken(_ context.Context, _ string) (*auth.Claims, error) {
	return nil, m.err
}

// A signed-out token is refused without being retried as an API key, and a sign-out lookup that fails is a 500 rather than a 401 or a pass.
func TestWithAuth_SignedOutToken(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{auth.ErrTokenRevoked, http.StatusUnauthorized},
		{fmt.Errorf("%w: database down", auth.ErrTokenCheck), http.StatusInternalServerError},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			triedAPIKey := false
			userSvc := &mockUserService{
				validateAPIKeyFn: func(_ context.Context, _ string) (*user.User, error) {
					triedAPIKey = true
					return &user.User{ID: "u1", Active: true}, nil
				},
			}
			reached := false
			handler := WithAuth(userSvc, &revokingAuthService{err: tc.err}, &config.Config{})(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
			})
			rec := httptest.NewRecorder()
			handler(rec, bearerRequest("signed-out-jwt"))
			if reached || triedAPIKey {
				t.Fatalf("reached handler = %v, tried as API key = %v", reached, triedAPIKey)
			}
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}
