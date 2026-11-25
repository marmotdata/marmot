// Package github catalogues the repositories of a GitHub organisation or
// user and offers a small set of typed reads and pull request tools.
package github

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// tokenEnv is read when the config carries no token, so a runner can keep
// the token in its environment and out of the pipeline file.
const tokenEnv = "GITHUB_TOKEN"

// maxRepositories bounds one discovery run, well above any owner the
// plugin is meant for, so a misconfigured owner cannot page forever.
const maxRepositories = 10000

// Config for the GitHub plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Owner string `json:"owner" label:"Organisation" description:"GitHub organisation or user, for example acme" validate:"required"`
	Token string `json:"token,omitempty" label:"Access Token" description:"Fine-grained personal access token. Defaults to $GITHUB_TOKEN" sensitive:"true"`

	Repositories    []string `json:"repositories,omitempty" group:"Discovery" description:"Only these repositories. Empty means all"`
	Topics          []string `json:"topics,omitempty" group:"Discovery" description:"Only repositories with one of these topics"`
	IncludePrivate  bool     `json:"include_private" group:"Discovery" description:"Include private and internal repositories" default:"false"`
	IncludeArchived bool     `json:"include_archived" group:"Discovery" description:"Include archived repositories" default:"false"`
	IncludeReadme   bool     `json:"include_readme" group:"Discovery" label:"Include README" description:"Attach each repository's README as documentation" default:"true"`
	IncludeOwners   bool     `json:"include_codeowners" group:"Discovery" label:"Include CODEOWNERS" description:"Set owners from CODEOWNERS" default:"false"`

	IncludeBranches     bool `json:"include_branches" group:"Discovery" description:"Discover every branch of each repository" default:"true"`
	IncludePullRequests bool `json:"include_pull_requests" group:"Discovery" description:"Discover open pull requests" default:"true"`
	IncludeIssues       bool `json:"include_issues" group:"Discovery" description:"Discover open issues" default:"true"`

	BaseURL string `json:"base_url,omitempty" group:"Advanced" label:"API URL" description:"Defaults to https://api.github.com. For GitHub Enterprise Server, https://<host>/api/v3"`

	Tools ToolsConfig `json:"tools,omitempty" group:"Tools" description:"Limits on the changes tools may make. Tools run only in a runner that allows them"`
}

// Example configuration for the plugin
var _ = `
owner: "acme"
token: "${GITHUB_TOKEN}"
topics:
  - "data"
include_private: true
tags:
  - "github"
`

var (
	// ownerPattern and repoPattern are GitHub's own name rules, so a value
	// that passes them can only name an account or repository.
	ownerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	repoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	topicPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
)

func validRepoName(name string) bool {
	return repoPattern.MatchString(name) && name != "." && name != ".."
}

// Validate checks the config without calling GitHub.
func (c *Config) Validate() error {
	if !ownerPattern.MatchString(c.Owner) {
		return fmt.Errorf("owner %q is not a GitHub organisation or user name", c.Owner)
	}
	if _, err := parseBaseURL(c.BaseURL); err != nil {
		return err
	}
	for _, r := range c.Repositories {
		if !validRepoName(r) {
			return fmt.Errorf("repositories: %q is not a repository name", r)
		}
	}
	for _, t := range c.Topics {
		if !topicPattern.MatchString(t) {
			return fmt.Errorf("topics: %q is not a GitHub topic", t)
		}
	}
	if c.Filter != nil {
		for _, p := range append(slices.Clone(c.Filter.Include), c.Filter.Exclude...) {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("filter: %q does not compile: %w", p, err)
			}
		}
	}
	if err := c.Tools.Validate(); err != nil {
		return err
	}
	// Writes are narrower than reads, never wider.
	if len(c.Repositories) > 0 {
		for _, r := range c.Tools.Repos {
			if !slices.Contains(c.Repositories, r) {
				return fmt.Errorf("tools.repos: %q is not in repositories, so it is not catalogued or readable", r)
			}
		}
	}
	return nil
}

func (c *Config) token() string {
	if c.Token != "" {
		return c.Token
	}
	return os.Getenv(tokenEnv)
}

// inScope reports whether the config allows a repository: catalogued,
// readable and, for those in tools.repos, writable. Discovery and every
// tool go through it, so a read can never see a repository the catalogue
// was told to leave out.
func (c *Config) inScope(r *repository) bool {
	if !strings.EqualFold(r.Owner.Login, c.Owner) {
		return false
	}
	if len(c.Repositories) > 0 && !slices.Contains(c.Repositories, r.Name) {
		return false
	}
	if !r.isPublic() && !c.IncludePrivate {
		return false
	}
	if r.Archived && !c.IncludeArchived {
		return false
	}
	if r.Disabled {
		return false
	}
	if len(c.Topics) > 0 && !slices.ContainsFunc(r.Topics, func(t string) bool { return slices.Contains(c.Topics, t) }) {
		return false
	}
	return c.filterAllows(c.Owner + "/" + r.Name)
}

