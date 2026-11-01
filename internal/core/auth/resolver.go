package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/user"
)

var ErrUserInactive = errors.New("user account is inactive")

var ErrSessionRevoked = errors.New("session revoked")

// Resolver converts validated JWT claims to a Principal.
type Resolver interface {
	Resolve(ctx context.Context, claims *Claims) (Principal, error)
}

type userResolver struct {
	users user.Service
}

func NewResolver(users user.Service) Resolver {
	return &userResolver{users: users}
}

func (r *userResolver) Resolve(ctx context.Context, claims *Claims) (Principal, error) {
	switch PrincipalType(claims.PrincipalType) {
	case PrincipalTypeOperator:
		return NewOperatorPrincipal(), nil
	case PrincipalTypeUser, "":
		u, err := r.users.Get(ctx, claims.Subject)
		if err != nil {
			return nil, fmt.Errorf("resolving user: %w", err)
		}
		if err := CheckUserSession(u, claims.IssuedTime()); err != nil {
			return nil, err
		}
		return NewUserPrincipal(u), nil
	default:
		return nil, fmt.Errorf("unknown principal_type %q", claims.PrincipalType)
	}
}

// CheckUserSession reports whether a credential issued at issuedAt may still act for u, failing if the account is inactive or its sessions were revoked after issue; every path that turns a credential into a user must call it
func CheckUserSession(u *user.User, issuedAt time.Time) error {
	if !u.Active {
		return ErrUserInactive
	}
	if u.SessionsInvalidatedAt != nil && issuedAt.Before(u.SessionsInvalidatedAt.Truncate(time.Second)) {
		return ErrSessionRevoked
	}
	return nil
}
