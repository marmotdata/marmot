package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/rs/zerolog/log"
)

// globalK8sValidator is set once at server startup for SA token auth.
var globalK8sValidator *K8sTokenValidator

// SetK8sTokenValidator registers the K8s token validator for use by WithAuth.
func SetK8sTokenValidator(v *K8sTokenValidator) {
	globalK8sValidator = v
}

var globalOAuthManager *auth.OAuthManager

// SetOAuthManager registers the OAuthManager for OIDC token exchange in WithAuth.
func SetOAuthManager(m *auth.OAuthManager) {
	globalOAuthManager = m
}

var globalServiceAccountService serviceaccount.Service

// SetServiceAccountService registers the SA service so WithAuth can fall through to SA key validation.
func SetServiceAccountService(svc serviceaccount.Service) {
	globalServiceAccountService = svc
}

// OAuthAuthorizeCompleter completes a pending OAuth authorise flow (PKCE) from the login endpoint.
type OAuthAuthorizeCompleter interface {
	HasPendingAuthorize(r *http.Request) bool
	CompleteAuthorize(w http.ResponseWriter, r *http.Request, userID, username string) (string, error)
}

var globalOAuthAuthorizeCompleter OAuthAuthorizeCompleter

func SetOAuthAuthorizeCompleter(c OAuthAuthorizeCompleter) {
	globalOAuthAuthorizeCompleter = c
}

func GetOAuthAuthorizeCompleter() OAuthAuthorizeCompleter {
	return globalOAuthAuthorizeCompleter
}

// WithAuth middleware handles API key, JWT, and K8s ServiceAccount token authentication.
func WithAuth(userService user.Service, authService auth.Service, cfg *config.Config) func(http.HandlerFunc) http.HandlerFunc {
	resolver := auth.NewResolver(userService)
	k8sValidator := globalK8sValidator
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")

			if apiKey != "" {
				u, err := userService.ValidateAPIKey(r.Context(), apiKey)
				if err == nil {
					ctx := setPrincipalContext(r.Context(), auth.NewUserPrincipal(u))
					next(w, r.WithContext(ctx))
					return
				}

				if errors.Is(err, user.ErrInvalidAPIKey) && globalServiceAccountService != nil {
					sa, saErr := globalServiceAccountService.ValidateAPIKey(r.Context(), apiKey)
					if saErr == nil {
						roleNames := make([]string, 0, len(sa.Roles))
						permKeys := make([]string, 0)
						for _, r := range sa.Roles {
							roleNames = append(roleNames, r.Name)
							for _, p := range r.Permissions {
								permKeys = append(permKeys, p.ResourceType+":"+p.Action)
							}
						}
						principal := auth.NewServiceAccountPrincipal(sa.ID, sa.Name, roleNames, permKeys)
						ctx := setPrincipalContext(r.Context(), principal)
						next(w, r.WithContext(ctx))
						return
					}
				}

				log.Debug().Err(err).
					Str("endpoint", r.URL.Path).
					Str("method", r.Method).
					Msg("Failed to validate API key")
				setWWWAuthenticate(w, cfg)
				RespondError(w, http.StatusUnauthorized, "Invalid API key")
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")

				// Try JWT validation first
				claims, err := authService.ValidateToken(r.Context(), tokenString)
				if err == nil {
					p, err := resolver.Resolve(r.Context(), claims)
					if err != nil {
						setWWWAuthenticate(w, cfg)
						if errors.Is(err, auth.ErrUserInactive) {
							RespondError(w, http.StatusUnauthorized, "User account is inactive")
						} else {
							RespondError(w, http.StatusUnauthorized, "Invalid token")
						}
						return
					}
					ctx := setPrincipalContext(r.Context(), p)
					next(w, r.WithContext(ctx))
					return
				}

				// Try K8s ServiceAccount token validation
				if k8sValidator != nil {
					ns, sa, k8sErr := k8sValidator.Validate(r.Context(), tokenString)
					if k8sErr == nil && sa == cfg.Operator.ServiceAccount && ns == cfg.Operator.Namespace {
						log.Debug().
							Str("namespace", ns).
							Str("service_account", sa).
							Str("endpoint", r.URL.Path).
							Msg("Authenticated via K8s ServiceAccount token")
						ctx := setPrincipalContext(r.Context(), auth.NewOperatorPrincipal())
						ctx = context.WithValue(ctx, UserContextKey, GetOperatorUser())
						next(w, r.WithContext(ctx))
						return
					}
				}

				if oauthMgr := globalOAuthManager; oauthMgr != nil {
					if exchUser := tryOIDCExchange(r.Context(), oauthMgr, tokenString); exchUser != nil {
						if !exchUser.Active {
							setWWWAuthenticate(w, cfg)
							RespondError(w, http.StatusUnauthorized, "User account is inactive")
							return
						}
						log.Debug().
							Str("user_id", exchUser.ID).
							Str("endpoint", r.URL.Path).
							Msg("Authenticated via OIDC token exchange")
						ctx := setPrincipalContext(r.Context(), auth.NewUserPrincipal(exchUser))
						next(w, r.WithContext(ctx))
						return
					}
				}

				// Fall back to API key in Bearer header
				u, err := userService.ValidateAPIKey(r.Context(), tokenString)
				if err != nil {
					log.Error().Err(err).
						Str("endpoint", r.URL.Path).
						Str("method", r.Method).
						Msg("Failed to validate bearer token as JWT or API key")
					setWWWAuthenticate(w, cfg)
					RespondError(w, http.StatusUnauthorized, "Invalid token")
					return
				}
				ctx := setPrincipalContext(r.Context(), auth.NewUserPrincipal(u))
				next(w, r.WithContext(ctx))
				return
			}

			// Check if anonymous auth is enabled
			if cfg.Auth.Anonymous.Enabled {
				anonymousUser := GetAnonymousUser(cfg.Auth.Anonymous.Role)
				ctx := setPrincipalContext(r.Context(), auth.NewUserPrincipal(anonymousUser))
				ctx = WithAnonymousContext(ctx, cfg.Auth.Anonymous.Role)
				log.Trace().
					Str("endpoint", r.URL.Path).
					Str("method", r.Method).
					Str("role", cfg.Auth.Anonymous.Role).
					Msg("Anonymous access granted")
				next(w, r.WithContext(ctx))
				return
			}

			setWWWAuthenticate(w, cfg)
			RespondError(w, http.StatusUnauthorized, "Authentication required")
		}
	}
}

