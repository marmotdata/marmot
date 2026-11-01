package install

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

// Media types of the signed plugin index artefact.
const (
	// IndexArtifactType identifies the plugin index artefact.
	IndexArtifactType = "application/vnd.marmot.plugin.index.v1"
	// IndexMediaType is the layer holding the index JSON.
	IndexMediaType = "application/vnd.marmot.plugin.index.v1+json"
	// IndexSignatureMediaType is the layer holding the index's detached signature.
	IndexSignatureMediaType = "application/vnd.marmot.plugin.index.signature.v1+json"
)

// IndexRepository is the repository under the plugin registry namespace the index is published to.
const IndexRepository = "index"

// DefaultIndexRef returns where the plugin index lives for these options: beside the plugins, so a mirror of the registry namespace carries it too.
func (o Options) DefaultIndexRef() (string, error) {
	manifest, err := CoreManifest()
	if err != nil {
		return "", err
	}
	return o.registry(manifest) + "/" + IndexRepository + ":latest", nil
}

// FetchIndex pulls the plugin index and its detached signature from ref: an OCI reference, or an https:// URL of the index with the signature at the same URL plus ".sig", as plugins.marmotdata.io serves it. Nothing here is trusted: the caller verifies the signature before reading the index.
func FetchIndex(ctx context.Context, opts Options, ref string) (index, signature []byte, err error) {
	if strings.HasPrefix(ref, "https://") || (opts.PlainHTTP && strings.HasPrefix(ref, "http://")) {
		return fetchIndexHTTP(ctx, ref)
	}
	repoRef, tag, err := splitRef(ref)
	if err != nil {
		return nil, nil, err
	}
	repo, err := opts.repository(repoRef)
	if err != nil {
		return nil, nil, err
	}
	return fetchIndexFrom(ctx, repo, ref, tag)
}

// maxIndexSize bounds what is read from an index URL; the real index is well under a megabyte.
const maxIndexSize = 16 << 20

func fetchIndexHTTP(ctx context.Context, url string) (index, signature []byte, err error) {
	client := &http.Client{Timeout: time.Minute}
	get := func(u string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", u, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetching %s: %s", u, resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxIndexSize))
	}
	if index, err = get(url); err != nil {
		return nil, nil, err
	}
	if signature, err = get(url + ".sig"); err != nil {
		return nil, nil, err
	}
	return index, signature, nil
}

// fetchIndexFrom reads the index and signature layers of the index artefact tagged tag in target.
func fetchIndexFrom(ctx context.Context, repo oras.ReadOnlyTarget, ref, tag string) (index, signature []byte, err error) {
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving plugin index %s: %w", ref, err)
	}
	data, err := content.FetchAll(ctx, repo, desc)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching plugin index manifest: %w", err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil, fmt.Errorf("parsing plugin index manifest: %w", err)
	}

	for _, layer := range manifest.Layers {
		switch layer.MediaType {
		case IndexMediaType:
			if index, err = content.FetchAll(ctx, repo, layer); err != nil {
				return nil, nil, fmt.Errorf("fetching plugin index: %w", err)
			}
		case IndexSignatureMediaType:
			if signature, err = content.FetchAll(ctx, repo, layer); err != nil {
				return nil, nil, fmt.Errorf("fetching plugin index signature: %w", err)
			}
		}
	}
	if index == nil || signature == nil {
		return nil, nil, fmt.Errorf("plugin index %s is missing its index or signature layer", ref)
	}
	return index, signature, nil
}

// splitRef splits registry/repo:tag into the repository and the tag, defaulting to latest. A reference by digest keeps its sha256:... as the tag.
func splitRef(ref string) (string, string, error) {
	if ref == "" {
		return "", "", fmt.Errorf("empty plugin index reference")
	}
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		return ref[:at], ref[at+1:], nil
	}
	// A colon after the last slash is a tag; one before it is a registry port.
	if colon := strings.LastIndex(ref, ":"); colon > strings.LastIndex(ref, "/") {
		return ref[:colon], ref[colon+1:], nil
	}
	return ref, "latest", nil
}
