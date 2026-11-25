package github

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const settings = "retries: 3\ntimeout: 30\nschedule: \"@daily\"\nowner: data\n"

func actionFixture(t *testing.T) *fakeGitHub {
	f := newFake(t)
	f.addRepo("pipelines", map[string]any{"private": true, "visibility": "private"}, map[string]string{
		"config/settings.yaml":       settings,
		"README.md":                  "# Pipelines\n",
		"run.sh":                     "#!/bin/sh\nexec python -m pipelines --env=prod\n",
		".github/workflows/ci.yml":   "on: pull_request\n",
		".github/actions/x/action.y": "runs: {}\n",
		"logo.png":                   "\x89PNG\x00",
		"twice.txt":                  "a=1\na=1\n",
	})
	f.repos["pipelines"].meta["topics"] = []string{"data"}
	f.addRepo("website", nil, map[string]string{"index.html": "hi"})
	return f
}

func writeEngine(t *testing.T, f *fakeGitHub, actions map[string]any) *engine {
	t.Helper()
	if actions == nil {
		actions = map[string]any{}
	}
	if _, ok := actions["repos"]; !ok {
		actions["repos"] = []string{"pipelines"}
	}
	return testEngine(t, f, map[string]any{"actions": actions})
}

func openRequest(edits ...string) pluginsdk.ActionRequest {
	params := map[string]any{"repo": "pipelines", "title": "Retry five times"}
	for i := 0; i+2 < len(edits); i += 3 {
		n := i/3 + 1
		params[fmt.Sprintf("path_%d", n)] = edits[i]
		params[fmt.Sprintf("old_%d", n)] = edits[i+1]
		params[fmt.Sprintf("new_%d", n)] = edits[i+2]
	}
	return request(actionOpenPullRequest, params)
}

// planAndExecute plans, checks the plan wrote nothing, then executes with
// the plan's precondition.
func planAndExecute(t *testing.T, f *fakeGitHub, e *engine, req pluginsdk.ActionRequest) (*pluginsdk.ActionPlan, *pluginsdk.ActionResult) {
	t.Helper()
	writes := f.writes()
	plan, err := e.Plan(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, writes, f.writes(), "a plan must write nothing")
	require.NotEmpty(t, plan.Precondition)

	req.Precondition = plan.Precondition
	result, err := e.Execute(context.Background(), req)
	require.NoError(t, err)
	return plan, result
}

func TestActionManifestIsValid(t *testing.T) {
	require.NoError(t, pluginsdk.ValidateManifest(ActionSpecs()))
	assert.Equal(t, pluginsdk.ManifestDigest(ActionSpecs()), pluginsdk.ManifestDigest(ActionSpecs()))
}

func TestOpenPullRequest(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	base := f.repos["pipelines"].branches["main"]
	req := openRequest(
		"config/settings.yaml", "retries: 3", "retries: 5",
		"run.sh", "--env=prod", "--env=prod --retries=5",
	)
	req.Params["body"] = "The nightly load flakes; give it more retries."

	plan, result := planAndExecute(t, f, e, req)
	assert.Equal(t, base, plan.Precondition, "bound to the base branch's head")
	assert.Equal(t, kindPullRequest, plan.TargetKind)
	assert.True(t, strings.HasPrefix(plan.Target, "acme/pipelines:marmot/retry-five-times-"), plan.Target)
	assert.Equal(t, "--- a/config/settings.yaml\n+++ b/config/settings.yaml\n@@ -1,4 +1,4 @@\n-retries: 3\n+retries: 5\n timeout: 30\n schedule: \"@daily\"\n owner: data\n"+
		"--- a/run.sh\n+++ b/run.sh\n@@ -1,3 +1,3 @@\n #!/bin/sh\n-exec python -m pipelines --env=prod\n+exec python -m pipelines --env=prod --retries=5\n \n", plan.Diff)
	assert.Contains(t, string(plan.Before), base)
	assert.Contains(t, string(plan.After), blobSHA(strings.Replace(settings, "retries: 3", "retries: 5", 1)), "the plan names the exact blob")

	branch := strings.TrimPrefix(plan.Target, "acme/pipelines:")
	got, _, ok := f.fileAt("pipelines", branch, "config/settings.yaml")
	require.True(t, ok)
	assert.Equal(t, strings.Replace(settings, "retries: 3", "retries: 5", 1), got)
	_, mode, _ := f.fileAt("pipelines", branch, "run.sh")
	assert.Equal(t, "100644", mode)
	untouched, _, _ := f.fileAt("pipelines", branch, "README.md")
	assert.Equal(t, "# Pipelines\n", untouched, "the rest of the tree is the base's")

	head := f.repos["pipelines"].branches[branch]
	c := f.commits[head]
	assert.Equal(t, []string{base}, c.parents, "one commit on the planned base")
	assert.Contains(t, c.message, "Marmot-Origin: task_run:42\nMarmot-Actor: user:alice")
	assert.Equal(t, base, f.repos["pipelines"].branches["main"], "the base branch is never written")

	pr := f.repos["pipelines"].pulls[1]
	require.NotNil(t, pr)
	assert.Equal(t, "Retry five times", pr.title)
	assert.Equal(t, "main", pr.base)
	assert.Contains(t, pr.body, "The nightly load flakes")
	assert.Contains(t, pr.body, "Opened by Marmot for `user:alice` (`task_run:42`)")

	require.NotNil(t, result.Undo)
	assert.Equal(t, actionClosePullRequest, result.Undo.Action)
	assert.Equal(t, int64(1), result.Undo.Params["number"])
	assert.Contains(t, string(result.After), `"number":1`)

	for _, ua := range f.userAgents {
		assert.Equal(t, "marmot-plugin-github/actions (origin task_run:42)", ua)
	}

	// Executing again finds its own branch rather than opening a second
	// pull request.
	req.Precondition = plan.Precondition
	_, err := e.Execute(context.Background(), req)
	require.ErrorContains(t, err, "already exists")
	assert.Len(t, f.repos["pipelines"].pulls, 1)
}

