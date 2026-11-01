package install

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog/log"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/marmotdata/marmot/internal/plugin"
)

// Media types for Marmot plugin OCI artifacts.
const (
	// ArtifactType identifies a Marmot plugin artifact.
	ArtifactType = "application/vnd.marmot.plugin.v1"
	// LayerMediaType is the media type of the gzipped plugin binary layer.
	LayerMediaType = "application/vnd.marmot.plugin.v1+gzip"
)

// Options configures plugin installation.
type Options struct {
	// Registry overrides the manifest's registry namespace, e.g. to
	// point at an internal mirror.
	Registry string
	// CacheDir overrides the plugin cache directory.
	CacheDir string
	// PlainHTTP allows non-TLS registries (local registries in tests).
	PlainHTTP bool
	// Pins overrides this release's pin for some plugins, with a version an admin or the auto-update policy moved them to. Nil means every plugin runs the release's pin.
	Pins PinSource
}

// PinSource supplies the version a plugin should run at in place of this release's pin.
type PinSource interface {
	Pin(name string) (ManifestPlugin, bool)
}

// pin returns the version a core plugin should run at: the override when there is one, otherwise the release's pin.
func (o Options) pin(m *Manifest, name string) (ManifestPlugin, bool) {
	if o.Pins != nil {
		if p, ok := o.Pins.Pin(name); ok {
			return p, true
		}
	}
	p, ok := m.Plugins[name]
	return p, ok
}

// RegistryFor returns the registry namespace plugins are pulled from, the configured mirror or else the release's registry.
func (o Options) RegistryFor(m *Manifest) string {
	return o.registry(m)
}

func (o Options) registry(m *Manifest) string {
	if o.Registry != "" {
		return o.Registry
	}
	return m.Registry
}

func (o Options) cacheDir() string {
	if o.CacheDir != "" {
		return o.CacheDir
	}
	return plugin.CacheDir()
}

// CachedPath returns the path a plugin binary is cached at:
// <cacheDir>/<registry>/<name>/<version>/<os>_<arch>/marmot-plugin-<name>.
func CachedPath(cacheDir, registry, name, version string) string {
	// Colons in registry hosts (localhost:5000) are not portable in
	// directory names.
	registryPath := strings.ReplaceAll(registry, ":", "_")
	return filepath.Join(cacheDir, filepath.FromSlash(registryPath), name, version,
		runtime.GOOS+"_"+runtime.GOARCH, pluginBinaryPrefix+name)
}

