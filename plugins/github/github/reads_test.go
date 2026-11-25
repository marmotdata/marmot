package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func request(tool string, params map[string]any) pluginsdk.ToolRequest {
	return pluginsdk.ToolRequest{Tool: tool, Params: params, Identity: "user:alice", Origin: "task_run:42"}
}

func testEngine(t *testing.T, f *fakeGitHub, extra map[string]any) *engine {
	t.Helper()
	e, err := newEngine(f.config(extra), "task_run:42")
	require.NoError(t, err)
	return e
}

func runRead[T any](t *testing.T, e *engine, req pluginsdk.ToolRequest) (*pluginsdk.ToolResult, T) {
	t.Helper()
	result, err := e.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Untrusted)
	assert.LessOrEqual(t, len(result.Output), pluginsdk.MaxReadOutputBytes)
	var out T
	require.NoError(t, json.Unmarshal(result.Output, &out))
	return result, out
}

func readFixture(t *testing.T) *fakeGitHub {
	f := newFake(t)
	f.addRepo("pipelines", map[string]any{"private": true, "visibility": "private"}, map[string]string{
		"README.md":        "# Pipelines\n",
		"dags/nightly.py":  "SCHEDULE = \"@daily\"\n",
		"dags/hourly.py":   "SCHEDULE = \"@hourly\"\n",
		"dags/lib/util.py": "def f():\n    return 1\n",
		"logo.png":         "\x89PNG\x00\x00",
		"big.sql":          strings.Repeat("<select 1;>\n", 10000),
	})
	f.commit("pipelines", "feature", map[string]string{"dags/nightly.py": "SCHEDULE = \"@weekly\"\n"}, nil)
	f.addRepo("secret", map[string]any{"private": true, "visibility": "private"}, map[string]string{"x": "y"})
	return f
}

func TestReadManifest(t *testing.T) {
	specs := ToolSpecs()
	require.NoError(t, pluginsdk.ValidateManifest(specs))
	for _, s := range specs {
		assert.Equal(t, isRead(s.Name), s.ReadOnly(), s.Name)
	}
}

func TestReadFile(t *testing.T) {
	f := readFixture(t)
	e := testEngine(t, f, nil)

	result, out := runRead[fileRead](t, e, request(toolReadFile, map[string]any{"repo": "pipelines", "path": "dags/nightly.py"}))
	assert.Equal(t, "SCHEDULE = \"@daily\"\n", out.Content)
	assert.Equal(t, "main", out.Ref)
	assert.Equal(t, blobSHA(out.Content), out.SHA)
	assert.Equal(t, kindFile, result.TargetKind)
	assert.Equal(t, "acme/pipelines/dags/nightly.py@main", result.Target)

	_, out = runRead[fileRead](t, e, request(toolReadFile, map[string]any{"repo": "pipelines", "path": "dags/nightly.py", "ref": "feature"}))
	assert.Equal(t, "SCHEDULE = \"@weekly\"\n", out.Content)
	commit := out.Commit
	_, out = runRead[fileRead](t, e, request(toolReadFile, map[string]any{"repo": "pipelines", "path": "dags/nightly.py", "ref": commit}))
	assert.Equal(t, "SCHEDULE = \"@weekly\"\n", out.Content, "a commit is a ref")

	_, out = runRead[fileRead](t, e, request(toolReadFile, map[string]any{"repo": "pipelines", "path": "logo.png"}))
	assert.True(t, out.Binary)
	assert.Empty(t, out.Content)

	_, out = runRead[fileRead](t, e, request(toolReadFile, map[string]any{"repo": "pipelines", "path": "big.sql"}))
	assert.True(t, out.Truncated)
	assert.Len(t, out.Content, maxReadFileBytes)
	assert.Equal(t, int64(120000), out.Size)

	for path, want := range map[string]string{
		"dags":            "directory",
		"dags/missing.py": "does not exist",
		"../etc/passwd":   "not a plain path",
		"dags/../x":       "not a plain path",
		".git/config":     "not a plain path",
	} {
		_, err := e.Execute(context.Background(), request(toolReadFile, map[string]any{"repo": "pipelines", "path": path}))
		require.ErrorContains(t, err, want, path)
	}
	_, err := e.Execute(context.Background(), request(toolReadFile, map[string]any{"repo": "pipelines", "path": "README.md", "ref": "main..feature"}))
	require.ErrorContains(t, err, "not a valid")
}

