package domains

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/domain"
)

type knownDomains map[string]bool

func (k knownDomains) Get(_ context.Context, id string) (*domain.Domain, error) {
	if !k[id] {
		return nil, domain.ErrNotFound
	}
	return &domain.Domain{ID: id}, nil
}

type createRoutes struct{ reached *int }

func (c createRoutes) Routes() []common.Route {
	return []common.Route{{Path: "/api/v1/products/", Method: http.MethodPost, Handler: func(w http.ResponseWriter, _ *http.Request) {
		*c.reached++
		w.WriteHeader(http.StatusCreated)
	}}}
}

func TestCreateTargets(t *testing.T) {
	reached := 0
	routes := WithCreateTargets(createRoutes{&reached}, knownDomains{"finance": true}, "/api/v1/products/").Routes()
	handler := routes[0].Handler
	for _, mw := range routes[0].Middleware {
		handler = mw(handler)
	}
	for _, tc := range []struct {
		url  string
		want int
	}{
		{"/api/v1/products/", http.StatusCreated},
		{"/api/v1/products/?domain_id=finance", http.StatusCreated},
		{"/api/v1/products/?domain_id=nowhere", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, tc.url, nil))
		if rec.Code != tc.want {
			t.Errorf("%s = %d, want %d", tc.url, rec.Code, tc.want)
		}
	}
	if reached != 2 {
		t.Fatalf("handler reached %d times, want 2", reached)
	}
}
