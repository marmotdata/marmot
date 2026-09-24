package domains

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/pkg/config"
)

type fakeService struct {
	domain.Service
	err      error
	created  domain.CreateInput
	assigned struct {
		kind   domain.Kind
		ids    []string
		domain string
	}
	calls int
}

func (f *fakeService) Create(_ context.Context, in domain.CreateInput) (*domain.Domain, error) {
	f.calls++
	f.created = in
	return &domain.Domain{ID: "new", Name: in.Name}, f.err
}

func (f *fakeService) Update(context.Context, string, domain.UpdateInput) (*domain.Domain, error) {
	f.calls++
	return &domain.Domain{}, f.err
}

func (f *fakeService) Delete(context.Context, string) error {
	f.calls++
	return f.err
}

func (f *fakeService) Move(context.Context, string, *string) (*domain.Domain, error) {
	f.calls++
	return &domain.Domain{}, f.err
}

func (f *fakeService) Assign(_ context.Context, kind domain.Kind, ids []string, domainID string) error {
	f.calls++
	f.assigned.kind, f.assigned.ids, f.assigned.domain = kind, ids, domainID
	return f.err
}

func (f *fakeService) DomainOf(context.Context, domain.Kind, string) (string, error) {
	return domain.UnassignedID, f.err
}

func (f *fakeService) Import(_ context.Context, in domain.ImportInput) (*domain.ImportReport, error) {
	f.calls++
	return &domain.ImportReport{Applied: in.Apply}, f.err
}

func (f *fakeService) Get(_ context.Context, id string) (*domain.Domain, error) {
	return &domain.Domain{ID: id}, f.err
}

func call(h http.HandlerFunc, method, target, body string, perms []string, pathValues map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	principal := auth.NewServiceAccountPrincipal("sa-1", "robot", nil, perms)
	req = req.WithContext(context.WithValue(req.Context(), common.PrincipalContextKey, principal))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestCreate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		err   error
		want  int
		calls int
	}{
		{"created", `{"name":"Finance"}`, nil, http.StatusCreated, 1},
		{"restricted is refused before the service", `{"name":"Finance","restricted":true}`, nil, http.StatusBadRequest, 0},
		{"restricted false is accepted", `{"name":"Finance","restricted":false}`, nil, http.StatusCreated, 1},
		{"malformed body", `{`, nil, http.StatusBadRequest, 0},
		{"sibling name taken", `{"name":"Finance"}`, domain.ErrNameConflict, http.StatusConflict, 1},
		{"invalid input", `{"name":""}`, domain.ErrInvalidInput, http.StatusBadRequest, 1},
		{"missing parent", `{"name":"x","parent_id":"nope"}`, domain.ErrNotFound, http.StatusNotFound, 1},
		{"too deep", `{"name":"x","parent_id":"deep"}`, domain.ErrTooDeep, http.StatusBadRequest, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{err: tc.err}
			rec := call((&Handler{service: svc}).create, http.MethodPost, "/api/v1/domains", tc.body, nil, nil)
			if rec.Code != tc.want || svc.calls != tc.calls {
				t.Fatalf("status %d (want %d), calls %d (want %d): %s", rec.Code, tc.want, svc.calls, tc.calls, rec.Body)
			}
			if tc.want == http.StatusCreated && svc.created.CreatedBy != "sa-1" {
				t.Fatalf("created_by = %q", svc.created.CreatedBy)
			}
		})
	}
}

func TestUpdateRefusesRestricted(t *testing.T) {
	svc := &fakeService{}
	rec := call((&Handler{service: svc}).update, http.MethodPut, "/api/v1/domains/d", `{"restricted":true}`, nil, map[string]string{"id": "d"})
	if rec.Code != http.StatusBadRequest || svc.calls != 0 {
		t.Fatalf("status %d, calls %d", rec.Code, svc.calls)
	}
}

func TestStructuralErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"has children", domain.ErrHasChildren, http.StatusConflict},
		{"not empty", domain.ErrNotEmpty, http.StatusConflict},
		{"protected", domain.ErrProtected, http.StatusConflict},
		{"not found", domain.ErrNotFound, http.StatusNotFound},
	} {
		t.Run("delete "+tc.name, func(t *testing.T) {
			rec := call((&Handler{service: &fakeService{err: tc.err}}).remove, http.MethodDelete, "/api/v1/domains/d", "", nil, map[string]string{"id": "d"})
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
	rec := call((&Handler{service: &fakeService{err: domain.ErrCycle}}).move, http.MethodPost, "/api/v1/domains/d/move", `{"parent_id":"c"}`, nil, map[string]string{"id": "d"})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"cycle"`) {
		t.Fatalf("move cycle: status %d: %s", rec.Code, rec.Body)
	}
}

func TestAssignNeedsTheEntityPermission(t *testing.T) {
	path := map[string]string{"id": "finance"}
	body := `{"kind":"glossary_term","ids":["t1","t2"]}`

	svc := &fakeService{}
	rec := call((&Handler{service: svc}).assign, http.MethodPut, "/api/v1/domains/finance/members", body, []string{"assets:manage"}, path)
	if rec.Code != http.StatusForbidden || svc.calls != 0 {
		t.Fatalf("without glossary:manage: status %d, calls %d", rec.Code, svc.calls)
	}

	svc = &fakeService{}
	rec = call((&Handler{service: svc}).assign, http.MethodPut, "/api/v1/domains/finance/members", body, []string{"glossary:manage"}, path)
	if rec.Code != http.StatusOK || svc.assigned.kind != domain.KindGlossaryTerm || len(svc.assigned.ids) != 2 || svc.assigned.domain != "finance" {
		t.Fatalf("status %d, assigned %+v", rec.Code, svc.assigned)
	}

	rec = call((&Handler{service: &fakeService{}}).assign, http.MethodPut, "/api/v1/domains/finance/members", `{"kind":"folder","ids":["x"]}`, []string{"assets:manage"}, path)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown kind status %d", rec.Code)
	}

	rec = call((&Handler{service: &fakeService{err: domain.ErrEntityNotFound}}).assign, http.MethodPut, "/api/v1/domains/finance/members", `{"kind":"asset","ids":["x"]}`, []string{"assets:manage"}, path)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing entity status %d", rec.Code)
	}
}

func TestDomainOfFallsBackToUnassigned(t *testing.T) {
	rec := call((&Handler{service: &fakeService{}}).domainOf, http.MethodGet, "/api/v1/domains/of/asset/a1", "", nil, map[string]string{"kind": "asset", "id": "a1"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), domain.UnassignedID) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

// Mirrors Server.RegisterRoutes: overlapping patterns would panic at startup.
func TestRoutesRegisterWithoutConflicts(t *testing.T) {
	h := NewHandler(&fakeService{}, nil, nil, &config.Config{})
	mux := http.NewServeMux()
	seen := map[string]bool{}
	for _, route := range h.Routes() {
		base := strings.TrimSuffix(route.Path, "/")
		for _, pattern := range []string{base, base + "/{$}"} {
			if !seen[pattern] {
				seen[pattern] = true
				mux.HandleFunc(pattern, func(http.ResponseWriter, *http.Request) {})
			}
		}
	}
}

func TestImportRequiresAGlobalAdministrator(t *testing.T) {
	body := `{"source":"metadata.dgu.domain","mapping":{"Finanzas":"d"},"apply":true}`
	svc := &fakeService{}
	rec := call((&Handler{service: svc}).importMemberships, http.MethodPost, "/api/v1/domains/import", body, []string{"domains:manage", "assets:manage"}, nil)
	if rec.Code != http.StatusForbidden || svc.calls != 0 {
		t.Fatalf("non-admin: status %d, calls %d", rec.Code, svc.calls)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/import", strings.NewReader(body))
	admin := auth.NewServiceAccountPrincipal("sa-admin", "admin robot", []string{auth.AdminRoleName}, nil)
	req = req.WithContext(context.WithValue(req.Context(), common.PrincipalContextKey, admin))
	rec = httptest.NewRecorder()
	(&Handler{service: svc}).importMemberships(rec, req)
	if rec.Code != http.StatusOK || svc.calls != 1 || !strings.Contains(rec.Body.String(), `"applied":true`) {
		t.Fatalf("admin: status %d, calls %d: %s", rec.Code, svc.calls, rec.Body)
	}
}
