package github

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findAsset(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

// assetNames returns the names of the repositories discovered.
func assetNames(result *pluginsdk.DiscoveryResult) []string {
	return namesOf(result, assetType)
}

func namesOf(result *pluginsdk.DiscoveryResult, assetType string) []string {
	var names []string
	for _, a := range result.Assets {
		if a.Type == assetType {
			names = append(names, *a.Name)
		}
	}
	return names
}

// orgFixture is an organisation with one repository of each kind discovery
// must tell apart.
func orgFixture(t *testing.T) *fakeGitHub {
	f := newFake(t)
	f.addRepo("pipelines", map[string]any{
		"private": true, "visibility": "private", "description": "Nightly ETL", "language": "Python",
		"topics": []string{"data", "etl"}, "stargazers_count": 3, "license": map[string]any{"spdx_id": "Apache-2.0"},
		"pushed_at": "2026-11-20T10:00:00Z",
	}, map[string]string{
		"README.md":          "# Pipelines\n\nRuns the nightly loads.\n",
		".github/CODEOWNERS": "# owners\n* @acme/data @alice\n/docs/ @acme/writers\n",
		"dags/nightly.py":    "SCHEDULE = \"@daily\"\n",
	})
	f.addRepo("website", map[string]any{"topics": []string{"web"}}, map[string]string{"index.html": "<p>hi</p>"})
	f.addRepo("legacy", map[string]any{"archived": true, "topics": []string{"data"}}, map[string]string{"README.md": "old"})
	f.addRepo("internal-tools", map[string]any{"visibility": "internal", "topics": []string{"data"}}, map[string]string{"x": "y"})
	return f
}

func discover(t *testing.T, f *fakeGitHub, extra map[string]any) *pluginsdk.DiscoveryResult {
	t.Helper()
	s := &Source{}
	raw, err := s.Validate(f.config(extra))
	require.NoError(t, err)
	result, err := s.Discover(context.Background(), raw)
	require.NoError(t, err)
	return result
}

func TestDiscoverOrganisation(t *testing.T) {
	f := orgFixture(t)
	result := discover(t, f, map[string]any{"include_codeowners": true, "tags": []string{"github", "${language}"}})

	assert.ElementsMatch(t, []string{"acme/pipelines", "acme/website", "acme/internal-tools"}, assetNames(result), "archived is left out by default")

	a := findAsset(result, "acme/pipelines")
	require.NotNil(t, a)
	assert.Equal(t, "Repository", a.Type)
	assert.Equal(t, []string{"GitHub"}, a.Providers)
	assert.Equal(t, "mrn://asset/repository/github/acme.pipelines", *a.MRN)
	assert.Equal(t, "Nightly ETL", *a.Description)
	assert.Equal(t, []string{"github", "Python", "data", "etl"}, a.Tags, "config tags, interpolated, then topics")
	assert.Equal(t, "main", a.Metadata["default_branch"])
	assert.Equal(t, "private", a.Metadata["visibility"])
	assert.Equal(t, "Apache-2.0", a.Metadata["license"])
	assert.Equal(t, "2026-11-20T10:00:00Z", a.Metadata["pushed_at"])
	assert.Equal(t, []string{"@acme/data", "@alice"}, a.Metadata["owners"], "the catch-all rule's owners only")
	require.Len(t, a.ExternalLinks, 1)
	assert.Equal(t, "https://github.com/acme/pipelines", a.ExternalLinks[0].URL)

	require.Len(t, result.Documentation, 1, "only pipelines has a README")
	assert.Equal(t, *a.MRN, result.Documentation[0].MRN)
	assert.Contains(t, result.Documentation[0].Content, "Runs the nightly loads.")
	for _, e := range result.Lineage {
		assert.Equal(t, "CONTAINS", e.Type, "only containment, never guessed data lineage")
	}
}

// childFixture is orgFixture with a feature branch, an open and a closed
// pull request, an issue and a repository with issues turned off.
func childFixture(t *testing.T) *fakeGitHub {
	f := orgFixture(t)
	f.commit("pipelines", "feat/backfill", map[string]string{"dags/backfill.py": "x"}, nil)
	f.repos["pipelines"].pulls[1] = &fakePR{number: 1, title: "Add backfill", body: "Backfills **2025**.", state: "open",
		head: "feat/backfill", base: "main", user: "alice", created: f.clock, updated: f.clock}
	f.repos["pipelines"].pulls[2] = &fakePR{number: 2, title: "Old", state: "closed", head: "old", base: "main", user: "bob", created: f.clock, updated: f.clock}
	f.repos["pipelines"].issues = []map[string]any{
		{"number": 3, "title": "Nightly load is slow", "body": "Takes 4h.", "html_url": "https://github.com/acme/pipelines/issues/3",
			"user": map[string]any{"login": "carol"}, "labels": []map[string]any{{"name": "bug"}}, "assignees": []map[string]any{{"login": "alice"}},
			"comments": 2, "created_at": "2026-11-01T00:00:00Z", "updated_at": "2026-11-02T00:00:00Z"},
		{"number": 1, "title": "Add backfill", "pull_request": map[string]any{}, "user": map[string]any{"login": "alice"},
			"created_at": "2026-11-01T00:00:00Z", "updated_at": "2026-11-02T00:00:00Z"},
	}
	f.repos["website"].issues = nil
	return f
}

func TestDiscoverBranchesPullRequestsAndIssues(t *testing.T) {
	for _, path := range []string{"graphql", "rest"} {
		t.Run(path, func(t *testing.T) {
			f := childFixture(t)
			f.noGraphQL = path == "rest"
			checkChildren(t, f)
			if path == "graphql" {
				for _, p := range []string{"/branches", "/pulls", "/issues", "/readme"} {
					assert.False(t, f.requested("GET /repos/acme/pipelines"+p), "GraphQL answered %s", p)
				}
			}
		})
	}
}

func checkChildren(t *testing.T, f *fakeGitHub) {
	t.Helper()
	result := discover(t, f, map[string]any{"repositories": []string{"pipelines", "website"}})

	assert.ElementsMatch(t, []string{"acme/pipelines:feat/backfill", "acme/pipelines:main", "acme/website:main"}, namesOf(result, branchType))
	assert.Equal(t, []string{"acme/pipelines#1"}, namesOf(result, pullRequestType), "open pull requests only")
	assert.Equal(t, []string{"acme/pipelines#3"}, namesOf(result, issueType), "pull requests in the issues API are not issues; disabled issues are skipped")

	main := findAsset(result, "acme/pipelines:main")
	require.NotNil(t, main)
	assert.Equal(t, "mrn://asset/branch/github/acme.pipelines.main", *main.MRN)
	assert.Equal(t, true, main.Metadata["default"])
	assert.Equal(t, "https://github.com/acme/pipelines/tree/feat/backfill", findAsset(result, "acme/pipelines:feat/backfill").Metadata["url"])

	pr := findAsset(result, "acme/pipelines#1")
	require.NotNil(t, pr)
	assert.Equal(t, "Add backfill", *pr.Description)
	assert.Equal(t, "feat/backfill", pr.Metadata["head_branch"])
	assert.Equal(t, "alice", pr.Metadata["author"])

	is := findAsset(result, "acme/pipelines#3")
	require.NotNil(t, is)
	assert.Equal(t, "mrn://asset/issue/github/acme.pipelines.3", *is.MRN)
	assert.Equal(t, []string{"bug"}, is.Metadata["labels"])
	assert.Equal(t, []string{"alice"}, is.Metadata["assignees"])
	assert.Contains(t, is.Tags, "bug", "labels become tags")

	repo := findAsset(result, "acme/pipelines")
	for _, child := range []*pluginsdk.Asset{main, pr, is} {
		assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{Source: *repo.MRN, Target: *child.MRN, Type: "CONTAINS"})
	}
	docs := map[string]string{}
	for _, d := range result.Documentation {
		docs[d.MRN] = d.Content
	}
	assert.Equal(t, "Backfills **2025**.", docs[*pr.MRN])
	assert.Equal(t, "Takes 4h.", docs[*is.MRN])

	result = discover(t, f, map[string]any{"include_branches": false, "include_pull_requests": false, "include_issues": false})
	assert.Equal(t, len(assetNames(result)), len(result.Assets), "each kind can be turned off")
	assert.Empty(t, result.Lineage)
}