func setPrincipalContext(ctx context.Context, p auth.Principal) context.Context {
	if p == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, PrincipalContextKey, p)
	if u := p.AsUser(); u != nil {
		ctx = context.WithValue(ctx, UserContextKey, u)
	}
	return ctx
}

// RequirePermission middleware checks if the authenticated principal has the required permission.
// It supports both user principals (via UserContextKey) and non-user principals like service
// accounts (via PrincipalContextKey).
func RequirePermission(userService user.Service, resourceType, action string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			usr, userOk := r.Context().Value(UserContextKey).(*user.User)

			if userOk {
				// handle anonymous users
				if usr.Username == "anonymous" {
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

				hasPermission, err := userService.HasPermission(r.Context(), usr.ID, resourceType, action)
				if err != nil {
					RespondError(w, http.StatusInternalServerError, "Failed to check permissions")
					return
				}
				if !hasPermission {
					RespondError(w, http.StatusForbidden, "Permission denied")
					return
				}
				next(w, r)
				return
			}

			// Non-user principal (e.g. service account) — use the Principal interface directly.
			p, principalOk := PrincipalFromContext(r.Context())
			if !principalOk {
				RespondError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			if !p.HasPermission(resourceType, action) {
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

// RequireEncryption middleware blocks requests when encryption is not configured
func RequireEncryption(configured bool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !configured {
				RespondError(w, http.StatusServiceUnavailable,
					"Encryption key not configured. Set MARMOT_SERVER_ENCRYPTION_KEY to enable this feature.")
				return
			}
			next(w, r)
		}
	}
}

// GetAuthenticatedUser returns the current authenticated user
func GetAuthenticatedUser(ctx context.Context) (*user.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*user.User)
	return user, ok
}

func setWWWAuthenticate(w http.ResponseWriter, cfg *config.Config) {
	if cfg.Server.RootURL != "" {
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`,
				cfg.Server.RootURL))
	}
}

// tryOIDCExchange tries each registered OIDC provider: first via JWKS ID token
// verification, then via UserInfo for access tokens.
func tryOIDCExchange(ctx context.Context, oauthMgr *auth.OAuthManager, tokenString string) *user.User {
	if !looksLikeOIDCToken(tokenString) {
		return nil
	}

	for _, provider := range oauthMgr.GetProviders() {
		te, ok := provider.(auth.TokenExchanger)
		if !ok {
			continue
		}
		usr, err := te.ExchangeToken(ctx, tokenString)
		if err != nil {
			log.Debug().Err(err).
				Str("provider", provider.Type()).
				Msg("OIDC ID token exchange failed, trying next")
			continue
		}
		return usr
	}

	issuer := extractIssuer(tokenString)
	for _, provider := range oauthMgr.GetProviders() {
		if ip, ok := provider.(auth.IssuerProvider); ok && issuer != "" {
			if ip.IssuerURL() != issuer {
				continue
			}
		}
		ate, ok := provider.(auth.AccessTokenExchanger)
		if !ok {
			continue
		}
		usr, err := ate.ExchangeAccessToken(ctx, tokenString)
		if err != nil {
			log.Debug().Err(err).
				Str("provider", provider.Type()).
				Msg("OIDC access token exchange via userinfo failed")
			continue
		}
		return usr
	}

	return nil
}

// extractIssuer returns the iss claim from a JWT without verification.
func extractIssuer(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Issuer
}

// looksLikeOIDCToken returns true for JWTs with an iss claim (Marmot's own JWTs have none).
func looksLikeOIDCToken(token string) bool {
	return extractIssuer(token) != ""
}