func TestOpenPullRequestKeepsTheFileMode(t *testing.T) {
	f := actionFixture(t)
	f.commit("pipelines", "main", map[string]string{"bin/run": "#!/bin/sh\necho old\n"}, map[string]string{"bin/run": "100755"})
	e := writeEngine(t, f, nil)
	plan, _ := planAndExecute(t, f, e, openRequest("bin/run", "echo old", "echo new"))
	_, mode, _ := f.fileAt("pipelines", strings.TrimPrefix(plan.Target, "acme/pipelines:"), "bin/run")
	assert.Equal(t, "100755", mode)
}

func TestOpenPullRequestRefusesAMovedBase(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	req := openRequest("config/settings.yaml", "retries: 3", "retries: 5")
	plan, err := e.Plan(context.Background(), req)
	require.NoError(t, err)

	// Someone pushes to main between the plan and its execution.
	f.commit("pipelines", "main", map[string]string{"README.md": "# Changed\n"}, nil)

	req.Precondition = plan.Precondition
	_, err = e.Execute(context.Background(), req)
	require.ErrorIs(t, err, ErrStale)
	assert.Equal(t, codes.FailedPrecondition, status.Code(wireError(err)))
	assert.Len(t, f.repos["pipelines"].branches, 1, "no branch was created")
	assert.Empty(t, f.repos["pipelines"].pulls)

	req.Precondition = ""
	_, err = e.Execute(context.Background(), req)
	require.Error(t, err, "no precondition, no execution")
}

func TestOpenPullRequestRefusesCIChanges(t *testing.T) {
	f := actionFixture(t)
	for _, path := range []string{".github/workflows/ci.yml", ".GitHub/Workflows/ci.yml", ".github/actions/x/action.y"} {
		_, err := writeEngine(t, f, nil).Plan(context.Background(), openRequest(path, "on: pull_request", "on: push"))
		require.ErrorIs(t, err, ErrCIPath, path)
		assert.Contains(t, err.Error(), "actions.allow_ci_changes")
		assert.Equal(t, codes.PermissionDenied, status.Code(wireError(err)))
	}

	_, err := writeEngine(t, f, map[string]any{"allow_ci_changes": true}).Plan(context.Background(), openRequest(".github/workflows/ci.yml", "on: pull_request", "on: push"))
	require.NoError(t, err, "a runner can allow it")
}

func TestOpenPullRequestValidatesPaths(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	for _, path := range []string{"../etc/passwd", "config/../../x", "/etc/passwd", ".git/config", "config//settings.yaml", "config/./settings.yaml", "config/"} {
		_, err := e.Plan(context.Background(), openRequest(path, "a", "b"))
		require.ErrorIs(t, err, ErrBadPath, path)
	}
	for _, path := range []string{"config/settings yaml", "config\\settings.yaml", "~/x"} {
		_, err := e.Plan(context.Background(), openRequest(path, "a", "b"))
		require.ErrorContains(t, err, "must match", path)
	}
	assert.Zero(t, f.writes())
}

