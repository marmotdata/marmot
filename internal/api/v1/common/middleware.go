package common

import (
	"context"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/services/auth"
	"github.com/marmotdata/marmot/internal/services/user"
	"github.com/rs/zerolog/log"
)

// WithAuth middleware handles both API key and JWT authentication
func WithAuth(userService user.Service, authService auth.Service) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				user, err := userService.ValidateAPIKey(r.Context(), apiKey)
				if err != nil {
					log.Error().Err(err).
						Str("endpoint", r.URL.Path).
						Str("method", r.Method).
						Msg("Failed to validate API key")
					RespondError(w, http.StatusUnauthorized, "Invalid API key")
					return
				}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next(w, r.WithContext(ctx))
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				RespondError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := authService.ValidateToken(r.Context(), tokenString)
			if err != nil {
				RespondError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			user, err := userService.Get(r.Context(), claims.Subject)
			if err != nil {
				RespondError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			if !user.Active {
				RespondError(w, http.StatusUnauthorized, "User account is inactive")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next(w, r.WithContext(ctx))
		}
	}
}

func RequirePermission(userService user.Service, resourceType, action string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserContextKey).(*user.User)
			if !ok {
				RespondError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			hasPermission, err := userService.HasPermission(r.Context(), user.ID, resourceType, action)
			if err != nil {
				RespondError(w, http.StatusInternalServerError, "Failed to check permissions")
				return
			}

			if !hasPermission {
				RespondError(w, http.StatusForbidden, "Permission denied")
				return
			}

			next(w, r)
		}
	}
}

func GetAuthenticatedUser(ctx context.Context) (*user.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*user.User)
	return user, ok
}
