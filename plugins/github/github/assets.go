package github

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

const (
	provider  = "GitHub"
	assetType = "Repository"
	// maxReadmeBytes caps the README kept as documentation.
	maxReadmeBytes = 128 << 10
	// maxCodeownersBytes caps the CODEOWNERS file parsed; GitHub itself
	// ignores one over 3 MB, and a catch-all rule sits near the top.
	maxCodeownersBytes = 256 << 10
)

// codeownersPaths are where GitHub looks for CODEOWNERS, in its order.
var codeownersPaths = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

func describeRepository(ctx context.Context, c *client, config *Config, r *repository) (pluginsdk.Asset, *pluginsdk.Documentation) {
	visibility := r.Visibility
	if visibility == "" {
		visibility = "public"
		if r.Private {
			visibility = "private"
		}
	}
	metadata := map[string]any{
		"owner":          config.Owner,
		"repository":     r.Name,
		"full_name":      config.Owner + "/" + r.Name,
		"default_branch": r.DefaultBranch,
		"visibility":     visibility,
		"archived":       r.Archived,
		"fork":           r.Fork,
		"url":            r.HTMLURL,
		"stars":          r.StargazersCount,
		"forks":          r.ForksCount,
		"open_issues":    r.OpenIssuesCount,
	}
	if r.Language != "" {
		metadata["language"] = r.Language
	}
	if len(r.Topics) > 0 {
		metadata["topics"] = slices.Clone(r.Topics)
	}
	if r.License != nil && r.License.SPDXID != "" && r.License.SPDXID != "NOASSERTION" {
		metadata["license"] = r.License.SPDXID
	}
	if r.CreatedAt != nil {
		metadata["created_at"] = r.CreatedAt.UTC().Format(time.RFC3339)
	}
	if r.PushedAt != nil {
		metadata["pushed_at"] = r.PushedAt.UTC().Format(time.RFC3339)
	}
	if config.IncludeOwners {
		if owners := readCodeowners(ctx, c, config.Owner, r); len(owners) > 0 {
			metadata["owners"] = owners
		}
	}

	name := config.Owner + "/" + r.Name
	mrn := config.AssetMRN(assetType, provider, config.Owner, r.Name)
	tags := pluginsdk.InterpolateTags(config.Tags, metadata)
	for _, t := range r.Topics {
		if !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrn,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      tags,
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if r.Description != "" {
		desc := r.Description
		asset.Description = &desc
	}
	if r.HTMLURL != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "GitHub", Icon: "github", URL: r.HTMLURL}}
	}

	if !config.IncludeReadme {
		return asset, nil
	}
	readme, ok := readText(ctx, c, repoPath(config.Owner, r.Name)+"/readme", maxReadmeBytes)
	if !ok {
		return asset, nil
	}
	return asset, &pluginsdk.Documentation{MRN: mrn, Content: readme, Source: provider}
}

// readText fetches a file through the contents API and returns it as text,
// cut to max bytes. A missing, binary or unreadable file is skipped: none
// of it is worth failing discovery over.
func readText(ctx context.Context, c *client, path string, max int) (string, bool) {
	var f contentFile
	if err := c.do(ctx, "GET", path, nil, nil, &f); err != nil {
		if !isNotFound(err) {
			log.Warn().Err(err).Str("path", path).Msg("Failed to read file")
		}
		return "", false
	}
	if f.Type != "file" || f.Encoding != "base64" {
		return "", false
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err != nil || isBinary(data) {
		return "", false
	}
	return capText(string(data), max), true
}

func readCodeowners(ctx context.Context, c *client, owner string, r *repository) []string {
	for _, p := range codeownersPaths {
		text, ok := readText(ctx, c, repoPath(owner, r.Name)+"/contents/"+escapePath(p), maxCodeownersBytes)
		if ok {
			return defaultOwners(text)
		}
	}
	return nil
}

// defaultOwners returns the owners of the last catch-all rule, which is
// who GitHub asks to review a file no later rule claims. Per-path owners
// say nothing about the repository as a whole, so they are not kept.
func defaultOwners(text string) []string {
	var owners []string
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, " #"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if fields[0] == "*" || fields[0] == "**" || fields[0] == "/**" {
			owners = fields[1:]
		}
	}
	return owners
}

// isBinary treats text with NUL bytes, or that is not UTF-8, as binary,
// which is what git's own heuristic amounts to for the files that matter.
func isBinary(data []byte) bool {
	return slices.Contains(data, 0) || !utf8.Valid(data)
}
