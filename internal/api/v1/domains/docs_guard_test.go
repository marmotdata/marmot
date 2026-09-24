package domains

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/domain"
)

type docRoutes struct{}

func (docRoutes) Routes() []common.Route {
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	return []common.Route{
		{Path: "/api/v1/docs/pages/{pageId}", Method: http.MethodGet, Handler: ok},
		{Path: "/api/v1/docs/pages/{pageId}", Method: http.MethodPut, Handler: ok},
		{Path: "/api/v1/docs/images/{imageId}", Method: http.MethodDelete, Handler: ok},
	}
}

type denyPages struct{ seen []string }

func (d *denyPages) AuthorizeDoc(_ context.Context, _, _, pageID, imageID string) error {
	d.seen = append(d.seen, pageID+imageID)
	return domain.ErrForbidden
}

func TestGuardDocsChecksWritesOnly(t *testing.T) {
	guard := &denyPages{}
	mux := http.NewServeMux()
	for _, route := range GuardDocs(docRoutes{}, guard).Routes() {
		handler := route.Handler
		for i := len(route.Middleware) - 1; i >= 0; i-- {
			handler = route.Middleware[i](handler)
		}
		mux.HandleFunc(route.Method+" "+route.Path, handler)
	}
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/v1/docs/pages/p1", http.StatusOK},
		{http.MethodPut, "/api/v1/docs/pages/p1", http.StatusForbidden},
		{http.MethodDelete, "/api/v1/docs/images/i1", http.StatusForbidden},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
	if len(guard.seen) != 2 || guard.seen[0] != "p1" || guard.seen[1] != "i1" {
		t.Fatalf("checked %v", guard.seen)
	}
}