func TestOpenPullRequestCaps(t *testing.T) {
	f := actionFixture(t)
	f.commit("pipelines", "main", map[string]string{"link": "config/settings.yaml"}, map[string]string{"link": "120000"})
	plan := func(actions map[string]any, req pluginsdk.ActionRequest) error {
		_, err := writeEngine(t, f, actions).Plan(context.Background(), req)
		return err
	}

	require.ErrorContains(t, plan(map[string]any{"max_files": 1}, openRequest(
		"config/settings.yaml", "retries: 3", "retries: 5", "run.sh", "prod", "dev")), "max_files of 1")
	require.NoError(t, plan(map[string]any{"max_files": 1}, openRequest(
		"config/settings.yaml", "retries: 3", "retries: 5", "config/settings.yaml", "timeout: 30", "timeout: 60")), "two edits to one file are one file")

	require.ErrorContains(t, plan(map[string]any{"max_file_bytes": 20}, openRequest("config/settings.yaml", "retries: 3", "retries: 5")), "max_file_bytes of 20")
	require.ErrorContains(t, plan(nil, openRequest("twice.txt", "a=1", "a=2")), "occurs 2 times")
	require.ErrorContains(t, plan(nil, openRequest("config/settings.yaml", "retries: 9", "retries: 5")), "occurs 0 times")
	require.ErrorContains(t, plan(nil, openRequest("logo.png", "PNG", "GIF")), "binary")
	require.ErrorContains(t, plan(nil, openRequest("link", "x", "y")), "symbolic link")
	require.ErrorContains(t, plan(nil, openRequest("config", "x", "y")), "directory")
	require.ErrorContains(t, plan(nil, openRequest("missing.txt", "x", "y")), "does not exist")
	require.ErrorContains(t, plan(nil, openRequest("config/settings.yaml", "retries: 3", "retries: 3")), "changes nothing")
	require.ErrorContains(t, plan(nil, openRequest("config/settings.yaml", "", "x")), "is empty")

	// The SDK's own bounds: no line breaks, at most 1 KiB a parameter.
	require.ErrorContains(t, plan(nil, openRequest("config/settings.yaml", "retries: 3", "retries: 5\nrm: -rf")), "control characters")
	require.ErrorContains(t, plan(nil, openRequest("config/settings.yaml", "retries: 3", strings.Repeat("x", 1025))), "at most 1024 bytes")

	partial := openRequest("config/settings.yaml", "retries: 3", "retries: 5")
	partial.Params["path_2"] = "run.sh"
	require.ErrorContains(t, plan(nil, partial), "together")

	assert.Zero(t, f.writes())
}

func TestWritesStayInScope(t *testing.T) {
	f := actionFixture(t)
	req := openRequest("index.html", "hi", "hello")
	req.Params["repo"] = "website"

	_, err := writeEngine(t, f, nil).Plan(context.Background(), req)
	require.ErrorIs(t, err, ErrOutOfScope)
	assert.False(t, f.requested("GET /repos/acme/website"), "a repository outside actions.repos is not even read")

	_, err = testEngine(t, f, nil).Plan(context.Background(), openRequest("config/settings.yaml", "retries: 3", "retries: 5"))
	require.ErrorIs(t, err, ErrOutOfScope, "no actions.repos allows no writes")

	_, err = writeEngine(t, f, map[string]any{"repos": []string{"pipelines"}}).Plan(context.Background(), func() pluginsdk.ActionRequest {
		r := openRequest("config/settings.yaml", "retries: 3", "retries: 5")
		r.Params["base"] = "nope"
		return r
	}())
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(wireError(err)))
}

func TestOpenPullRequestRemovesItsBranchWhenThePullRequestFails(t *testing.T) {
	f := actionFixture(t)
	f.failPulls = true
	e := writeEngine(t, f, nil)
	req := openRequest("config/settings.yaml", "retries: 3", "retries: 5")
	plan, err := e.Plan(context.Background(), req)
	require.NoError(t, err)
	req.Precondition = plan.Precondition
	_, err = e.Execute(context.Background(), req)
	require.ErrorContains(t, err, "opening the pull request")
	assert.Equal(t, []string{"main"}, keys(f.repos["pipelines"].branches))
}

