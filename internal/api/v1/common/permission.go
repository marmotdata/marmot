package common

import "context"

// HasPermission reports whether the request's principal holds a permission across the whole catalog. Handlers use it for decisions a route gate cannot make, such as whether to show everyone's rows or only the caller's.
func HasPermission(ctx context.Context, resourceType, action string) bool {
	p, ok := PrincipalFromContext(ctx)
	return ok && (p.IsAdmin() || p.HasPermission(resourceType, action))
}
