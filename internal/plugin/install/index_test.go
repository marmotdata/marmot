package install

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
)

func TestSplitRef(t *testing.T) {
	cases := map[string][2]string{
		"ghcr.io/marmotdata/plugins/index:latest":    {"ghcr.io/marmotdata/plugins/index", "latest"},
		"localhost:5000/plugins/index":               {"localhost:5000/plugins/index", "latest"},
		"localhost:5000/plugins/index:v2":            {"localhost:5000/plugins/index", "v2"},
		"ghcr.io/marmotdata/plugins/index@sha256:ab": {"ghcr.io/marmotdata/plugins/index", "sha256:ab"},
	}
	for ref, want := range cases {
		repo, tag, err := splitRef(ref)
		if err != nil {
			t.Fatalf("splitRef(%q): %v", ref, err)
		}
		if repo != want[0] || tag != want[1] {
			t.Errorf("splitRef(%q) = %q, %q, want %q, %q", ref, repo, tag, want[0], want[1])
		}
	}
}

func TestFetchIndexReadsBothLayers(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	body := []byte(`{"version": 3}`)
	sig := []byte(`{"key_id": "k"}`)

	manifest := ocispec.Manifest{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: IndexArtifactType,
		Config:       push(t, store, ocispec.MediaTypeEmptyJSON, []byte("{}")),
		Layers: []ocispec.Descriptor{
			push(t, store, IndexMediaType, body),
			push(t, store, IndexSignatureMediaType, sig),
		},
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	desc := push(t, store, ocispec.MediaTypeImageManifest, data)
	require.NoError(t, store.Tag(ctx, desc, "latest"))

	gotBody, gotSig, err := fetchIndexFrom(ctx, store, "test/index", "latest")
	require.NoError(t, err)
	assert.Equal(t, body, gotBody)
	assert.Equal(t, sig, gotSig)

	unsigned := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    manifest.Config,
		Layers:    manifest.Layers[:1],
	}
	data, err = json.Marshal(unsigned)
	require.NoError(t, err)
	desc = push(t, store, ocispec.MediaTypeImageManifest, data)
	require.NoError(t, store.Tag(ctx, desc, "unsigned"))
	_, _, err = fetchIndexFrom(ctx, store, "test/index", "unsigned")
	assert.ErrorContains(t, err, "missing its index or signature")
}

func TestFetchIndexOverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write([]byte(`{"version": 3}`))
		case "/index.json.sig":
			_, _ = w.Write([]byte(`{"key_id": "k"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	body, sig, err := FetchIndex(context.Background(), Options{PlainHTTP: true}, srv.URL+"/index.json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"version": 3}`, string(body))
	assert.JSONEq(t, `{"key_id": "k"}`, string(sig))

	_, _, err = FetchIndex(context.Background(), Options{PlainHTTP: true}, srv.URL+"/missing.json")
	assert.ErrorContains(t, err, "404")
}
