package auth

import (
	"context"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/marmotdata/marmot/internal/core/user"
)

type mockRepo struct {
	key string
}

func (m *mockRepo) GetSigningKey(_ context.Context, _ string) (string, error) {
	if m.key == "" {
		return "", ErrNotFound
	}
	return m.key, nil
}

func (m *mockRepo) StoreSigningKey(_ context.Context, _, value string) error {
	m.key = value
	return nil
}

func newTestAuthService(t *testing.T) Service {
	t.Helper()
	return NewService(&mockRepo{}, nil)
}

func TestGenerateToken_UserPrincipalTypeOmitted(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	u := &user.User{
		ID:       "user-abc",
		Username: "alice",
		Roles: []user.Role{
			{
				Name:        "viewer",
				Permissions: []user.Permission{{ResourceType: "assets", Action: "read"}},
			},
		},
	}

	tokenStr, err := svc.GenerateToken(ctx, u, nil)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := svc.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.PrincipalType != "" {
		t.Errorf("PrincipalType = %q, want empty for user token", claims.PrincipalType)
	}
	if claims.Subject != "user-abc" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-abc")
	}
}

func TestGenerateTokenForPrincipal_OperatorEmitsType(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	tokenStr, err := svc.GenerateTokenForPrincipal(ctx, NewOperatorPrincipal(), nil)
	if err != nil {
		t.Fatalf("GenerateTokenForPrincipal: %v", err)
	}

	claims, err := svc.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.PrincipalType != string(PrincipalTypeOperator) {
		t.Errorf("PrincipalType = %q, want %q", claims.PrincipalType, PrincipalTypeOperator)
	}
	if claims.Subject != OperatorPrincipalID {
		t.Errorf("Subject = %q, want %q", claims.Subject, OperatorPrincipalID)
	}
}

func TestGenerateTokenForPrincipal_UserPrincipalTypeOmitted(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	p := NewUserPrincipal(&user.User{ID: "u-1", Username: "bob"})
	tokenStr, err := svc.GenerateTokenForPrincipal(ctx, p, nil)
	if err != nil {
		t.Fatalf("GenerateTokenForPrincipal: %v", err)
	}

	claims, err := svc.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.PrincipalType != "" {
		t.Errorf("PrincipalType = %q, want empty for user principal", claims.PrincipalType)
	}
}

func TestValidateToken_LegacyTokenDecodesWithoutError(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	legacy, err := svc.GenerateToken(ctx, &user.User{ID: "leg-1", Username: "legacy"}, nil)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := svc.ValidateToken(ctx, legacy)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.PrincipalType != "" {
		t.Errorf("legacy token should decode with PrincipalType empty, got %q", claims.PrincipalType)
	}
}

func TestGenerateToken_NilUserReturnsError(t *testing.T) {
	svc := newTestAuthService(t)
	_, err := svc.GenerateToken(context.Background(), nil, nil)
	if err == nil {
		t.Error("GenerateToken(nil) should return an error")
	}
}

func TestValidateToken_ExpiredTokenRejected(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	// Seed the signing key by generating any valid token first.
	if _, err := svc.GenerateToken(ctx, &user.User{ID: "seed", Username: "seed"}, nil); err != nil {
		t.Fatalf("seeding key: %v", err)
	}

	key, err := svc.GetSigningKey(ctx)
	if err != nil {
		t.Fatalf("GetSigningKey: %v", err)
	}

	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "exp-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	})
	tokenStr, err := expired.SignedString(key)
	if err != nil {
		t.Fatalf("signing expired token: %v", err)
	}

	if _, err := svc.ValidateToken(ctx, tokenStr); err == nil {
		t.Error("ValidateToken should reject an expired token")
	}
}