func TestReadsStayInScope(t *testing.T) {
	f := readFixture(t)
	read := request(toolReadFile, map[string]any{"repo": "secret", "path": "x"})

	_, err := testEngine(t, f, map[string]any{"repositories": []string{"pipelines"}}).Execute(context.Background(), read)
	require.ErrorIs(t, err, ErrOutOfScope)
	assert.Equal(t, codes.PermissionDenied, status.Code(wireError(err)))
	assert.False(t, f.requested("GET /repos/acme/secret"), "a repository outside repositories is not even read")

	_, err = testEngine(t, f, map[string]any{"include_private": false}).Execute(context.Background(), read)
	require.ErrorIs(t, err, ErrOutOfScope, "a private repository is out of scope without include_private")

	_, err = testEngine(t, f, map[string]any{"filter": map[string]any{"exclude": []string{"secret"}}}).Execute(context.Background(), read)
	require.ErrorIs(t, err, ErrOutOfScope, "the name filter applies to reads")

	_, err = testEngine(t, f, nil).Execute(context.Background(), read)
	require.NoError(t, err, "reads need no tools.repos entry")
}

func TestListFiles(t *testing.T) {
	f := readFixture(t)
	e := testEngine(t, f, nil)

	_, out := runRead[fileList](t, e, request(toolListFiles, map[string]any{"repo": "pipelines", "path": "dags", "limit": int64(10)}))
	var paths []string
	for _, file := range out.Files {
		paths = append(paths, file.Path)
	}
	assert.Equal(t, []string{"dags/hourly.py", "dags/lib/util.py", "dags/nightly.py"}, paths, "recursive, files only, full paths")
	assert.False(t, out.Truncated)

	_, out = runRead[fileList](t, e, request(toolListFiles, map[string]any{"repo": "pipelines", "limit": int64(2)}))
	assert.Len(t, out.Files, 2)
	assert.Equal(t, 6, out.Total)
	assert.True(t, out.Truncated)

	_, err := e.Execute(context.Background(), request(toolListFiles, map[string]any{"repo": "pipelines", "path": "README.md", "limit": int64(1)}))
	require.ErrorContains(t, err, "not a directory")
	_, err = e.Execute(context.Background(), request(toolListFiles, map[string]any{"repo": "pipelines", "limit": int64(501)}))
	require.Error(t, err, "the count is capped by the manifest")
}

func TestListPullRequests(t *testing.T) {
	f := readFixture(t)
	r := f.repos["pipelines"]
	f.commit("pipelines", "marmot/bump-1234", map[string]string{"x": "1"}, nil)
	r.pulls[1] = &fakePR{number: 1, title: "Bump \x1b[31mschedule", state: "open", head: "marmot/bump-1234", base: "main", headRepo: "acme/pipelines", user: "marmot-bot"}
	r.pulls[2] = &fakePR{number: 2, title: "Human change", state: "open", head: "feature", base: "main", headRepo: "acme/pipelines", user: "bob"}
	r.pulls[3] = &fakePR{number: 3, title: "Forked", state: "open", head: "marmot/evil", base: "main", headRepo: "mallory/pipelines", user: "mallory"}
	r.pulls[4] = &fakePR{number: 4, title: "Old", state: "closed", head: "old", base: "main", headRepo: "acme/pipelines", user: "bob", merged: true}
	e := testEngine(t, f, nil)

	_, out := runRead[pullRequestList](t, e, request(toolListPullRequests, map[string]any{"repo": "pipelines", "state": "open", "limit": int64(10)}))
	require.Len(t, out.PullRequests, 3)
	byNumber := map[int]pullRequestSummary{}
	for _, pr := range out.PullRequests {
		byNumber[pr.Number] = pr
	}
	assert.True(t, byNumber[1].OpenedByMarmot)
	assert.Equal(t, "Bump [31mschedule", byNumber[1].Title, "control characters are dropped")
	assert.False(t, byNumber[2].OpenedByMarmot)
	assert.False(t, byNumber[3].OpenedByMarmot, "a fork's branch under the prefix is not the plugin's")

	_, out = runRead[pullRequestList](t, e, request(toolListPullRequests, map[string]any{"repo": "pipelines", "state": "closed", "limit": int64(10)}))
	require.Len(t, out.PullRequests, 1)
	assert.True(t, out.PullRequests[0].Merged)

	_, out = runRead[pullRequestList](t, e, request(toolListPullRequests, map[string]any{"repo": "pipelines", "state": "open", "limit": int64(1)}))
	assert.Len(t, out.PullRequests, 1)
}

func TestReadsAreNotPlanned(t *testing.T) {
	f := readFixture(t)
	_, err := testEngine(t, f, nil).Plan(context.Background(), request(toolReadFile, map[string]any{"repo": "pipelines", "path": "README.md"}))
	require.ErrorContains(t, err, "nothing to plan")
}
