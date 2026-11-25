package github

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

const (
	branchType      = "Branch"
	pullRequestType = "Pull Request"
	issueType       = "Issue"

	// maxChildren bounds the branches, pull requests or issues read from
	// one repository, so one busy repository cannot dominate a run.
	maxChildren = 1000
	// maxBodyBytes caps the pull request or issue body kept as
	// documentation.
	maxBodyBytes = 64 << 10
)

type branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type label struct {
	Name string `json:"name"`
}

// issue is an issue or, when PullRequest is set, a pull request as the
// issues API returns it.
type issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	User        account   `json:"user"`
	Labels      []label   `json:"labels"`
	Assignees   []account `json:"assignees"`
	Comments    int       `json:"comments"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"`
}

type openPullRequest struct {
	pullRequest
	Body               string    `json:"body"`
	Labels             []label   `json:"labels"`
	Assignees          []account `json:"assignees"`
	RequestedReviewers []account `json:"requested_reviewers"`
}

// children collects the assets found inside one repository, each linked
// to it by a CONTAINS edge.
type children struct {
	config *Config
	repo   *repository
	parent string
	seen   map[string]bool
	result *pluginsdk.DiscoveryResult
}

// describeChildren adds a repository's branches, open pull requests and
// open issues. Each list comes from d when a GraphQL query read all of it,
// and otherwise from REST.
func describeChildren(ctx context.Context, c *client, config *Config, r *repository, d *repoDetails, parentMRN string, result *pluginsdk.DiscoveryResult) {
	ch := &children{config: config, repo: r, parent: parentMRN, seen: map[string]bool{}, result: result}
	path := repoPath(config.Owner, r.Name)
	open := url.Values{"state": {"open"}, "sort": {"updated"}, "direction": {"desc"}}

	if config.IncludeBranches {
		branches := func() []branch {
			if d != nil && d.branchesDone {
				return d.branches
			}
			b, err := getPaged[branch](ctx, c, path+"/branches", nil, maxChildren)
			ch.warn(err, "branches")
			return b
		}()
		for _, b := range branches {
			ch.addBranch(b)
		}
	}
	if config.IncludePullRequests {
		pulls := func() []openPullRequest {
			if d != nil && d.pullsDone {
				return d.pulls
			}
			p, err := getPaged[openPullRequest](ctx, c, path+"/pulls", open, maxChildren)
			ch.warn(err, "pull requests")
			return p
		}()
		for i := range pulls {
			ch.addPullRequest(&pulls[i])
		}
	}
	if config.IncludeIssues {
		issues := func() []issue {
			if d != nil && d.issuesDone {
				return d.issues
			}
			is, err := getPaged[issue](ctx, c, path+"/issues", open, maxChildren)
			ch.warn(err, "issues")
			return is
		}()
		for i := range issues {
			// The REST issues API returns pull requests too.
			if issues[i].PullRequest == nil {
				ch.addIssue(&issues[i])
			}
		}
	}
}

// warn logs a failed listing and carries on: a repository with issues
// turned off answers 410, and the repository itself is still worth having.
func (ch *children) warn(err error, what string) {
	if err != nil {
		log.Warn().Err(err).Str("repository", ch.repo.Name).Msgf("Failed to list %s", what)
	}
}

func (ch *children) fullName() string { return ch.config.Owner + "/" + ch.repo.Name }

func (ch *children) addBranch(b branch) {
	metadata := map[string]any{
		"repository": ch.fullName(),
		"branch":     b.Name,
		"default":    b.Name == ch.repo.DefaultBranch,
		"commit_sha": b.Commit.SHA,
	}
	link := ""
	if ch.repo.HTMLURL != "" {
		link = ch.repo.HTMLURL + "/tree/" + escapePath(b.Name)
		metadata["url"] = link
	}
	ch.add(branchType, ch.fullName()+":"+b.Name, b.Name, "", link, metadata, nil)
}

func (ch *children) addPullRequest(pr *openPullRequest) {
	metadata := map[string]any{
		"repository":  ch.fullName(),
		"number":      pr.Number,
		"title":       stripControl(pr.Title),
		"author":      pr.User.Login,
		"draft":       pr.Draft,
		"head_branch": pr.Head.Ref,
		"base_branch": pr.Base.Ref,
		"url":         pr.HTMLURL,
		"created_at":  pr.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":  pr.UpdatedAt.UTC().Format(time.RFC3339),
	}
	setLogins(metadata, "assignees", pr.Assignees)
	setLogins(metadata, "reviewers", pr.RequestedReviewers)
	labels := labelNames(pr.Labels)
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	if mrn, ok := ch.add(pullRequestType, ch.fullName()+"#"+strconv.Itoa(pr.Number), strconv.Itoa(pr.Number), pr.Title, pr.HTMLURL, metadata, labels); ok {
		ch.document(mrn, pr.Body)
	}
}

func (ch *children) addIssue(is *issue) {
	metadata := map[string]any{
		"repository": ch.fullName(),
		"number":     is.Number,
		"title":      stripControl(is.Title),
		"author":     is.User.Login,
		"comments":   is.Comments,
		"url":        is.HTMLURL,
		"created_at": is.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": is.UpdatedAt.UTC().Format(time.RFC3339),
	}
	setLogins(metadata, "assignees", is.Assignees)
	labels := labelNames(is.Labels)
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	if mrn, ok := ch.add(issueType, ch.fullName()+"#"+strconv.Itoa(is.Number), strconv.Itoa(is.Number), is.Title, is.HTMLURL, metadata, labels); ok {
		ch.document(mrn, is.Body)
	}
}

// add appends a child asset under name, with key as the last MRN level,
// and returns its MRN.
// Two branches whose names differ only in case or in a slash against a
// dash share an MRN, so only the first is kept.
func (ch *children) add(assetType, name, key, description, link string, metadata map[string]any, labels []string) (string, bool) {
	mrn := ch.config.AssetMRN(assetType, provider, ch.config.Owner, ch.repo.Name, key)
	if ch.seen[mrn] {
		log.Debug().Str("asset", name).Msg("Skipping asset whose MRN is already taken")
		return "", false
	}
	ch.seen[mrn] = true

	tags := pluginsdk.InterpolateTags(ch.config.Tags, metadata)
	for _, l := range labels {
		if !slices.Contains(tags, l) {
			tags = append(tags, l)
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
	if d := stripControl(description); d != "" {
		asset.Description = &d
	}
	if link != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "GitHub", Icon: "github", URL: link}}
	}
	ch.result.Assets = append(ch.result.Assets, asset)
	ch.result.Lineage = append(ch.result.Lineage, pluginsdk.LineageEdge{Source: ch.parent, Target: mrn, Type: "CONTAINS"})
	return mrn, true
}

// document attaches a pull request or issue body to its asset.
func (ch *children) document(mrn, body string) {
	if body == "" {
		return
	}
	ch.result.Documentation = append(ch.result.Documentation, pluginsdk.Documentation{
		MRN: mrn, Content: capText(body, maxBodyBytes), Source: provider,
	})
}

func setLogins(metadata map[string]any, key string, accounts []account) {
	var logins []string
	for _, a := range accounts {
		if a.Login != "" {
			logins = append(logins, a.Login)
		}
	}
	if len(logins) > 0 {
		metadata[key] = logins
	}
}

func labelNames(labels []label) []string {
	var names []string
	for _, l := range labels {
		if l.Name != "" && !slices.Contains(names, l.Name) {
			names = append(names, l.Name)
		}
	}
	return names
}