// EnsureCore installs every core plugin that is not already cached or
// shadowed by a locally installed plugin. Failures are logged per
// plugin and returned aggregated, so an unreachable registry does not
// have to prevent startup.
func EnsureCore(ctx context.Context, opts Options) error {
	manifest, err := CoreManifest()
	if err != nil {
		return err
	}

	var errs []string
	for name := range manifest.Plugins {
		pin, _ := opts.pin(manifest, name)
		if HasLocalOverride(name) {
			log.Debug().Str("plugin", name).Msg("Local plugin overrides core plugin, skipping install")
			continue
		}

		if _, err := EnsurePlugin(ctx, opts, name); err != nil {
			log.Warn().Err(err).Str("plugin", name).Str("version", pin.Version).Msg("Failed to install core plugin")
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("installing core plugins: %s", strings.Join(errs, "; "))
	}
	return nil
}

// EnsurePlugin installs the version the named core plugin should run at if it is not already cached, and returns the path to its binary.
func EnsurePlugin(ctx context.Context, opts Options, name string) (string, error) {
	manifest, err := CoreManifest()
	if err != nil {
		return "", err
	}

	pin, ok := opts.pin(manifest, name)
	if !ok {
		return "", fmt.Errorf("plugin %s is not a known core plugin", name)
	}
	return EnsureVersion(ctx, opts, name, pin)
}

// EnsureVersion installs one version of a plugin, by its digest, if it is not already cached, and returns the path to its binary.
func EnsureVersion(ctx context.Context, opts Options, name string, pin ManifestPlugin) (string, error) {
	manifest, err := CoreManifest()
	if err != nil {
		return "", err
	}

	registry := opts.registry(manifest)
	dest := CachedPath(opts.cacheDir(), registry, name, pin.Version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}

	if err := Install(ctx, opts, registry, name, pin.Version, pin.Digest, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Install pulls a plugin from <registry>/<name> by the digest this release pins and writes its binary for the current platform to dest. A plugin with no pin is refused: resolving a tag would let whoever controls the registry, or a mirror, choose the code that runs.
func Install(ctx context.Context, opts Options, registry, name, version, digest, dest string) error {
	if digest == "" {
		return fmt.Errorf("plugin %s@%s has no digest pin in this release; refusing to install it by tag", name, version)
	}
	repoRef := registry + "/" + name
	repo, err := opts.repository(repoRef)
	if err != nil {
		return err
	}

	log.Info().Str("plugin", name).Str("version", version).Str("repository", repoRef).
		Msg("Installing plugin")

	if err := installFromTarget(ctx, repo, digest, dest); err != nil {
		return fmt.Errorf("installing %s@%s from %s: %w", name, version, repoRef, err)
	}
	return nil
}

// repository opens an OCI repository with the user's Docker credentials, so private mirrors work the way docker pull does.
func (o Options) repository(repoRef string) (*remote.Repository, error) {
	repo, err := remote.NewRepository(repoRef)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin repository %s: %w", repoRef, err)
	}
	repo.PlainHTTP = o.PlainHTTP

	credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err == nil {
		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.NewCache(),
			Credential: credentials.Credential(credStore),
		}
	}
	return repo, nil
}

// installFromTarget resolves ref, follows the multi-platform index to
// the manifest for the current platform, and writes the plugin binary
// layer to dest. Content digests are verified during fetch.
func installFromTarget(ctx context.Context, target oras.ReadOnlyTarget, ref, dest string) error {
	desc, err := target.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", ref, err)
	}

	manifestDesc, err := resolvePlatform(ctx, target, desc)
	if err != nil {
		return err
	}

	manifestData, err := content.FetchAll(ctx, target, manifestDesc)
	if err != nil {
		return fmt.Errorf("fetching plugin manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parsing plugin manifest: %w", err)
	}

	layer, err := pickBinaryLayer(manifest)
	if err != nil {
		return err
	}

	blob, err := content.FetchAll(ctx, target, layer)
	if err != nil {
		return fmt.Errorf("fetching plugin binary: %w", err)
	}

	binary := blob
	if strings.HasSuffix(layer.MediaType, "+gzip") {
		gz, err := gzip.NewReader(bytes.NewReader(blob))
		if err != nil {
			return fmt.Errorf("decompressing plugin binary: %w", err)
		}
		defer gz.Close()
		binary, err = io.ReadAll(gz)
		if err != nil {
			return fmt.Errorf("decompressing plugin binary: %w", err)
		}
	}

	return writeAtomic(dest, binary)
}

// resolvePlatform follows an image index to the manifest matching the
// current OS and architecture. A plain manifest is returned as-is.
func resolvePlatform(ctx context.Context, target oras.ReadOnlyTarget, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex, "application/vnd.docker.distribution.manifest.list.v2+json":
	default:
		return desc, nil
	}

	data, err := content.FetchAll(ctx, target, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("fetching plugin index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("parsing plugin index: %w", err)
	}

	for _, m := range index.Manifests {
		if m.Platform == nil {
			continue
		}
		if m.Platform.OS == runtime.GOOS && m.Platform.Architecture == runtime.GOARCH {
			return m, nil
		}
	}

	return ocispec.Descriptor{}, fmt.Errorf("no plugin build for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// pickBinaryLayer selects the plugin binary layer from a manifest,
// preferring the Marmot plugin media type.
func pickBinaryLayer(manifest ocispec.Manifest) (ocispec.Descriptor, error) {
	for _, layer := range manifest.Layers {
		if layer.MediaType == LayerMediaType || layer.MediaType == ArtifactType {
			return layer, nil
		}
	}
	if len(manifest.Layers) == 1 {
		return manifest.Layers[0], nil
	}
	return ocispec.Descriptor{}, fmt.Errorf("no plugin binary layer in manifest (want media type %s)", LayerMediaType)
}

// writeAtomic writes the binary to dest, executable, via a temp file
// and rename so a concurrent reader never observes a partial binary.
func writeAtomic(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating plugin cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".marmot-plugin-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing plugin binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("marking plugin binary executable: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing plugin binary: %w", err)
	}

	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("moving plugin binary into cache: %w", err)
	}
	return nil
}

// HasLocalOverride reports whether a locally installed plugin with
// this name exists; local plugins take precedence over downloaded ones,
// so there is nothing to install.
func HasLocalOverride(name string) bool {
	info, err := os.Stat(filepath.Join(plugin.PluginsDir(), pluginBinaryPrefix+name))
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