// stamped is an undo as whoever undoes it sends it.
func stamped(undo *pluginsdk.ActionRequest) pluginsdk.ActionRequest {
	return request(undo.Action, undo.Params)
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// openOne opens a pull request through the plugin and returns its number.
func openOne(t *testing.T, f *fakeGitHub, e *engine) int64 {
	t.Helper()
	_, result := planAndExecute(t, f, e, openRequest("config/settings.yaml", "retries: 3", "retries: 5"))
	return result.Undo.Params["number"].(int64)
}

func TestCloseAndReopenPullRequest(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	number := openOne(t, f, e)

	plan, result := planAndExecute(t, f, e, request(actionClosePullRequest, map[string]any{"repo": "pipelines", "number": number}))
	assert.Equal(t, "- state: open\n+ state: closed\n", plan.Diff)
	assert.Equal(t, "closed", f.repos["pipelines"].pulls[1].state)
	require.NotNil(t, result.Undo)
	assert.Equal(t, actionReopenPullRequest, result.Undo.Action)
	assert.Len(t, f.repos["pipelines"].branches, 2, "the branch is kept so the close can be undone")

	_, err := e.Plan(context.Background(), request(actionClosePullRequest, map[string]any{"repo": "pipelines", "number": number}))
	require.ErrorContains(t, err, "already closed")

	_, result = planAndExecute(t, f, e, stamped(result.Undo))
	assert.Equal(t, "open", f.repos["pipelines"].pulls[1].state)
	assert.Equal(t, actionClosePullRequest, result.Undo.Action)
}

func TestClosePullRequestRefusesAChangedPullRequest(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	number := openOne(t, f, e)
	req := request(actionClosePullRequest, map[string]any{"repo": "pipelines", "number": number})
	plan, err := e.Plan(context.Background(), req)
	require.NoError(t, err)

	f.repos["pipelines"].pulls[1].updated = f.tick()
	req.Precondition = plan.Precondition
	_, err = e.Execute(context.Background(), req)
	require.ErrorIs(t, err, ErrStale)
	assert.Equal(t, "open", f.repos["pipelines"].pulls[1].state)
}

func TestClosePullRequestOnlyTouchesItsOwn(t *testing.T) {
	f := actionFixture(t)
	r := f.repos["pipelines"]
	f.commit("pipelines", "feature", map[string]string{"x": "1"}, nil)
	f.commit("pipelines", "marmot/done", map[string]string{"x": "2"}, nil)
	r.pulls[1] = &fakePR{number: 1, state: "open", head: "feature", base: "main", headRepo: "acme/pipelines", user: "bob"}
	r.pulls[2] = &fakePR{number: 2, state: "open", head: "marmot/evil", base: "main", headRepo: "mallory/pipelines", user: "mallory"}
	r.pulls[3] = &fakePR{number: 3, state: "closed", head: "marmot/done", base: "main", headRepo: "acme/pipelines", merged: true}
	r.pulls[4] = &fakePR{number: 4, state: "closed", head: "marmot/gone", base: "main", headRepo: "acme/pipelines"}
	e := writeEngine(t, f, nil)

	for n, want := range map[int64]error{1: ErrNotOurs, 2: ErrNotOurs} {
		_, err := e.Plan(context.Background(), request(actionClosePullRequest, map[string]any{"repo": "pipelines", "number": n}))
		require.ErrorIs(t, err, want, n)
		assert.Equal(t, codes.PermissionDenied, status.Code(wireError(err)))
	}
	_, err := e.Plan(context.Background(), request(actionReopenPullRequest, map[string]any{"repo": "pipelines", "number": int64(3)}))
	require.ErrorContains(t, err, "merged")
	_, err = e.Plan(context.Background(), request(actionReopenPullRequest, map[string]any{"repo": "pipelines", "number": int64(4)}))
	require.ErrorContains(t, err, "has been deleted")
	_, err = e.Plan(context.Background(), request(actionClosePullRequest, map[string]any{"repo": "pipelines", "number": int64(99)}))
	assert.Equal(t, codes.NotFound, status.Code(wireError(err)))
	assert.Zero(t, f.writes())
}

func TestCommentAndDeleteComment(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	number := openOne(t, f, e)

	plan, result := planAndExecute(t, f, e, request(actionComment, map[string]any{"repo": "pipelines", "number": number, "body": "CI passed; ready for review."}))
	assert.Equal(t, f.repos["pipelines"].pulls[1].headSHA, plan.Precondition)
	assert.Contains(t, plan.Warnings[0], "notified")
	require.NotNil(t, result.Undo)
	assert.Equal(t, actionDeleteComment, result.Undo.Action)
	id := result.Undo.Params["comment_id"].(int64)
	c := f.comments[id]
	assert.True(t, strings.HasPrefix(c.body, "CI passed; ready for review.\n\n---\n_Posted by Marmot for `user:alice` (`task_run:42`)._"))
	assert.True(t, strings.HasSuffix(c.body, commentMarker))

	_, result = planAndExecute(t, f, e, stamped(result.Undo))
	assert.Empty(t, f.comments)
	require.NotNil(t, result.Undo)
	assert.Equal(t, actionComment, result.Undo.Action)
	assert.Equal(t, "CI passed; ready for review.", result.Undo.Params["body"], "undo posts the same text again")
	require.NoError(t, pluginsdk.ValidateParams(ActionSpecs()[3], result.Undo.Params))
}

func TestDeleteCommentOnlyDeletesItsOwn(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	number := openOne(t, f, e)
	now := f.tick()
	f.comments[1] = &fakeComment{id: 1, repo: "pipelines", issue: int(number), body: "LGTM", user: "bob", created: now, updated: now}
	f.comments[2] = &fakeComment{id: 2, repo: "pipelines", issue: int(number), body: "sneaky\n" + commentMarker, user: "bob", created: now, updated: now}

	for _, id := range []int64{1, 2} {
		_, err := e.Plan(context.Background(), request(actionDeleteComment, map[string]any{"repo": "pipelines", "comment_id": id}))
		require.ErrorIs(t, err, ErrNotOurs, id)
	}

	// With an installation token there is no /user; the app's bot is the
	// author instead.
	f.login = ""
	_, result := planAndExecute(t, f, e, request(actionComment, map[string]any{"repo": "pipelines", "number": number, "body": "from the app"}))
	_, err := e.Plan(context.Background(), request(actionDeleteComment, map[string]any{"repo": "pipelines", "comment_id": int64(2)}))
	require.ErrorIs(t, err, ErrNotOurs, "a person's comment with the marker pasted in")
	planAndExecute(t, f, e, stamped(result.Undo))
}

func TestCommentRefusesAMovedHead(t *testing.T) {
	f := actionFixture(t)
	e := writeEngine(t, f, nil)
	number := openOne(t, f, e)
	req := request(actionComment, map[string]any{"repo": "pipelines", "number": number, "body": "looks fine"})
	plan, err := e.Plan(context.Background(), req)
	require.NoError(t, err)

	f.repos["pipelines"].pulls[1].headSHA = "0000000000000000000000000000000000000000"
	req.Precondition = plan.Precondition
	_, err = e.Execute(context.Background(), req)
	require.ErrorIs(t, err, ErrStale)
	assert.Empty(t, f.comments)
}

func TestThroughSource(t *testing.T) {
	f := actionFixture(t)
	s := &Source{}
	raw := f.config(map[string]any{"actions": map[string]any{"repos": []string{"pipelines"}}})
	req := openRequest("config/settings.yaml", "retries: 3", "retries: 5")
	plan, err := s.PlanAction(context.Background(), raw, req)
	require.NoError(t, err)
	req.Precondition = plan.Precondition
	result, err := s.ExecuteAction(context.Background(), raw, req)
	require.NoError(t, err)
	assert.NotNil(t, result.Undo)

	_, err = s.PlanAction(context.Background(), raw, openRequest(".github/workflows/ci.yml", "on: pull_request", "on: push"))
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestWireErrorCodes(t *testing.T) {
	assert.Equal(t, codes.FailedPrecondition, status.Code(wireError(fmt.Errorf("x: %w", ErrStale))))
	assert.Equal(t, codes.PermissionDenied, status.Code(wireError(fmt.Errorf("x: %w", ErrOutOfScope))))
	assert.Equal(t, codes.PermissionDenied, status.Code(wireError(fmt.Errorf("x: %w", ErrNotOurs))))
	assert.Equal(t, codes.InvalidArgument, status.Code(wireError(fmt.Errorf("x: %w", ErrBadPath))))
	assert.Equal(t, codes.PermissionDenied, status.Code(wireError(&apiError{Status: 403})))
	assert.Equal(t, codes.FailedPrecondition, status.Code(wireError(&apiError{Status: 422})))
	assert.Nil(t, wireError(nil))
}
