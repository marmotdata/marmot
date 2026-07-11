package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFakeBinary(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755))
}

func testManifest(plugins map[string]ManifestPlugin) *Manifest {
	return &Manifest{Registry: "ghcr.io/example/plugins", Plugins: plugins}
}

func TestPinnedPathsReturnsCachedPinnedBinary(t *testing.T) {
	opts := Options{CacheDir: t.TempDir()}
	manifest := testManifest(map[string]ManifestPlugin{"gcs": {Version: "1.2.0"}})

	pinned := CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.2.0")
	writeFakeBinary(t, pinned)

	assert.Equal(t, []string{pinned}, pinnedPaths(opts, manifest))
}

func TestPinnedPathsSkipsUncachedPlugins(t *testing.T) {
	opts := Options{CacheDir: t.TempDir()}
	manifest := testManifest(map[string]ManifestPlugin{
		"gcs": {Version: "1.2.0"},
		"s3":  {Version: "2.0.0"},
	})

	pinned := CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.2.0")
	writeFakeBinary(t, pinned)

	assert.Equal(t, []string{pinned}, pinnedPaths(opts, manifest))
}

func TestPinnedPathsIgnoresOtherCachedVersions(t *testing.T) {
	opts := Options{CacheDir: t.TempDir()}
	manifest := testManifest(map[string]ManifestPlugin{"gcs": {Version: "1.2.0"}})

	// A lexically earlier version left behind by another binary sharing
	// the cache must not be picked up.
	writeFakeBinary(t, CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.10.0"))
	pinned := CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.2.0")
	writeFakeBinary(t, pinned)

	assert.Equal(t, []string{pinned}, pinnedPaths(opts, manifest))
}

func TestPinnedPathsUsesRegistryOverride(t *testing.T) {
	opts := Options{CacheDir: t.TempDir(), Registry: "mirror.internal/plugins"}
	manifest := testManifest(map[string]ManifestPlugin{"gcs": {Version: "1.2.0"}})

	// Only the binary cached under the manifest's default registry
	// exists; with an override in effect it must not be loaded.
	writeFakeBinary(t, CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.2.0"))

	assert.Empty(t, pinnedPaths(opts, manifest))

	pinned := CachedPath(opts.CacheDir, opts.Registry, "gcs", "1.2.0")
	writeFakeBinary(t, pinned)

	assert.Equal(t, []string{pinned}, pinnedPaths(opts, manifest))
}

func TestPinnedPathsSkipsNonExecutableFile(t *testing.T) {
	opts := Options{CacheDir: t.TempDir()}
	manifest := testManifest(map[string]ManifestPlugin{"gcs": {Version: "1.2.0"}})

	pinned := CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.2.0")
	require.NoError(t, os.MkdirAll(filepath.Dir(pinned), 0o755))
	require.NoError(t, os.WriteFile(pinned, []byte("not executable"), 0o644))

	assert.Empty(t, pinnedPaths(opts, manifest))
}

func TestLoadLocalPluginsMissingDirIsNotAnError(t *testing.T) {
	require.NoError(t, loadLocalPlugins(filepath.Join(t.TempDir(), "does-not-exist")))
}

func TestPinnedPathsSortsByPluginName(t *testing.T) {
	opts := Options{CacheDir: t.TempDir()}
	manifest := testManifest(map[string]ManifestPlugin{
		"s3":  {Version: "2.0.0"},
		"gcs": {Version: "1.2.0"},
	})

	gcs := CachedPath(opts.CacheDir, manifest.Registry, "gcs", "1.2.0")
	s3 := CachedPath(opts.CacheDir, manifest.Registry, "s3", "2.0.0")
	writeFakeBinary(t, gcs)
	writeFakeBinary(t, s3)

	assert.Equal(t, []string{gcs, s3}, pinnedPaths(opts, manifest))
}
