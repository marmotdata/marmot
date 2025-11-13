package common

import (
	"context"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

// WithAuth middleware handles both API key and JWT authentication
func WithAuth(userService user.Service, authService auth.Service, cfg *config.Config) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				user, err := userService.ValidateAPIKey(r.Context(), apiKey)
				if err != nil {
					log.Debug().Err(err).
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
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")

				claims, err := authService.ValidateToken(r.Context(), tokenString)
				if err == nil {
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
					return
				}

				user, err := userService.ValidateAPIKey(r.Context(), tokenString)
				if err != nil {
					log.Error().Err(err).
						Str("endpoint", r.URL.Path).
						Str("method", r.Method).
						Msg("Failed to validate bearer token as JWT or API key")
					RespondError(w, http.StatusUnauthorized, "Invalid token")
					return
				}

				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next(w, r.WithContext(ctx))
				return
			}

			// Check if anonymous auth is enabled
			if cfg.Auth.Anonymous.Enabled {
				anonymousUser := GetAnonymousUser(cfg.Auth.Anonymous.Role)

				ctx := context.WithValue(r.Context(), UserContextKey, anonymousUser)
				ctx = WithAnonymousContext(ctx, cfg.Auth.Anonymous.Role)

				log.Trace().
					Str("endpoint", r.URL.Path).
					Str("method", r.Method).
					Str("role", cfg.Auth.Anonymous.Role).
					Msg("Anonymous access granted")
				next(w, r.WithContext(ctx))
				return
			}

			RespondError(w, http.StatusUnauthorized, "Authentication required")
		}
	}
}

// RequirePermission middleware checks if the user has required permissions
func RequirePermission(userService user.Service, resourceType, action string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserContextKey).(*user.User)
			if !ok {
				RespondError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// handle anonymous users
			if user.Username == "anonymous" {
				anonymousCtx, ok := GetAnonymousContext(r.Context())
				if ok {
					hasRolePermission, err := checkAnonymousPermission(userService, anonymousCtx.RoleName, resourceType, action)
					if err != nil {
						RespondError(w, http.StatusInternalServerError, "Failed to check permissions")
						return
					}

					if !hasRolePermission {
						RespondError(w, http.StatusForbidden, "Permission denied")
						return
					}

					next(w, r)
					return
				}
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

// checkAnonymousPermission verifies if the anonymous role has the required permission
func checkAnonymousPermission(userService user.Service, roleName, resourceType, action string) (bool, error) {
	permissions, err := userService.GetPermissionsByRoleName(context.Background(), roleName)
	if err != nil {
		return false, err
	}

	for _, perm := range permissions {
		if perm.ResourceType == resourceType && perm.Action == action {
			return true, nil
		}
	}

	return false, nil
}

// GetAuthenticatedUser returns the current authenticated user
func GetAuthenticatedUser(ctx context.Context) (*user.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*user.User)
	return user, ok
}
