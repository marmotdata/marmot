package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/marmotdata/marmot/internal/core/user"
)

func TestResolver_UserPrincipal(t *testing.T) {
	u := &user.User{ID: "u-1", Username: "alice", Active: true}
	svc := &mockUserService{getFn: func(_ context.Context, id string) (*user.User, error) {
		if id != "u-1" {
			t.Fatalf("unexpected id %q", id)
		}
		return u, nil
	}}

	p, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "u-1"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Type() != PrincipalTypeUser {
		t.Errorf("Type() = %q, want %q", p.Type(), PrincipalTypeUser)
	}
	if p.AsUser() != u {
		t.Error("AsUser() did not return the wrapped user")
	}
}

func TestResolver_ExplicitUserType(t *testing.T) {
	u := &user.User{ID: "u-2", Username: "bob", Active: true}
	svc := &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) { return u, nil }}

	p, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		PrincipalType:    string(PrincipalTypeUser),
		RegisteredClaims: jwt.RegisteredClaims{Subject: "u-2"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Type() != PrincipalTypeUser {
		t.Errorf("Type() = %q, want %q", p.Type(), PrincipalTypeUser)
	}
}

func TestResolver_OperatorPrincipal(t *testing.T) {
	svc := &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) {
		t.Fatal("Get should not be called for operator principal")
		return nil, nil
	}}

	p, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		PrincipalType: string(PrincipalTypeOperator),
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Type() != PrincipalTypeOperator {
		t.Errorf("Type() = %q, want %q", p.Type(), PrincipalTypeOperator)
	}
	if !p.IsAdmin() {
		t.Error("operator IsAdmin() = false, want true")
	}
}

func TestResolver_InactiveUser(t *testing.T) {
	svc := &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) {
		return &user.User{ID: "u-3", Active: false}, nil
	}}

	_, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "u-3"},
	})
	if !errors.Is(err, ErrUserInactive) {
		t.Errorf("err = %v, want ErrUserInactive", err)
	}
}

func TestResolver_UserNotFound(t *testing.T) {
	svc := &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) {
		return nil, user.ErrUserNotFound
	}}

	_, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "missing"},
	})
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestResolver_UnknownType(t *testing.T) {
	svc := &mockUserService{}
	_, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		PrincipalType: "superadmin",
	})
	if err == nil {
		t.Error("expected error for unknown principal_type")
	}
}

// A leaked token could previously only be killed by rotating the signing key,
// which signed everyone out. The cutoff rejects tokens issued before the user's
// sessions were invalidated while leaving every other user untouched.
func TestResolver_SessionRevocation(t *testing.T) {
	cutoff := time.Date(2026, 9, 21, 12, 0, 0, 500_000_000, time.UTC)

	tests := []struct {
		name     string
		issuedAt *jwt.NumericDate
		wantErr  error
	}{
		{
			name:     "token issued before the cutoff is revoked",
			issuedAt: jwt.NewNumericDate(cutoff.Add(-time.Hour)),
			wantErr:  ErrSessionRevoked,
		},
		{
			// jwt iat carries whole seconds, so a replacement token minted in the
			// same second as the invalidation has to survive or a password change
			// would hand back a dead token.
			name:     "token issued in the same second survives",
			issuedAt: jwt.NewNumericDate(cutoff.Truncate(time.Second)),
		},
		{
			name:     "token issued after the cutoff survives",
			issuedAt: jwt.NewNumericDate(cutoff.Add(time.Hour)),
		},
		{
			// A token without iat cannot prove it postdates the cutoff, so once
			// one exists the token is refused rather than trusted.
			name:    "token without iat is revoked once a cutoff exists",
			wantErr: ErrSessionRevoked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{ID: "u-1", Username: "alice", Active: true, SessionsInvalidatedAt: &cutoff}
			svc := &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) { return u, nil }}

			_, err := NewResolver(svc).Resolve(context.Background(), &Claims{
				RegisteredClaims: jwt.RegisteredClaims{Subject: "u-1", IssuedAt: tt.issuedAt},
			})

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
		})
	}
}

func TestResolver_NoCutoffAcceptsAnyIssuedAt(t *testing.T) {
	u := &user.User{ID: "u-1", Username: "alice", Active: true}
	svc := &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) { return u, nil }}

	if _, err := NewResolver(svc).Resolve(context.Background(), &Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "u-1"},
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}
