package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type doc struct {
	Name  string   `json:"name"`
	Items []string `json:"items,omitempty"`
}

func TestDecodeDocumentJSONAndYAMLAgree(t *testing.T) {
	var fromJSON, fromYAML doc
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"viewer","items":["a","b"]}`))
	require.NoError(t, DecodeDocument(req, &fromJSON))

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name: viewer\nitems:\n  - a\n  - b\n"))
	req.Header.Set("Content-Type", "application/yaml")
	require.NoError(t, DecodeDocument(req, &fromYAML))

	assert.Equal(t, fromJSON, fromYAML)
}

func TestDecodeDocumentRefusesUnknownFields(t *testing.T) {
	var d doc
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name: viewer\nitem: a\n"))
	req.Header.Set("Content-Type", "application/x-yaml; charset=utf-8")
	err := DecodeDocument(req, &d)
	assert.True(t, errors.Is(err, ErrDocumentInvalid))
	assert.Contains(t, err.Error(), "item")
}

func TestDecodeDocumentRefusesTrailingContent(t *testing.T) {
	var d doc
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a"}{"name":"b"}`))
	assert.True(t, errors.Is(DecodeDocument(req, &d), ErrDocumentInvalid))
}

func TestRespondDocumentHonoursFormat(t *testing.T) {
	for _, tc := range []struct {
		target, accept, wantType, wantBody string
	}{
		{"/", "", "application/json", `"name":"viewer"`},
		{"/?format=yaml", "", "application/yaml", "name: viewer"},
		{"/", "application/yaml", "application/yaml", "name: viewer"},
		{"/?format=json", "application/yaml", "application/json", `"name":"viewer"`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.target, nil)
		if tc.accept != "" {
			req.Header.Set("Accept", tc.accept)
		}
		rec := httptest.NewRecorder()
		RespondDocument(rec, req, http.StatusOK, doc{Name: "viewer"})
		assert.Equal(t, tc.wantType, rec.Header().Get("Content-Type"), tc.target)
		assert.Contains(t, rec.Body.String(), tc.wantBody, tc.target)
	}
}

type ordered struct {
	Role    string   `json:"role"`
	Members []string `json:"members"`
	Note    string   `json:"note,omitempty"`
}

func TestMarshalDocumentYAMLKeepsFieldOrderAndRoundTrips(t *testing.T) {
	in := []ordered{{Role: "asset.viewer", Members: []string{"user:alice", "allAuthenticated"}, Note: "yes: no"}}
	out, err := MarshalDocumentYAML(in)
	require.NoError(t, err)
	text := string(out)
	assert.Less(t, strings.Index(text, "role:"), strings.Index(text, "members:"), "fields keep their declared order:\n%s", text)
	assert.NotContains(t, text, "[", "block style, not flow")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(text))
	req.Header.Set("Content-Type", "application/yaml")
	var back []ordered
	require.NoError(t, DecodeDocument(req, &back))
	assert.Equal(t, in, back, "a value that needs quoting survives the round trip")
}
