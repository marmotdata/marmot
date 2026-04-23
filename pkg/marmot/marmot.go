package marmot

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/marmotdata/marmot/internal/api/v1"
	"github.com/marmotdata/marmot/internal/staticfiles"
	"github.com/marmotdata/marmot/internal/store/postgres"
	"github.com/marmotdata/marmot/pkg/config"
)

// Server wraps the internal Marmot API server for use by external modules.
type Server struct {
	internal *v1.Server
}

// NewServer creates a new Marmot server with all services initialized.
func NewServer(cfg *config.Config, db *pgxpool.Pool) *Server {
	return &Server{internal: v1.New(cfg, db)}
}

// RegisterRoutes registers all API routes on the given ServeMux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	s.internal.RegisterRoutes(mux)
}

// Stop gracefully shuts down all background services.
func (s *Server) Stop() {
	s.internal.Stop()
}

// MigrateDB runs database migrations against the provided pool.
func MigrateDB(ctx context.Context, db *pgxpool.Pool) error {
	return postgres.NewSetup(db).Initialize(ctx)
}

// SetupStaticFiles registers the embedded SPA file server on the mux.
func SetupStaticFiles(mux *http.ServeMux) error {
	return staticfiles.New().SetupRoutes(mux)
}

// SecurityHeaders wraps an http.Handler to set standard security headers.
func SecurityHeaders(next http.Handler, custom map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; frame-ancestors 'self'")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		for key, value := range custom {
			w.Header().Set(key, value)
		}

		next.ServeHTTP(w, r)
	})
}
