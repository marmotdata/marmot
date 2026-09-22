package assets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/pkg/config"
)

type fieldService struct {
	asset.Service
	err    error
	called bool
}

func (s *fieldService) PatchFields(_ context.Context, id string, version int64, fields map[string]any) (*asset.Asset, error) {
	s.called = true
	if id != "asset-id" || version != 3 || fields["name"] != "renamed" {
		return nil, errors.New("unexpected patch")
	}
	return &asset.Asset{ID: id, Version: 4}, s.err
}

func TestPatchNativeAssetHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, header, body string
		err                error
		status             int
		called             bool
	}{
		{"success", `"3"`, `{"fields":{"name":"renamed"}}`, nil, 200, true},
		{"missing precondition", "", `{"fields":{}}`, nil, 428, false},
		{"bare precondition", "3", `{"fields":{}}`, nil, 400, false},
		{"weak precondition", `W/"3"`, `{"fields":{}}`, nil, 400, false},
		{"malformed quote", `"3`, `{"fields":{}}`, nil, 400, false},
		{"unknown property", `"3"`, `{"fields":{},"owner":true}`, nil, 400, false},
		{"multiple documents", `"3"`, `{"fields":{}} {}`, nil, 400, false},
		{"missing fields", `"3"`, `{}`, nil, 400, false},
		{"oversized body", `"3"`, `{"fields":{"name":"` + strings.Repeat("a", 1<<20) + `"}}`, nil, 400, false},
		{"stale version", `"3"`, `{"fields":{"name":"renamed"}}`, asset.ErrVersionConflict, 412, true},
		{"unknown asset", `"3"`, `{"fields":{"name":"renamed"}}`, asset.ErrAssetNotFound, 404, true},
		{"invalid field", `"3"`, `{"fields":{"name":"renamed"}}`, &metamodel.ValidationError{Fields: []metamodel.Violation{{Field: "name", Code: "required"}}}, 400, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fieldService{err: tc.err}
			h := &Handler{assetService: svc, config: &config.Config{}}
			// Register the actual asset paths, including both existing methods and PATCH.
			mux := http.NewServeMux()
			seen := map[string]bool{}
			for _, route := range h.Routes() {
				if route.Method == http.MethodPatch {
					if route.Path != "/api/v1/assets/{id}" || len(route.Middleware) != 2 {
						t.Fatal("patch lost native path or auth middleware")
					}
				}
				if !seen[route.Path] {
					mux.HandleFunc(route.Path, h.patchAssetFields)
					seen[route.Path] = true
				}
			}
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/assets/asset-id", strings.NewReader(tc.body))
			req.Header.Set("If-Match", tc.header)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.status || svc.called != tc.called {
				t.Fatalf("status=%d called=%v body=%s", rec.Code, svc.called, rec.Body.String())
			}
			if tc.status == 200 && rec.Header().Get("ETag") != `"4"` {
				t.Fatal("missing quoted ETag")
			}
		})
	}
}

func TestInvalidPUTPreconditionIsNotIgnored(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/assets/asset-id", strings.NewReader(`{"name":"renamed"}`))
	req.SetPathValue("id", "asset-id")
	req.Header.Set("If-Match", "garbage")
	rec := httptest.NewRecorder()
	h.updateAsset(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMetamodelRoutesRequireAuthentication(t *testing.T) {
	h := &Handler{config: &config.Config{}}
	for _, route := range h.Routes() {
		if route.Method != http.MethodPatch && route.Path != "/api/v1/metamodel" {
			continue
		}
		handler := route.Handler
		for i := len(route.Middleware) - 1; i >= 0; i-- {
			handler = route.Middleware[i](handler)
		}
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(route.Method, "/api/v1/assets/asset-id", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", route.Method, route.Path, rec.Code)
		}
	}
}
