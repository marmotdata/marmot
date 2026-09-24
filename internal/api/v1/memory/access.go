package memory

import (
	"context"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/memory"
	"github.com/marmotdata/marmot/internal/core/user"
)

// Access holds the memory access checks, so callers outside this package run
// the same checks as the HTTP routes.
type Access struct {
	Users user.Service
}

// CanRead reports whether the caller may read an entity's memory. It is the
// same check as seeing the catalog.
func (a Access) CanRead(ctx context.Context, _ memory.Entity) (bool, error) {
	return common.HasPermission(ctx, a.Users, "assets", "view")
}

// CanWrite reports whether the caller holds memory:write.
func (a Access) CanWrite(ctx context.Context, _ memory.Entity) (bool, error) {
	return common.HasPermission(ctx, a.Users, "memory", "write")
}

// AuthorFromContext is the request's principal as a memory author.
func AuthorFromContext(ctx context.Context) (memory.Author, bool) {
	p, ok := common.PrincipalFromContext(ctx)
	if !ok || p == nil {
		return memory.Author{}, false
	}
	return memory.AuthorFrom(p), true
}

// entityFrom reads the entity from the path. It writes a 400 for an unknown
// entity type and returns false. Permissions are checked by the route.
func entityFrom(w http.ResponseWriter, r *http.Request) (memory.Entity, bool) {
	e := memory.Entity{Type: memory.EntityType(r.PathValue("entityType")), ID: r.PathValue("entityId")}
	if !e.Type.Valid() {
		common.RespondError(w, http.StatusBadRequest, "Unsupported entity type")
		return e, false
	}
	return e, true
}
