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
	calls       int
	movedAssets bool
	getErr      error
	scope       domain.Scope
	enforced    bool
}

func (f *fakeService) Enforcement(context.Context) (*domain.EnforcementState, error) {
	return &domain.EnforcementState{Write: f.enforced}, nil
}

func (f *fakeService) Scope(context.Context, auth.Principal) (*domain.Scope, error) {
	return &f.scope, nil
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

// passthroughGuard stands in for domain.Guard with enforcement off.
type passthroughGuard struct{ svc *fakeService }

func (g passthroughGuard) Transfer(ctx context.Context, kind domain.Kind, ids []string, domainID string) error {
	return g.svc.Assign(ctx, kind, ids, domainID)
}

func (g passthroughGuard) AssignPipeline(ctx context.Context, scheduleID, domainID string, move bool) (*domain.PipelineMoveResult, error) {
	return g.svc.AssignPipeline(ctx, scheduleID, domainID, move)
}

func (passthroughGuard) AuditMove(context.Context, string, *string, *string) error { return nil }

func (passthroughGuard) AuditLog(context.Context, string, string) ([]domain.AuditEntry, error) {
	return nil, nil
}

func handlerFor(svc *fakeService) *Handler {
	return &Handler{service: svc, guard: passthroughGuard{svc}}
}

func (f *fakeService) DomainOf(context.Context, domain.Kind, string) (string, error) {
	return domain.UnassignedID, f.err
}

func (f *fakeService) Import(_ context.Context, in domain.ImportInput) (*domain.ImportReport, error) {
	f.calls++
	return &domain.ImportReport{Applied: in.Apply}, f.err
}

func (f *fakeService) AssignPipeline(_ context.Context, scheduleID, domainID string, move bool) (*domain.PipelineMoveResult, error) {
	f.calls++
	f.assigned.ids, f.assigned.domain = []string{scheduleID}, domainID
	f.movedAssets = move
	return &domain.PipelineMoveResult{DomainID: domainID}, f.err
}

func (f *fakeService) Get(_ context.Context, id string) (*domain.Domain, error) {
	if id == "child" {
		parent := "parent"
		return &domain.Domain{ID: id, ParentID: &parent, Path: "/parent/child/"}, f.getErr
	}
	return &domain.Domain{ID: id, Path: "/" + id + "/"}, f.getErr
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

var manager = []string{"domains:manage"}

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
			rec := call(handlerFor(svc).create, http.MethodPost, "/api/v1/domains", tc.body, manager, nil)
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
	rec := call(handlerFor(svc).update, http.MethodPut, "/api/v1/domains/d", `{"restricted":true}`, manager, map[string]string{"id": "d"})
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
			rec := call(handlerFor(&fakeService{err: tc.err}).remove, http.MethodDelete, "/api/v1/domains/d", "", manager, map[string]string{"id": "d"})
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
	rec := call(handlerFor(&fakeService{err: domain.ErrCycle}).move, http.MethodPost, "/api/v1/domains/d/move", `{"parent_id":"c"}`, manager, map[string]string{"id": "d"})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"cycle"`) {
		t.Fatalf("move cycle: status %d: %s", rec.Code, rec.Body)
	}
}

func TestAssignNeedsTheEntityPermission(t *testing.T) {
	path := map[string]string{"id": "finance"}
	body := `{"kind":"glossary_term","ids":["t1","t2"]}`

	svc := &fakeService{}
	rec := call(handlerFor(svc).assign, http.MethodPut, "/api/v1/domains/finance/members", body, []string{"assets:manage"}, path)
	if rec.Code != http.StatusForbidden || svc.calls != 0 {
		t.Fatalf("without glossary:manage: status %d, calls %d", rec.Code, svc.calls)
	}

	svc = &fakeService{}
	rec = call(handlerFor(svc).assign, http.MethodPut, "/api/v1/domains/finance/members", body, []string{"glossary:manage"}, path)
	if rec.Code != http.StatusOK || svc.assigned.kind != domain.KindGlossaryTerm || len(svc.assigned.ids) != 2 || svc.assigned.domain != "finance" {
		t.Fatalf("status %d, assigned %+v", rec.Code, svc.assigned)
	}

	rec = call(handlerFor(&fakeService{}).assign, http.MethodPut, "/api/v1/domains/finance/members", `{"kind":"folder","ids":["x"]}`, []string{"assets:manage"}, path)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown kind status %d", rec.Code)
	}

	rec = call(handlerFor(&fakeService{err: domain.ErrEntityNotFound}).assign, http.MethodPut, "/api/v1/domains/finance/members", `{"kind":"asset","ids":["x"]}`, []string{"assets:manage"}, path)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing entity status %d", rec.Code)
	}
}

func TestDomainOfFallsBackToUnassigned(t *testing.T) {
	rec := call(handlerFor(&fakeService{}).domainOf, http.MethodGet, "/api/v1/domains/of/asset/a1", "", nil, map[string]string{"kind": "asset", "id": "a1"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), domain.UnassignedID) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

// Mirrors Server.RegisterRoutes: overlapping patterns would panic at startup.
func TestRoutesRegisterWithoutConflicts(t *testing.T) {
	h := NewHandler(&fakeService{}, nil, nil, nil, &config.Config{})
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
	rec := call(handlerFor(svc).importMemberships, http.MethodPost, "/api/v1/domains/import", body, []string{"domains:manage", "assets:manage"}, nil)
	if rec.Code != http.StatusForbidden || svc.calls != 0 {
		t.Fatalf("non-admin: status %d, calls %d", rec.Code, svc.calls)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/import", strings.NewReader(body))
	admin := auth.NewServiceAccountPrincipal("sa-admin", "admin robot", []string{auth.AdminRoleName}, nil)
	req = req.WithContext(context.WithValue(req.Context(), common.PrincipalContextKey, admin))
	rec = httptest.NewRecorder()
	handlerFor(svc).importMemberships(rec, req)
	if rec.Code != http.StatusOK || svc.calls != 1 || !strings.Contains(rec.Body.String(), `"applied":true`) {
		t.Fatalf("admin: status %d, calls %d: %s", rec.Code, svc.calls, rec.Body)
	}
}

func TestAssignPipelinePermissions(t *testing.T) {
	path := map[string]string{"scheduleId": "sched"}
	for _, tc := range []struct {
		name  string
		body  string
		perms []string
		want  int
	}{
		{"pipeline only needs ingestion:manage", `{"domain_id":"d"}`, []string{"ingestion:manage"}, http.StatusOK},
		{"moving assets also needs assets:manage", `{"domain_id":"d","move_assets":true}`, []string{"ingestion:manage"}, http.StatusForbidden},
		{"moving assets with both", `{"domain_id":"d","move_assets":true}`, []string{"ingestion:manage", "assets:manage"}, http.StatusOK},
		{"no ingestion:manage", `{"domain_id":"d"}`, []string{"assets:manage"}, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{}
			rec := call(handlerFor(svc).assignPipeline, http.MethodPut, "/api/v1/domains/pipelines/sched/assignment", tc.body, tc.perms, path)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if tc.want == http.StatusOK && svc.movedAssets != strings.Contains(tc.body, "move_assets") {
				t.Fatalf("move_assets not passed through: %v", svc.movedAssets)
			}
		})
	}

	svc := &fakeService{}
	call(handlerFor(svc).assignPipeline, http.MethodPut, "/x", `{"domain_id":""}`, []string{"ingestion:manage"}, path)
	if svc.assigned.domain != domain.UnassignedID {
		t.Fatalf("empty domain_id must mean Unassigned, got %q", svc.assigned.domain)
	}
}

func TestDomainAdminsManageTheirSubtree(t *testing.T) {
	adminOfParent := domain.Scope{Grants: []domain.Grant{{Path: "/parent/", Role: domain.RoleDomainAdmin}}}
	adminOfChild := domain.Scope{Grants: []domain.Grant{{Path: "/parent/child/", Role: domain.RoleDomainAdmin}}}
	steward := domain.Scope{Grants: []domain.Grant{{Path: "/parent/", Role: domain.RoleSteward}}}
	for _, tc := range []struct {
		name    string
		scope   domain.Scope
		handler func(*Handler) http.HandlerFunc
		method  string
		body    string
		id      string
		want    int
	}{
		{"domain admin creates a subdomain", adminOfParent, func(h *Handler) http.HandlerFunc { return h.create }, http.MethodPost, `{"name":"x","parent_id":"parent"}`, "", http.StatusCreated},
		{"domain admin cannot create a root", adminOfParent, func(h *Handler) http.HandlerFunc { return h.create }, http.MethodPost, `{"name":"x"}`, "", http.StatusForbidden},
		{"steward cannot create subdomains", steward, func(h *Handler) http.HandlerFunc { return h.create }, http.MethodPost, `{"name":"x","parent_id":"parent"}`, "", http.StatusForbidden},
		{"domain admin renames its domain", adminOfChild, func(h *Handler) http.HandlerFunc { return h.update }, http.MethodPut, `{"name":"y"}`, "child", http.StatusOK},
		{"domain admin cannot delete its own root", adminOfChild, func(h *Handler) http.HandlerFunc { return h.remove }, http.MethodDelete, "", "child", http.StatusForbidden},
		{"admin of the parent deletes it", adminOfParent, func(h *Handler) http.HandlerFunc { return h.remove }, http.MethodDelete, "", "child", http.StatusOK},
		{"moving out of the subtree is refused", adminOfParent, func(h *Handler) http.HandlerFunc { return h.move }, http.MethodPost, `{"parent_id":"elsewhere"}`, "child", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{scope: tc.scope}
			rec := call(tc.handler(handlerFor(svc)), tc.method, "/api/v1/domains", tc.body, nil, map[string]string{"id": tc.id})
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	svc := &fakeService{scope: domain.Scope{Grants: []domain.Grant{{Path: "/parent/", Role: domain.RoleSteward}}}, enforced: true}
	rec := call(handlerFor(svc).capabilities, http.MethodGet, "/api/v1/domains/capabilities?domain_id=child", "", nil, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"write":true`) || !strings.Contains(rec.Body.String(), `"admin":false`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	outside := &fakeService{scope: domain.Scope{Grants: []domain.Grant{{Path: "/elsewhere/", Role: domain.RoleSteward}}}, enforced: true}
	rec = call(handlerFor(outside).capabilities, http.MethodGet, "/api/v1/domains/capabilities?domain_id=child", "", nil, nil)
	if !strings.Contains(rec.Body.String(), `"write":false`) {
		t.Fatalf("enforced, outside the grant: %s", rec.Body)
	}
	outside.enforced = false
	rec = call(handlerFor(outside).capabilities, http.MethodGet, "/api/v1/domains/capabilities?domain_id=child", "", nil, nil)
	if !strings.Contains(rec.Body.String(), `"write":true`) || !strings.Contains(rec.Body.String(), `"enforced":false`) {
		t.Fatalf("not enforced, native RBAC decides: %s", rec.Body)
	}
	rec = call(handlerFor(svc).capabilities, http.MethodGet, "/api/v1/domains/capabilities", "", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("without a target: status %d", rec.Code)
	}
}
