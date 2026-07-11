// Package install downloads external Marmot plugins from an OCI
// registry and caches them on disk, where the plugin loader picks them
// up alongside locally installed plugins.
package install

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// coreManifestJSON pins the core plugin set for this Marmot release. It
// is updated by release tooling when core plugin versions are bumped.
//
//go:embed core_plugins.json
var coreManifestJSON []byte

// Manifest describes a set of plugins and the registry they are
// distributed from.
type Manifest struct {
	// Registry is the OCI registry namespace plugins live under,
	// e.g. ghcr.io/marmotdata/plugins. A plugin's repository is
	// <registry>/<name>.
	Registry string                    `json:"registry"`
	Plugins  map[string]ManifestPlugin `json:"plugins"`
}

// ManifestPlugin pins a single plugin.
type ManifestPlugin struct {
	Version string `json:"version"`
	// Digest is the digest of the plugin's OCI image index for this
	// version (sha256:...). When set, the plugin is resolved by digest
	// so a re-tagged registry artifact cannot change what runs. When
	// empty, the version tag is resolved instead and a warning is
	// logged.
	Digest string `json:"digest"`
}

// CoreManifest returns the core plugin manifest embedded in this build.
func CoreManifest() (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(coreManifestJSON, &m); err != nil {
		return nil, fmt.Errorf("parsing embedded core plugin manifest: %w", err)
	}
	if m.Registry == "" {
		return nil, fmt.Errorf("embedded core plugin manifest has no registry")
	}
	return &m, nil
}
