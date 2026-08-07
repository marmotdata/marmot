// Package release publishes a Marmot plugin to an OCI registry. It
// packs per-platform binaries under a multi-arch index and attaches an
// info referrer carrying the plugin's README, metadata, and asset
// schemas. Callers pass built binaries in; this package does not build.
package release

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Platform is one OS+arch the plugin ships for.
type Platform struct {
	OS   string
	Arch string
}

// DefaultPlatforms lists the platforms core plugins currently release
// for. Treat as read-only; mutation affects every caller in the process.
var DefaultPlatforms = []Platform{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

// Options configures a plugin release push.
type Options struct {
	// Ref is a fully qualified OCI reference including tag, e.g.
	// ghcr.io/marmotdata/plugins/postgresql:0.1.0.
	Ref string
	// Source is the plugin's source directory.
	Source string
	// Dist is the directory containing pre-built binaries.
	Dist string
	// RepoURL sets the org.opencontainers.image.source annotation.
	RepoURL string
	// Platforms overrides DefaultPlatforms when non-nil.
	Platforms []Platform
	// PlainHTTP allows non-TLS registries for local testing.
	PlainHTTP bool
}

// Result reports the digests of the pushed index and info referrer.
type Result struct {
	IndexDigest string
	InfoDigest  string
}

// Push publishes the release. Callers sign the returned digests
// separately if required.
func Push(ctx context.Context, opts Options) (Result, error) {
	if opts.Source == "" {
		return Result{}, fmt.Errorf("source is required")
	}
	if opts.Dist == "" {
		return Result{}, fmt.Errorf("dist is required")
	}
	ref, err := registry.ParseReference(opts.Ref)
	if err != nil {
		return Result{}, fmt.Errorf("parsing ref %q: %w", opts.Ref, err)
	}
	if ref.Reference == "" {
		return Result{}, fmt.Errorf("ref %q must include a tag", opts.Ref)
	}

	version := ref.Reference
	name := pluginName(ref)
	platforms := opts.Platforms
	if len(platforms) == 0 {
		platforms = DefaultPlatforms
	}

	repo, err := newRepository(ref, opts.PlainHTTP)
	if err != nil {
		return Result{}, err
	}

	annotations := map[string]string{versionAnnotation: version}
	if opts.RepoURL != "" {
		annotations[sourceAnnotation] = opts.RepoURL
	}

	platformManifests := make([]ocispec.Descriptor, 0, len(platforms))
	for _, plat := range platforms {
		binary, err := findBinary(opts.Dist, name, plat)
		if err != nil {
			return Result{}, err
		}
		desc, err := pushPlatformManifest(ctx, repo, name, version, plat, binary, annotations)
		if err != nil {
			return Result{}, fmt.Errorf("push %s/%s: %w", plat.OS, plat.Arch, err)
		}
		platformManifests = append(platformManifests, desc)
	}

	indexDesc, err := pushIndex(ctx, repo, version, platformManifests, annotations)
	if err != nil {
		return Result{}, fmt.Errorf("push index: %w", err)
	}

	bundle, err := buildInfoBundle(ctx, name, opts.Source, opts.Dist)
	if err != nil {
		return Result{}, fmt.Errorf("build info bundle: %w", err)
	}
	infoDesc, err := pushInfoReferrer(ctx, repo, indexDesc, bundle, annotations)
	if err != nil {
		return Result{}, fmt.Errorf("attach info: %w", err)
	}

	return Result{
		IndexDigest: indexDesc.Digest.String(),
		InfoDigest:  infoDesc.Digest.String(),
	}, nil
}

func newRepository(ref registry.Reference, plainHTTP bool) (*remote.Repository, error) {
	repoRef := ref.Registry + "/" + ref.Repository
	repo, err := remote.NewRepository(repoRef)
	if err != nil {
		return nil, fmt.Errorf("invalid repository %s: %w", repoRef, err)
	}
	repo.PlainHTTP = plainHTTP

	// Auth via the standard Docker credentials store; anonymous otherwise.
	if credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{}); err == nil {
		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.NewCache(),
			Credential: credentials.Credential(credStore),
		}
	}
	return repo, nil
}