func TestDiscoverReadsOverflowingListsThroughREST(t *testing.T) {
	f := childFixture(t)
	graphqlFirst = 1
	t.Cleanup(func() { graphqlFirst = 100 })
	checkChildren(t, f)
	assert.True(t, f.requested("GET /repos/acme/pipelines/branches"), "two branches overflow a page of one")
	assert.False(t, f.requested("GET /repos/acme/website/branches"), "one branch fits")
}

func TestDiscoverFallsBackToRESTWhenGraphQLFails(t *testing.T) {
	f := childFixture(t)
	f.graphqlError = true
	checkChildren(t, f)
	assert.True(t, f.requested("GET /repos/acme/pipelines/readme"))
}

func TestDiscoverFilters(t *testing.T) {
	f := orgFixture(t)

	result := discover(t, f, map[string]any{"include_private": false})
	assert.Equal(t, []string{"acme/website"}, assetNames(result), "private and internal both need include_private")

	result = discover(t, f, map[string]any{"include_archived": true, "topics": []string{"data"}})
	assert.ElementsMatch(t, []string{"acme/pipelines", "acme/legacy", "acme/internal-tools"}, assetNames(result))

	result = discover(t, f, map[string]any{"repositories": []string{"website", "missing"}})
	assert.Equal(t, []string{"acme/website"}, assetNames(result), "a missing configured repository is skipped")

	result = discover(t, f, map[string]any{"filter": map[string]any{"exclude": []string{"^acme/web"}}})
	assert.ElementsMatch(t, []string{"acme/pipelines", "acme/internal-tools"}, assetNames(result))

	result = discover(t, f, map[string]any{"include_readme": false})
	assert.Empty(t, result.Documentation)
	assert.False(t, f.requested("GET /repos/acme/pipelines/contents"), "CODEOWNERS is opt in")
}

