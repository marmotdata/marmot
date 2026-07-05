package install

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/errdef"
)

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(data)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func push(t *testing.T, store *memory.Store, mediaType string, data []byte) ocispec.Descriptor {
	t.Helper()
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
	err := store.Push(context.Background(), desc, bytes.NewReader(data))
	if !errors.Is(err, errdef.ErrAlreadyExists) {
		require.NoError(t, err)
	}
	return desc
}

// pushPluginArtifact assembles a plugin artifact for the given platform
// in the store: gzipped binary layer -> manifest, returning the
// manifest descriptor with platform set, as an index entry would carry.
func pushPluginArtifact(t *testing.T, store *memory.Store, os, arch string, binary []byte) ocispec.Descriptor {
	t.Helper()

	layerDesc := push(t, store, LayerMediaType, gzipBytes(t, binary))
	configDesc := push(t, store, ocispec.MediaTypeEmptyJSON, []byte("{}"))

	manifest := ocispec.Manifest{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: ArtifactType,
		Config:       configDesc,
		Layers:       []ocispec.Descriptor{layerDesc},
	}
	manifestData, err := json.Marshal(manifest)
	require.NoError(t, err)

	desc := push(t, store, ocispec.MediaTypeImageManifest, manifestData)
	desc.Platform = &ocispec.Platform{OS: os, Architecture: arch}
	return desc
}

func pushIndex(t *testing.T, store *memory.Store, tag string, manifests ...ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()

	index := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: manifests,
	}
	indexData, err := json.Marshal(index)
	require.NoError(t, err)

	desc := push(t, store, ocispec.MediaTypeImageIndex, indexData)
	require.NoError(t, store.Tag(context.Background(), desc, tag))
	return desc
}

func TestInstallFromIndexSelectsCurrentPlatform(t *testing.T) {
	store := memory.New()
	binary := []byte("#!/bin/sh\necho current platform\n")

	current := pushPluginArtifact(t, store, runtime.GOOS, runtime.GOARCH, binary)
	other := pushPluginArtifact(t, store, "plan9", "mips", []byte("wrong binary"))
	pushIndex(t, store, "1.2.3", other, current)

	dest := filepath.Join(t.TempDir(), "marmot-plugin-test")
	require.NoError(t, installFromTarget(context.Background(), store, "1.2.3", dest))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, binary, got)

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "binary should be executable")
}

func TestInstallByIndexDigest(t *testing.T) {
	store := memory.New()
	binary := []byte("pinned binary")

	current := pushPluginArtifact(t, store, runtime.GOOS, runtime.GOARCH, binary)
	indexDesc := pushIndex(t, store, "1.0.0", current)
	// remote registries resolve digest references natively; the memory
	// store only resolves tags, so register the digest as one.
	require.NoError(t, store.Tag(context.Background(), indexDesc, indexDesc.Digest.String()))

	dest := filepath.Join(t.TempDir(), "marmot-plugin-test")
	require.NoError(t, installFromTarget(context.Background(), store, indexDesc.Digest.String(), dest))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, binary, got)
}

func TestInstallFailsWithoutPlatformBuild(t *testing.T) {
	store := memory.New()

	other := pushPluginArtifact(t, store, "plan9", "mips", []byte("wrong binary"))
	pushIndex(t, store, "1.0.0", other)

	dest := filepath.Join(t.TempDir(), "marmot-plugin-test")
	err := installFromTarget(context.Background(), store, "1.0.0", dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no plugin build for")
	assert.NoFileExists(t, dest)
}

func TestInstallSinglePlatformManifest(t *testing.T) {
	store := memory.New()
	binary := []byte("single platform binary")

	desc := pushPluginArtifact(t, store, runtime.GOOS, runtime.GOARCH, binary)
	require.NoError(t, store.Tag(context.Background(), desc, "1.0.0"))

	dest := filepath.Join(t.TempDir(), "marmot-plugin-test")
	require.NoError(t, installFromTarget(context.Background(), store, "1.0.0", dest))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, binary, got)
}

func TestCachedPathLayout(t *testing.T) {
	path := CachedPath("/cache", "ghcr.io/marmotdata/plugins", "gcs", "0.1.0")
	expected := filepath.Join("/cache", "ghcr.io", "marmotdata", "plugins", "gcs", "0.1.0",
		runtime.GOOS+"_"+runtime.GOARCH, "marmot-plugin-gcs")
	assert.Equal(t, expected, path)
}

func TestCachedPathSanitizesRegistryPort(t *testing.T) {
	path := CachedPath("/cache", "localhost:5000/plugins", "gcs", "0.1.0")
	assert.NotContains(t, path, ":")
}

func TestCoreManifestParses(t *testing.T) {
	manifest, err := CoreManifest()
	require.NoError(t, err)

	assert.Equal(t, "ghcr.io/marmotdata/plugins", manifest.Registry)
	require.Contains(t, manifest.Plugins, "gcs")
	assert.NotEmpty(t, manifest.Plugins["gcs"].Version)
}
