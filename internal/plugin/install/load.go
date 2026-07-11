package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marmotdata/marmot/internal/plugin"
)

// pluginBinaryPrefix is the filename prefix a plugin binary must have
// to be picked up, e.g. marmot-plugin-gcs.
const pluginBinaryPrefix = "marmot-plugin-"

// LoadPlugins registers locally installed plugin binaries, then the
// manifest-pinned version of each core plugin found in the cache.
// Locally installed plugins load first, so they shadow cached ones.
//
// Cached versions other than the pin are ignored: several Marmot
// binaries with different embedded manifests can share one cache and
// each still loads exactly the versions it pins.
func LoadPlugins(opts Options) error {
	if err := loadLocalPlugins(plugin.PluginsDir()); err != nil {
		return err
	}

	manifest, err := CoreManifest()
	if err != nil {
		return err
	}

	for _, path := range pinnedPaths(opts, manifest) {
		plugin.LoadBinary(path)
	}
	return nil
}

// loadLocalPlugins registers every plugin binary in dir, the directory
// for plugins the user installed by hand. A missing directory is not an
// error: it simply means no plugins are installed.
func loadLocalPlugins(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading plugins directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), pluginBinaryPrefix) {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.Mode()&0o111 == 0 {
			continue
		}

		plugin.LoadBinary(filepath.Join(dir, entry.Name()))
	}

	return nil
}

// pinnedPaths returns the cache paths of the manifest-pinned plugin
// binaries that exist on disk and are executable, sorted by plugin name.
// A missing binary simply means the plugin was never installed (or
// installation failed and was warned about already).
func pinnedPaths(opts Options, manifest *Manifest) []string {
	var paths []string
	for name, pin := range manifest.Plugins {
		path := CachedPath(opts.cacheDir(), opts.registry(manifest), name, pin.Version)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
