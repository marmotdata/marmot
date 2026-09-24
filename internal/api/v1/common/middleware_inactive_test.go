package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
