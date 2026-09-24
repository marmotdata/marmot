package memory

import (
	"context"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/memory"
)

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
