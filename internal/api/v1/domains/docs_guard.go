package domains

import (
	"context"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/rs/zerolog/log"
)

type routeProvider interface {
	Routes() []common.Route
}

// DocGuard is the part of domain.Guard that scopes documentation pages.
type DocGuard interface {
	AuthorizeDoc(ctx context.Context, entityType, entityID, pageID, imageID string) error
}

type guardedDocs struct {
	inner routeProvider
	guard DocGuard
}

// GuardDocs scopes the documentation page routes by the entity that owns each
// page. docs.Service is a concrete type with no interface to decorate, so the
// check runs as the innermost middleware of every write route, after auth.
func GuardDocs(inner routeProvider, guard DocGuard) routeProvider {
	return &guardedDocs{inner: inner, guard: guard}
}

func (d *guardedDocs) Routes() []common.Route {
	routes := d.inner.Routes()
	for i, route := range routes {
		if route.Method == http.MethodGet || route.Method == http.MethodOptions {
			continue
		}
		mw := append([]func(http.HandlerFunc) http.HandlerFunc{}, route.Middleware...)
		routes[i].Middleware = append(mw, d.check)
	}
	return routes
}

func (d *guardedDocs) check(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := d.guard.AuthorizeDoc(r.Context(), r.PathValue("entityType"), r.PathValue("entityId"), r.PathValue("pageId"), r.PathValue("imageId"))
		switch {
		case errors.Is(err, domain.ErrForbidden):
			respondCode(w, http.StatusForbidden, "forbidden", "Not allowed in this domain")
		case err != nil:
			log.Error().Err(err).Str("path", r.URL.Path).Msg("Failed to check documentation domain")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		default:
			next(w, r)
		}
	}
}
