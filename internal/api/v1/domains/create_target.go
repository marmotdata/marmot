package domains

import (
	"context"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/rs/zerolog/log"
)

// DomainLookup is the part of domain.Service the create middleware needs.
type DomainLookup interface {
	Get(ctx context.Context, id string) (*domain.Domain, error)
}

type createTargets struct {
	inner   routeProvider
	domains DomainLookup
	paths   map[string]bool
}

// WithCreateTargets lets the create routes at paths take an optional
// ?domain_id=, so an entity is created straight into its domain. The upstream
// handlers are untouched: the target travels in the context to the decorators,
// which check it and place the new entity.
func WithCreateTargets(inner routeProvider, domains DomainLookup, paths ...string) routeProvider {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return &createTargets{inner: inner, domains: domains, paths: set}
}

func (c *createTargets) Routes() []common.Route {
	routes := c.inner.Routes()
	for i, route := range routes {
		if route.Method != http.MethodPost || !c.paths[route.Path] {
			continue
		}
		mw := append([]func(http.HandlerFunc) http.HandlerFunc{}, route.Middleware...)
		routes[i].Middleware = append(mw, c.target)
	}
	return routes
}

func (c *createTargets) target(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("domain_id")
		if id == "" {
			next(w, r)
			return
		}
		_, err := c.domains.Get(r.Context(), id)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			respondCode(w, http.StatusNotFound, "not_found", "Target domain not found")
		case err != nil:
			log.Error().Err(err).Str("domain", id).Msg("Failed to resolve create target domain")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		default:
			next(w, r.WithContext(domain.WithTarget(r.Context(), id)))
		}
	}
}
