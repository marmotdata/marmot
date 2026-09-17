package assets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/metamodel"
)

func TestParseIfMatch(t *testing.T) {
	if v, ok := parseIfMatch(`"3"`); !ok || v != 3 {
		t.Fatalf("quoted: %d %v", v, ok)
	}
	if v, ok := parseIfMatch("2"); !ok || v != 2 {
		t.Fatalf("bare: %d %v", v, ok)
	}
	for _, header := range []string{"", "*", "0", "-1", "abc"} {
		if _, ok := parseIfMatch(header); ok {
			t.Fatalf("accepted %q", header)
		}
	}
}

func TestRespondAssetWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	if !respondAssetWriteError(rec, &metamodel.ValidationError{Fields: []metamodel.Violation{{Field: "retention", Code: "required"}}}) {
		t.Fatal("validation not mapped")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var body metamodel.ValidationError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.Fields[0].Field != "retention" {
		t.Fatalf("body: %+v %v", body, err)
	}

	rec = httptest.NewRecorder()
	if !respondAssetWriteError(rec, asset.ErrVersionConflict) || rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("conflict status %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	if !respondAssetWriteError(rec, asset.ErrVersionRequired) || rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("required status %d", rec.Code)
	}
	if respondAssetWriteError(httptest.NewRecorder(), asset.ErrAssetNotFound) {
		t.Fatal("unrelated error claimed")
	}
}
