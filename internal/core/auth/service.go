package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/marmotdata/marmot/internal/core/user"
)

const JWTSigningKeyID = "jwt_signing_key"

type Claims struct {
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
	jwt.RegisteredClaims
}

type Service interface {
	GenerateToken(ctx context.Context, user *user.User, preferencesClaims map[string]interface{}) (string, error)
	ValidateToken(ctx context.Context, tokenString string) (*Claims, error)
	GetSigningKey(ctx context.Context) ([]byte, error)
}

type service struct {
	repo        Repository
	userService user.Service
}

func NewService(repo Repository, userService user.Service) Service {
	return &service{
		repo:        repo,
		userService: userService,
	}
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

func (s *service) GenerateToken(ctx context.Context, user *user.User, preferencesClaims map[string]interface{}) (string, error) {
	signingKey, err := s.GetSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("getting signing key: %w", err)
	}

	roleNames := make([]string, len(user.Roles))
	permissionSet := make(map[string]bool)

	for i, role := range user.Roles {
		roleNames[i] = role.Name

		for _, perm := range role.Permissions {
			permKey := perm.ResourceType + ":" + perm.Action
			permissionSet[permKey] = true
		}
	}

	permissions := make([]string, 0, len(permissionSet))
	for perm := range permissionSet {
		permissions = append(permissions, perm)
	}

	claims := &Claims{
		Roles:       roleNames,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		Preferences: preferencesClaims,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// ValidateToken validates the given JWT token and returns the claims.
func (s *service) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
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
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
