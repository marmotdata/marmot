package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/user"
)

const JWTSigningKeyID = "jwt_signing_key"

// TokenTTL is how long an access token stays valid. The OAuth token endpoint
// reports it as expires_in.
const TokenTTL = 24 * time.Hour

type Claims struct {
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
	// Omitted for user principals; consumers must treat "" as PrincipalTypeUser.
	PrincipalType string `json:"principal_type,omitempty"`
	// ClientID and GrantID are set on tokens issued to an OAuth client, so the person can revoke that client's access from their account.
	ClientID string `json:"client_id,omitempty"`
	GrantID  string `json:"grant_id,omitempty"`
	jwt.RegisteredClaims
}

// IssuedTime returns the token's iat, or the zero time when it has none so that it fails any revocation check
func (c *Claims) IssuedTime() time.Time {
	if c.IssuedAt == nil {
		return time.Time{}
	}
	return c.IssuedAt.Time
}

type Service interface {
	GenerateToken(ctx context.Context, user *user.User, preferencesClaims map[string]interface{}) (string, error)
	GenerateTokenForPrincipal(ctx context.Context, p Principal, preferencesClaims map[string]interface{}) (string, error)
	// GenerateTokenForClient issues a user token on behalf of an OAuth client, tied to the grant the person gave it so revoking the grant ends the token.
	GenerateTokenForClient(ctx context.Context, u *user.User, clientID, grantID string) (string, error)
	ValidateToken(ctx context.Context, tokenString string) (*Claims, error)
	// RevokeToken signs one token out before it expires, leaving the person's other sessions alone. A token that does not verify returns ErrInvalidToken and is not recorded.
	RevokeToken(ctx context.Context, tokenString string) error
	GetSigningKey(ctx context.Context) ([]byte, error)
}

// GrantChecker answers whether an OAuth grant a token was issued under still stands.
type GrantChecker interface {
	GrantValid(ctx context.Context, userID, grantID string) (bool, error)
}

// ErrGrantRevoked is returned for a token whose OAuth grant has been revoked.
var ErrGrantRevoked = errors.New("oauth grant revoked")

type service struct {
	repo        Repository
	userService user.Service
	grants      GrantChecker
	revocations RevocationStore
}

// ServiceOption configures the auth service.
type ServiceOption func(*service)

// WithGrantChecker makes ValidateToken refuse client tokens whose grant was revoked. Without it such tokens are accepted until they expire, which is what tests and tools without a database get.
func WithGrantChecker(g GrantChecker) ServiceOption {
	return func(s *service) { s.grants = g }
}

func NewService(repo Repository, userService user.Service, opts ...ServiceOption) Service {
	s := &service{
		repo:        repo,
		userService: userService,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *service) GetSigningKey(ctx context.Context) ([]byte, error) {
	value, err := s.repo.GetSigningKey(ctx, JWTSigningKeyID)
	if err == nil {
		return []byte(value), nil
	}

	if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("querying signing key: %w", err)
	}

	// Generate new key if it doesn't exist
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generating random bytes: %w", err)
	}

	value = base64.URLEncoding.EncodeToString(randomBytes)

	if err := s.repo.StoreSigningKey(ctx, JWTSigningKeyID, value); err != nil {
		return nil, fmt.Errorf("storing signing key: %w", err)
	}

	return []byte(value), nil
}

// GenerateToken is a BC shim; use GenerateTokenForPrincipal for new callers.
func (s *service) GenerateToken(ctx context.Context, u *user.User, preferencesClaims map[string]interface{}) (string, error) {
	if u == nil || u.ID == "" {
		return "", fmt.Errorf("cannot generate token for nil user")
	}
	p := NewUserPrincipal(u)
	return s.GenerateTokenForPrincipal(ctx, p, preferencesClaims)
}

func (s *service) GenerateTokenForClient(ctx context.Context, u *user.User, clientID, grantID string) (string, error) {
	if u == nil || u.ID == "" {
		return "", fmt.Errorf("cannot generate token for nil user")
	}
	return s.generate(ctx, NewUserPrincipal(u), nil, func(c *Claims) {
		c.ClientID = clientID
		c.GrantID = grantID
	})
}

func (s *service) GenerateTokenForPrincipal(ctx context.Context, p Principal, preferencesClaims map[string]interface{}) (string, error) {
	return s.generate(ctx, p, preferencesClaims, nil)
}

func (s *service) generate(ctx context.Context, p Principal, preferencesClaims map[string]interface{}, extra func(*Claims)) (string, error) {
	signingKey, err := s.GetSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("getting signing key: %w", err)
	}

	claims := &Claims{
		Roles:       p.Roles(),
		Permissions: p.Permissions(),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   p.ID(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		Preferences: preferencesClaims,
	}

	if p.Type() != PrincipalTypeUser {
		claims.PrincipalType = string(p.Type())
	}
	if extra != nil {
		extra(claims)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// ValidateToken validates the given JWT token and returns the claims. A signed-out token returns ErrTokenRevoked, and one whose sign-out state could not be read returns ErrTokenCheck.
func (s *service) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	claims, err := s.parse(ctx, tokenString)
	if err != nil {
		return nil, err
	}

	if err := s.checkRevoked(ctx, claims, tokenString); err != nil {
		return nil, err
	}

	// Only client tokens pay for this lookup; a failed lookup refuses the token rather than letting a revoked client back in.
	if claims.GrantID != "" && s.grants != nil {
		ok, err := s.grants.GrantValid(ctx, claims.Subject, claims.GrantID)
		if err != nil {
			return nil, fmt.Errorf("checking oauth grant: %w", err)
		}
		if !ok {
			return nil, ErrGrantRevoked
		}
	}

	return claims, nil
}

// parse checks the token's signature, algorithm and time claims and nothing that needs the database beyond the signing key.
func (s *service) parse(ctx context.Context, tokenString string) (*Claims, error) {
	signingKey, err := s.GetSigningKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting signing key: %w", err)
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return signingKey, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