func TestDiscoverFollowsPagesOnTheAPIHostOnly(t *testing.T) {
	f := orgFixture(t)
	f.pageSize = 1
	result := discover(t, f, nil)
	assert.Len(t, assetNames(result), 3)

	f.foreignNext = true
	result = discover(t, f, nil)
	assert.Len(t, assetNames(result), 1, "a next link to another host is not followed")
}

func TestDiscoverUser(t *testing.T) {
	f := newFake(t)
	f.owner, f.ownerType, f.login = "alice", "User", "alice"
	f.addRepo("dotfiles", map[string]any{"private": true, "visibility": "private"}, nil)
	f.addRepo("blog", nil, nil)

	result := discover(t, f, nil)
	assert.ElementsMatch(t, []string{"alice/dotfiles", "alice/blog"}, assetNames(result))
	assert.True(t, f.requested("GET /user/repos"), "the token's own user lists private repositories")

	// An installation token has no user, so only public repositories show.
	f.login = ""
	result = discover(t, f, nil)
	assert.Equal(t, []string{"alice/blog"}, assetNames(result))
}

func TestTokenFromEnvironment(t *testing.T) {
	f := orgFixture(t)
	t.Setenv(tokenEnv, testToken)
	raw := f.config(nil)
	delete(raw, "token")
	result, err := (&Source{}).Discover(context.Background(), raw)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Assets)

	t.Setenv(tokenEnv, "")
	_, err = (&Source{}).Discover(context.Background(), raw)
	require.ErrorContains(t, err, "no GitHub token")
}

func TestTokenNeverInErrors(t *testing.T) {
	f := orgFixture(t)
	raw := f.config(map[string]any{"token": "ghp_wrong-secret-value"})
	_, err := (&Source{}).Discover(context.Background(), raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401 Bad credentials")
	assert.NotContains(t, err.Error(), "ghp_wrong-secret-value")
}

func TestConfigValidation(t *testing.T) {
	valid := pluginsdk.RawConfig{"owner": "acme"}
	_, err := (&Source{}).Validate(valid)
	require.NoError(t, err)

	cases := map[string]pluginsdk.RawConfig{
		"no owner":                 {},
		"bad owner":                {"owner": "acme/evil"},
		"plain http":               {"owner": "acme", "base_url": "http://github.example.com/api/v3"},
		"not https":                {"owner": "acme", "base_url": "ftp://github.example.com"},
		"credentials in base_url":  {"owner": "acme", "base_url": "https://user:pw@github.example.com/api/v3"},
		"bad repository":           {"owner": "acme", "repositories": []string{"../x"}},
		"bad topic":                {"owner": "acme", "topics": []string{"Data Sets"}},
		"bad filter":               {"owner": "acme", "filter": map[string]any{"include": []string{"("}}},
		"write outside repos":      {"owner": "acme", "repositories": []string{"a"}, "tools": map[string]any{"repos": []string{"b"}}},
		"bad tool repo":            {"owner": "acme", "tools": map[string]any{"repos": []string{".."}}},
		"prefix without slash":     {"owner": "acme", "tools": map[string]any{"branch_prefix": "marmot"}},
		"prefix with traversal":    {"owner": "acme", "tools": map[string]any{"branch_prefix": "../"}},
		"too many files":           {"owner": "acme", "tools": map[string]any{"max_files": 4}},
		"file size above ceiling":  {"owner": "acme", "tools": map[string]any{"max_file_bytes": 2 << 20}},
		"negative file size limit": {"owner": "acme", "tools": map[string]any{"max_file_bytes": -1}},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := (&Source{}).Validate(raw)
			require.Error(t, err)
		})
	}

	for _, ok := range []string{"https://github.example.com/api/v3", "http://127.0.0.1:8080", "http://localhost:9000/api/v3"} {
		_, err := (&Source{}).Validate(pluginsdk.RawConfig{"owner": "acme", "base_url": ok})
		require.NoError(t, err, ok)
	}
}

func TestMeta(t *testing.T) {
	m := Meta()
	assert.Equal(t, "github", m.ID)
	require.Len(t, m.AssetSchemas, 4)
	for _, s := range m.AssetSchemas {
		assert.NotEmpty(t, s.Fields)
	}
	for _, f := range m.ConfigSpec {
		if f.Name == "token" {
			assert.True(t, f.Sensitive, "the token is never shown back")
		}
	}
}

func TestDefaultOwners(t *testing.T) {
	assert.Equal(t, []string{"@b"}, defaultOwners("* @a\n/src/ @x\n* @b # later wins\n"))
	assert.Nil(t, defaultOwners("/src/ @x\n"))
	assert.Equal(t, []string{"@team/x"}, defaultOwners("# c\n\n**   @team/x\n"))
}