func pushPlatformManifest(
	ctx context.Context,
	repo *remote.Repository,
	name, version string,
	plat Platform,
	binaryPath string,
	baseAnnotations map[string]string,
) (ocispec.Descriptor, error) {
	gz, err := gzipFile(binaryPath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	layer, err := pushBlob(ctx, repo, gz, MediaTypePluginGzip, map[string]string{
		titleAnnotation: fmt.Sprintf("marmot-plugin-%s.gz", name),
	})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("push binary blob: %w", err)
	}

	manDesc, err := oras.PackManifest(ctx, repo, oras.PackManifestVersion1_1, ArtifactTypePlugin, oras.PackManifestOptions{
		Layers:              []ocispec.Descriptor{layer},
		ManifestAnnotations: baseAnnotations,
	})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pack manifest: %w", err)
	}

	tag := fmt.Sprintf("%s-%s-%s", version, plat.OS, plat.Arch)
	if err := repo.Tag(ctx, manDesc, tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("tag %s: %w", tag, err)
	}

	manDesc.Platform = &ocispec.Platform{OS: plat.OS, Architecture: plat.Arch}
	return manDesc, nil
}

func pushIndex(
	ctx context.Context,
	repo *remote.Repository,
	tag string,
	manifests []ocispec.Descriptor,
	annotations map[string]string,
) (ocispec.Descriptor, error) {
	index := ocispec.Index{
		Versioned:   specs.Versioned{SchemaVersion: 2},
		MediaType:   ocispec.MediaTypeImageIndex,
		Manifests:   manifests,
		Annotations: annotations,
	}
	data, err := json.Marshal(index)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	desc := ocispec.Descriptor{
		MediaType:   ocispec.MediaTypeImageIndex,
		Digest:      digest.FromBytes(data),
		Size:        int64(len(data)),
		Annotations: annotations,
	}
	if err := repo.Manifests().PushReference(ctx, desc, bytes.NewReader(data), tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("push index: %w", err)
	}
	return desc, nil
}

func pushInfoReferrer(
	ctx context.Context,
	repo *remote.Repository,
	subject ocispec.Descriptor,
	bundle infoBundle,
	baseAnnotations map[string]string,
) (ocispec.Descriptor, error) {
	layers := make([]ocispec.Descriptor, 0, 3)
	add := func(data []byte, mediaType, filename string) error {
		if len(data) == 0 {
			return nil
		}
		desc, err := pushBlob(ctx, repo, data, mediaType, map[string]string{titleAnnotation: filename})
		if err != nil {
			return err
		}
		layers = append(layers, desc)
		return nil
	}
	if err := add(bundle.Readme, MediaTypeReadme, "README.md"); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("push readme: %w", err)
	}
	if err := add(bundle.Metadata, MediaTypeMetadata, "metadata.json"); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("push metadata: %w", err)
	}
	if err := add(bundle.AssetSchemas, MediaTypeAssetSchemas, "asset_schemas.json"); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("push asset schemas: %w", err)
	}
	if len(layers) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("info bundle is empty")
	}

	return oras.PackManifest(ctx, repo, oras.PackManifestVersion1_1, ArtifactTypePluginInfo, oras.PackManifestOptions{
		Subject:             &subject,
		Layers:              layers,
		ManifestAnnotations: baseAnnotations,
	})
}

func pushBlob(ctx context.Context, repo *remote.Repository, data []byte, mediaType string, annotations map[string]string) (ocispec.Descriptor, error) {
	desc := ocispec.Descriptor{
		MediaType:   mediaType,
		Digest:      digest.FromBytes(data),
		Size:        int64(len(data)),
		Annotations: annotations,
	}
	if err := repo.Blobs().Push(ctx, desc, bytes.NewReader(data)); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// findBinary locates a binary under dist. Matches GoReleaser's
// "..._<os>_<arch>[_v1]/marmot-plugin-<name>" layout by looking for the
// "<os>_<arch>" segment on the path.
func findBinary(dist, name string, plat Platform) (string, error) {
	target := fmt.Sprintf("marmot-plugin-%s", name)
	segment := fmt.Sprintf("_%s_%s", plat.OS, plat.Arch)

	var found string
	err := filepath.WalkDir(dist, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Name() != target || !strings.Contains(path, segment) {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", fmt.Errorf("search %s: %w", dist, err)
	}
	if found == "" {
		return "", fmt.Errorf("no binary for %s/%s under %s", plat.OS, plat.Arch, dist)
	}
	return found, nil
}

func gzipFile(path string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := gz.Write(src); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pluginName returns the last path segment of a repository reference.
func pluginName(ref registry.Reference) string {
	repo := ref.Repository
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return repo
}