// filterAllows applies the name filter Marmot itself applies to discovered
// assets, so the reads honour it too.
func (c *Config) filterAllows(name string) bool {
	if c.Filter == nil {
		return true
	}
	matches := func(patterns []string) bool {
		return slices.ContainsFunc(patterns, func(p string) bool {
			re, err := regexp.Compile(p)
			return err == nil && re.MatchString(name)
		})
	}
	if len(c.Filter.Include) > 0 && !matches(c.Filter.Include) {
		return false
	}
	return !matches(c.Filter.Exclude)
}

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:            "github",
		MarmotVersion: ">=0.13.0",
		Name:          "GitHub",
		Description:   "Discover repositories, branches, pull requests and issues from a GitHub organisation or user",
		Icon:          "github",
		Category:      "source-control",
		Status:        "experimental",
		Features:      []string{"Assets", "Lineage", "Documentation"},
		// GitHub asks for requests one at a time, and each repository costs
		// several, so even a small organisation outlasts the default test.
		ConnectionTestTimeoutSeconds: 120,
		ConfigSpec:                   pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(RepositoryFields{}, assetType,
				"GitHub repository metadata fields"),
			pluginsdk.AssetSchemaOf(BranchFields{}, branchType,
				"GitHub branch metadata fields"),
			pluginsdk.AssetSchemaOf(PullRequestFields{}, pullRequestType,
				"GitHub pull request metadata fields"),
			pluginsdk.AssetSchemaOf(IssueFields{}, issueType,
				"GitHub issue metadata fields"),
		},
	}
}

// Source implements the GitHub plugin.
type Source struct{}

func loadConfig(rawConfig pluginsdk.RawConfig) (*Config, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)
	return config, nil
}

// Validate validates the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := loadConfig(rawConfig)
	if err != nil {
		return nil, err
	}
	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return rawConfig, nil
}

// Discover catalogues the owner's repositories and, inside each, its
// branches, open pull requests and open issues.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := loadConfig(rawConfig)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	c, err := newClient(config.BaseURL, config.token(), "marmot-plugin-github")
	if err != nil {
		return nil, err
	}

	repos, err := listRepositories(ctx, c, config)
	if err != nil {
		return nil, err
	}

	var inScope []*repository
	for i := range repos {
		if config.inScope(&repos[i]) {
			inScope = append(inScope, &repos[i])
		}
	}
	details := fetchDetails(ctx, c, config, inScope)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := &pluginsdk.DiscoveryResult{}
	for _, r := range inScope {
		d := details[r.Name]
		asset, doc := describeRepository(ctx, c, config, r, d)
		result.Assets = append(result.Assets, asset)
		if doc != nil {
			result.Documentation = append(result.Documentation, *doc)
		}
		describeChildren(ctx, c, config, r, d, *asset.MRN, result)
	}

	log.Info().
		Int("assets", len(result.Assets)).
		Int("lineage", len(result.Lineage)).
		Int("documentation", len(result.Documentation)).
		Msg("GitHub discovery completed")
	return result, nil
}

// listRepositories reads the configured repositories one by one, or lists
// the owner's. An organisation lists every repository the token can see;
// a user only lists their private ones through /user/repos, as themselves.
func listRepositories(ctx context.Context, c *client, config *Config) ([]repository, error) {
	if len(config.Repositories) > 0 {
		var repos []repository
		for _, name := range config.Repositories {
			var r repository
			if err := c.do(ctx, "GET", repoPath(config.Owner, name), nil, nil, &r); err != nil {
				if isNotFound(err) {
					log.Warn().Str("repository", name).Msg("Configured repository not found, or the token cannot see it")
					continue
				}
				return nil, err
			}
			repos = append(repos, r)
		}
		return repos, nil
	}

	var owner account
	if err := c.do(ctx, "GET", "/users/"+url.PathEscape(config.Owner), nil, nil, &owner); err != nil {
		return nil, fmt.Errorf("looking up owner %s: %w", config.Owner, err)
	}
	if owner.Type == "Organization" {
		return getPaged[repository](ctx, c, "/orgs/"+url.PathEscape(config.Owner)+"/repos", url.Values{"type": {"all"}}, maxRepositories)
	}

	// An installation token has no user, so /user fails and only the
	// owner's public repositories are listed.
	var me account
	if err := c.do(ctx, "GET", "/user", nil, nil, &me); err == nil && strings.EqualFold(me.Login, config.Owner) {
		return getPaged[repository](ctx, c, "/user/repos", url.Values{"affiliation": {"owner"}, "visibility": {"all"}}, maxRepositories)
	}
	return getPaged[repository](ctx, c, "/users/"+url.PathEscape(config.Owner)+"/repos", url.Values{"type": {"owner"}}, maxRepositories)
}

// repositoryInScope fetches a repository named in a tool and refuses it
// unless the config allows it, before anything else is read from it.
func repositoryInScope(ctx context.Context, c *client, config *Config, name string) (*repository, error) {
	if !validRepoName(name) {
		return nil, fmt.Errorf("%q is not a repository name", name)
	}
	if len(config.Repositories) > 0 && !slices.Contains(config.Repositories, name) {
		return nil, fmt.Errorf("repository %s is %w: it is not in repositories", name, ErrOutOfScope)
	}
	var r repository
	if err := c.do(ctx, "GET", repoPath(config.Owner, name), nil, nil, &r); err != nil {
		return nil, fmt.Errorf("reading repository %s/%s: %w", config.Owner, name, err)
	}
	if !config.inScope(&r) {
		return nil, fmt.Errorf("repository %s/%s is %w: the discovery settings leave it out", config.Owner, name, ErrOutOfScope)
	}
	return &r, nil
}
