package auth

import (
	"context"
	"errors"
	"testing"

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
