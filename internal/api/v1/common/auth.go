package common

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/user"
)

var (
	anonymousUser     *user.User
	anonymousUserLock sync.RWMutex
)

// GetAnonymousUser returns a singleton anonymous user with the specified role
func GetAnonymousUser(roleName string) *user.User {
	anonymousUserLock.RLock()
	if anonymousUser != nil && anonymousUser.Roles[0].Name == roleName {
		defer anonymousUserLock.RUnlock()
		return anonymousUser
	}
	anonymousUserLock.RUnlock()

	anonymousUserLock.Lock()
	defer anonymousUserLock.Unlock()

	if anonymousUser != nil && anonymousUser.Roles[0].Name == roleName {
		return anonymousUser
	}

	anonymousUser = &user.User{
		ID:       uuid.MustParse("00000000-0000-0000-0000-000000000000").String(), // Fixed ID for anonymous user
		Username: "anonymous",
		Name:     "Anonymous User",
		Active:   true,
		Roles: []user.Role{
			{
				Name: roleName,
			},
		},
	}
	return anonymousUser
}

// AnonymousContext represents an authentication context for anonymous access
type AnonymousContext struct {
	RoleName string
}

// WithAnonymousContext adds anonymous context to the request context
func WithAnonymousContext(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, "anonymous_role", AnonymousContext{RoleName: role})
}

// GetAnonymousContext retrieves anonymous context from the request context
func GetAnonymousContext(ctx context.Context) (AnonymousContext, bool) {
	val, ok := ctx.Value("anonymous_role").(AnonymousContext)
	return val, ok
}
