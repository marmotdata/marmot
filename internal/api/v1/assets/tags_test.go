package assets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/metamodel"
)

type tagService struct {
	asset.Service
	err error
}

func (s *tagService) AddTag(context.Context, string, string) (*asset.Asset, error) {
	return nil, s.err
}

func (s *tagService) RemoveTag(context.Context, string, string) (*asset.Asset, error) {
	return nil, s.err
}

func TestTagWriteErrors(t *testing.T) {
	invalid := &metamodel.ValidationError{Fields: []metamodel.Violation{{Field: "tags", Code: "items"}}}
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"validation", invalid, http.StatusBadRequest},
		{"version conflict", asset.ErrVersionConflict, http.StatusPreconditionFailed},
		{"not found", asset.ErrAssetNotFound, http.StatusNotFound},
	} {
		for _, op := range []struct {
			method  string
			handler func(*Handler) http.HandlerFunc
		}{
			{http.MethodPost, func(h *Handler) http.HandlerFunc { return h.addTag }},
			{http.MethodDelete, func(h *Handler) http.HandlerFunc { return h.removeTag }},
		} {
			t.Run(tc.name+" "+op.method, func(t *testing.T) {
				h := &Handler{assetService: &tagService{err: tc.err}}
				req := httptest.NewRequest(op.method, "/api/v1/assets/asset-id/tags", strings.NewReader(`{"tag":"pii"}`))
				req.SetPathValue("id", "asset-id")
				rec := httptest.NewRecorder()
				op.handler(h)(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}
	}
}
