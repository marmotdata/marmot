package glossary

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/glossary/importer"
	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/pkg/config"
)

type noTerms struct{}

func (noTerms) ByNames(context.Context, []string) (map[string][]*glossary.GlossaryTerm, error) {
	return map[string][]*glossary.GlossaryTerm{}, nil
}

type anyOwner struct{}

func (anyOwner) ResolveOwner(_ context.Context, ref string) (glossary.OwnerInput, error) {
	if ref == "ghost" {
		return glossary.OwnerInput{}, importer.ErrOwnerNotFound
	}
	return glossary.OwnerInput{ID: ref, Type: "user"}, nil
}

type importingGlossary struct {
	glossary.Service
	imported int
}

func (g *importingGlossary) Import(_ context.Context, terms []glossary.ImportTerm) ([]*glossary.GlossaryTerm, error) {
	g.imported += len(terms)
	return nil, nil
}

func upload(t *testing.T, h http.HandlerFunc, query, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	_ = form.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/glossary/import"+query, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestImportEndpoints(t *testing.T) {
	svc := &importingGlossary{}
	h := NewHandler(svc, importer.New(metamodel.Native(), noTerms{}, anyOwner{}), nil, nil, &config.Config{}, nil)

	mux := http.NewServeMux()
	for _, route := range h.Routes() {
		mux.HandleFunc(route.Method+" "+route.Path, route.Handler)
	}

	rec := httptest.NewRecorder()
	h.importTemplate(rec, httptest.NewRequest(http.MethodGet, "/api/v1/glossary/import/template?format=csv", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Body.String(), "name,definition,description,parent,owners,tags") {
		t.Fatalf("template: %d %s %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	valid := "name,definition,owners\nInvoice,A bill,ana\n"
	rec = upload(t, h.importTerms, "", "terms.csv", valid)
	var result importer.Result
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if rec.Code != http.StatusOK || result.Summary.Create != 1 || result.Applied || svc.imported != 0 {
		t.Fatalf("validate: %d %s", rec.Code, rec.Body)
	}

	rec = upload(t, h.importTerms, "?mode=apply", "terms.csv", "name,definition,owners\nInvoice,A bill,ghost\n")
	if rec.Code != http.StatusUnprocessableEntity || svc.imported != 0 {
		t.Fatalf("apply with errors: %d %s", rec.Code, rec.Body)
	}

	rec = upload(t, h.importTerms, "?mode=apply", "terms.csv", valid)
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if rec.Code != http.StatusOK || !result.Applied || svc.imported != 1 {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body)
	}

	rec = upload(t, h.importTerms, "", "terms.txt", valid)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported format: %d", rec.Code)
	}
	rec = upload(t, h.importTerms, "?on_existing=overwrite", "terms.csv", valid)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad on_existing: %d", rec.Code)
	}
}
