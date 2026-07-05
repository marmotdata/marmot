package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/marmotdata/marmot/internal/core/user"
)

var ErrUserInactive = errors.New("user account is inactive")

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
		if !u.Active {
			return nil, ErrUserInactive
		}
		return NewUserPrincipal(u), nil
	default:
		return nil, fmt.Errorf("unknown principal_type %q", claims.PrincipalType)
	}
}
