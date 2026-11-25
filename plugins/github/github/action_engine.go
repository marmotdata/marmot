package github

import (
	"context"
	"crypto/sha1" //nolint:gosec // G505: git's own object id, not a security hash
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

const (
	// commentMarker is appended, hidden, to every comment the plugin posts,
	// so delete_comment can refuse comments it did not write.
	commentMarker = "<!-- created-by: marmot -->"
	// footerRule starts the footer the plugin appends to what it writes.
	footerRule = "\n\n---\n_"
	// symlinkMode and submoduleType are git entries that are not files.
	symlinkMode   = "120000"
	submoduleType = "commit"
	diffContext   = 3
)

// engine plans and executes the plugin's actions against one owner.
type engine struct {
	client *client
	config *Config
}

func newEngine(rawConfig pluginsdk.RawConfig, origin string) (*engine, error) {
	config, err := loadConfig(rawConfig)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	// The SDK has already refused an origin that is not printable ASCII
	// without spaces, so it cannot split the header.
	c, err := newClient(config.BaseURL, config.token(), fmt.Sprintf("marmot-plugin-github/actions (origin %s)", origin))
	if err != nil {
		return nil, err
	}
	return &engine{client: c, config: config}, nil
}

// prepared is what a write would do, worked out against live state.
type prepared struct {
	target       string
	kind         pluginsdk.TargetKind
	summary      string
	before       any
	after        any
	diff         string
	precondition string
	warnings     []string
	// run makes the change; only Execute calls it.
	run func(ctx context.Context) (after any, undo *pluginsdk.ActionRequest, err error)
}

// Plan previews a write. Every call it makes is a read.
func (e *engine) Plan(ctx context.Context, req pluginsdk.ActionRequest) (*pluginsdk.ActionPlan, error) {
	if _, err := pluginsdk.CheckActionRequest(ActionSpecs(), req, false); err != nil {
		return nil, err
	}
	p, err := e.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	before, after, err := marshalPair(p.before, p.after)
	if err != nil {
		return nil, err
	}
	return &pluginsdk.ActionPlan{
		Target:       p.target,
		TargetKind:   p.kind,
		Summary:      p.summary,
		Before:       before,
		After:        after,
		Diff:         p.diff,
		Precondition: p.precondition,
		Warnings:     p.warnings,
	}, nil
}

// Execute runs a read, or a write only if its target is still in the state
// its plan saw. GitHub has no conditional write for most of these calls, so
// the check is a fresh read just before the write; open_pull_request is the
// exception that needs none, since its commit is built on the planned base
// commit itself and never on whatever the branch points at by then.
func (e *engine) Execute(ctx context.Context, req pluginsdk.ActionRequest) (*pluginsdk.ActionResult, error) {
	if isRead(req.Action) {
		return e.read(ctx, req)
	}
	if _, err := pluginsdk.CheckActionRequest(ActionSpecs(), req, true); err != nil {
		return nil, err
	}
	p, err := e.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	if p.precondition != req.Precondition {
		return nil, fmt.Errorf("%s %w (planned at %s, now %s)", p.target, ErrStale, req.Precondition, p.precondition)
	}
	afterState, undo, err := p.run(ctx)
	if err != nil {
		return nil, err
	}
	before, after, err := marshalPair(p.before, afterState)
	if err != nil {
		// The change has happened; failing now would report it as not.
		before, after = nil, nil
	}
	return &pluginsdk.ActionResult{
		Target:     p.target,
		TargetKind: p.kind,
		Summary:    p.summary,
		Before:     before,
		After:      after,
		Undo:       undo,
		Warnings:   p.warnings,
	}, nil
}

func marshalPair(before, after any) (json.RawMessage, json.RawMessage, error) {
	b, err := json.Marshal(before)
	if err != nil {
		return nil, nil, err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return nil, nil, err
	}
	return b, a, nil
}

func (e *engine) prepare(ctx context.Context, req pluginsdk.ActionRequest) (*prepared, error) {
	repo := req.StringParam("repo")
	// Refused before anything is read from the repository.
	if !e.config.Actions.writable(repo) {
		return nil, fmt.Errorf("repository %s is %w: allow it in actions.repos", repo, ErrOutOfScope)
	}
	r, err := repositoryInScope(ctx, e.client, e.config, repo)
	if err != nil {
		return nil, err
	}
	if r.Archived {
		return nil, fmt.Errorf("repository %s is archived and read-only", r.FullName)
	}
	switch req.Action {
	case actionOpenPullRequest:
		return e.openPullRequest(ctx, r, req)
	case actionClosePullRequest:
		return e.setPullRequestState(ctx, r, req, "closed")
	case actionReopenPullRequest:
		return e.setPullRequestState(ctx, r, req, "open")
	case actionComment:
		return e.comment(ctx, r, req)
	case actionDeleteComment:
		return e.deleteComment(ctx, r, req)
	}
	return nil, fmt.Errorf("unknown action %q", req.Action)
}

// edit is one slot of open_pull_request.
type edit struct {
	path, old, new string
}

func editsOf(req pluginsdk.ActionRequest) ([]edit, error) {
	var edits []edit
	for i := 1; i <= maxEdits; i++ {
		p, okPath := req.Params[fmt.Sprintf("path_%d", i)]
		o, okOld := req.Params[fmt.Sprintf("old_%d", i)]
		n, okNew := req.Params[fmt.Sprintf("new_%d", i)]
		if !okPath && !okOld && !okNew {
			continue
		}
		if !okPath || !okOld || !okNew {
			return nil, fmt.Errorf("edit %d needs path_%d, old_%d and new_%d together", i, i, i, i)
		}
		ed := edit{path: p.(string), old: o.(string), new: n.(string)}
		if ed.old == "" {
			return nil, fmt.Errorf("old_%d is empty: an edit replaces text that is already in the file", i)
		}
		if ed.old == ed.new {
			return nil, fmt.Errorf("edit %d changes nothing", i)
		}
		edits = append(edits, ed)
	}
	return edits, nil
}

// checkPath refuses a path that is not a plain path inside the tree: no
// absolute paths, traversal, empty segments or .git, whatever the pattern
// already let through.
func checkPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return fmt.Errorf("%q is %w", p, ErrBadPath)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.EqualFold(seg, ".git") {
			return fmt.Errorf("%q is %w", p, ErrBadPath)
		}
	}
	return nil
}

// isCIPath reports whether a path defines CI. A pull request from a branch
// in the same repository runs its own version of these, with the
// repository's secrets, before anyone reviews it. Matched without case so
// no spelling slips past on a case-insensitive checkout.
func isCIPath(p string) bool {
	lower := strings.ToLower(p)
	return strings.HasPrefix(lower, ".github/workflows/") || strings.HasPrefix(lower, ".github/actions/")
}

var refSegment = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)

// checkRef holds a ref to git's own rules, within what the pattern allows.
func checkRef(ref string) error {
	if strings.HasSuffix(ref, ".lock") || strings.HasSuffix(ref, ".") || strings.Contains(ref, "..") {
		return fmt.Errorf("%q is not a valid branch, tag or commit", ref)
	}
	for _, seg := range strings.Split(ref, "/") {
		if !refSegment.MatchString(seg) {
			return fmt.Errorf("%q is not a valid branch, tag or commit", ref)
		}
	}
	return nil
}

// fileChange is one file open_pull_request rewrites.
type fileChange struct {
	path    string
	mode    string
	oldSHA  string
	newSHA  string
	oldText string
	newText string
}

func (e *engine) openPullRequest(ctx context.Context, r *repository, req pluginsdk.ActionRequest) (*prepared, error) {
	edits, err := editsOf(req)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, ed := range edits {
		if err := checkPath(ed.path); err != nil {
			return nil, err
		}
		if isCIPath(ed.path) && !e.config.Actions.AllowCIChanges {
			return nil, fmt.Errorf("%s is %w, which a pull request from this repository would run unreviewed with its secrets; set actions.allow_ci_changes to allow it: %w", ed.path, ErrCIPath, ErrOutOfScope)
		}
		if !slices.Contains(paths, ed.path) {
			paths = append(paths, ed.path)
		}
	}
	if limit := e.config.Actions.maxFiles(); len(paths) > limit {
		return nil, fmt.Errorf("%d files is above this runner's actions.max_files of %d", len(paths), limit)
	}

	base := req.StringParam("base")
	if base == "" {
		base = r.DefaultBranch
	}
	if err := checkRef(base); err != nil {
		return nil, err
	}
	var ref gitRef
	if err := e.client.do(ctx, "GET", repoPath(e.config.Owner, r.Name)+"/git/ref/heads/"+escapePath(base), nil, nil, &ref); err != nil {
		return nil, fmt.Errorf("reading branch %s: %w", base, err)
	}
	baseSHA := ref.Object.SHA
	var commit gitCommit
	if err := e.client.do(ctx, "GET", repoPath(e.config.Owner, r.Name)+"/git/commits/"+url.PathEscape(baseSHA), nil, nil, &commit); err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", baseSHA, err)
	}

	walker := &treeWalker{client: e.client, repo: repoPath(e.config.Owner, r.Name), cache: map[string]*tree{}}
	var files []*fileChange
	for _, p := range paths {
		fc, err := e.readForEdit(ctx, walker, commit.Tree.SHA, p)
		if err != nil {
			return nil, err
		}
		files = append(files, fc)
	}
	for i, ed := range edits {
		fc := files[slices.Index(paths, ed.path)]
		if n := strings.Count(fc.newText, ed.old); n != 1 {
			return nil, fmt.Errorf("old_%d occurs %d times in %s at %s; it must occur exactly once", i+1, n, ed.path, shortSHA(baseSHA))
		}
		fc.newText = strings.Replace(fc.newText, ed.old, ed.new, 1)
	}
	for _, fc := range files {
		if int64(len(fc.newText)) > e.config.Actions.maxFileBytes() {
			return nil, fmt.Errorf("%s would be %d bytes, above this runner's actions.max_file_bytes of %d", fc.path, len(fc.newText), e.config.Actions.maxFileBytes())
		}
		if fc.newText == fc.oldText {
			return nil, fmt.Errorf("the edits leave %s unchanged", fc.path)
		}
		fc.newSHA = blobSHA(fc.newText)
	}

	prefix := e.config.Actions.branchPrefix()
	branch := prefix + branchName(req)
	switch err := e.client.do(ctx, "GET", repoPath(e.config.Owner, r.Name)+"/git/ref/heads/"+escapePath(branch), nil, nil, &gitRef{}); {
	case err == nil:
		return nil, fmt.Errorf("branch %s already exists, so this pull request may already be open; change the title or branch to open another", branch)
	case !isNotFound(err):
		return nil, fmt.Errorf("checking for branch %s: %w", branch, err)
	}

	title, body := req.StringParam("title"), req.StringParam("body")
	beforeFiles, afterFiles := map[string]any{}, map[string]any{}
	var diff strings.Builder
	for _, fc := range files {
		beforeFiles[fc.path] = map[string]string{"blob": fc.oldSHA}
		afterFiles[fc.path] = map[string]string{"blob": fc.newSHA}
		diff.WriteString(unifiedDiff(fc.path, fc.oldText, fc.newText))
	}
	fullName := e.config.Owner + "/" + r.Name
	p := &prepared{
		target:       fmt.Sprintf("%s:%s", fullName, branch),
		kind:         kindPullRequest,
		summary:      fmt.Sprintf("Open a pull request on %s from %s into %s changing %d file(s): %s", fullName, branch, base, len(files), title),
		before:       map[string]any{"base": base, "base_sha": baseSHA, "files": beforeFiles},
		after:        map[string]any{"branch": branch, "files": afterFiles, "pull_request": map[string]any{"title": title, "base": base, "head": branch}},
		diff:         diff.String(),
		precondition: baseSHA,
		warnings:     []string{"workflows that run on pull_request will run against the new branch"},
	}
	if base != r.DefaultBranch {
		p.warnings = append(p.warnings, fmt.Sprintf("the pull request targets %s, not the default branch %s", base, r.DefaultBranch))
	}

	p.run = func(ctx context.Context) (any, *pluginsdk.ActionRequest, error) {
		repo := repoPath(e.config.Owner, r.Name)
		entries := make([]map[string]string, 0, len(files))
		for _, fc := range files {
			entries = append(entries, map[string]string{"path": fc.path, "mode": fc.mode, "type": "blob", "content": fc.newText})
		}
		var newTree tree
		if err := e.client.do(ctx, "POST", repo+"/git/trees", nil, map[string]any{"base_tree": commit.Tree.SHA, "tree": entries}, &newTree); err != nil {
			return nil, nil, fmt.Errorf("creating the tree: %w", err)
		}
		message := fmt.Sprintf("%s\n\nMarmot-Origin: %s\nMarmot-Actor: %s\n", title, req.Origin, req.Identity)
		var newCommit gitCommit
		if err := e.client.do(ctx, "POST", repo+"/git/commits", nil, map[string]any{"message": message, "tree": newTree.SHA, "parents": []string{baseSHA}}, &newCommit); err != nil {
			return nil, nil, fmt.Errorf("creating the commit: %w", err)
		}
		// Creating a ref fails if it exists, so this can never move a
		// branch someone else made in the meantime.
		if err := e.client.do(ctx, "POST", repo+"/git/refs", nil, map[string]string{"ref": "refs/heads/" + branch, "sha": newCommit.SHA}, nil); err != nil {
			return nil, nil, fmt.Errorf("creating branch %s: %w", branch, err)
		}
		var pr pullRequest
		prBody := strings.TrimSpace(body) + footer("Opened", req)
		if err := e.client.do(ctx, "POST", repo+"/pulls", nil, map[string]any{"title": title, "head": branch, "base": base, "body": prBody}, &pr); err != nil {
			// The branch is the plugin's own and was made a moment ago, so
			// it is removed rather than left behind with nothing pointing
			// at it.
			if derr := e.client.do(ctx, "DELETE", repo+"/git/refs/heads/"+escapePath(branch), nil, nil, nil); derr != nil {
				log.Warn().Err(derr).Str("branch", branch).Msg("Failed to delete the branch of a pull request that could not be opened")
			}
			return nil, nil, fmt.Errorf("opening the pull request: %w", err)
		}
		after := map[string]any{"branch": branch, "commit": newCommit.SHA, "files": afterFiles,
			"pull_request": map[string]any{"number": pr.Number, "url": pr.HTMLURL, "title": title, "base": base, "head": branch}}
		undo := &pluginsdk.ActionRequest{Action: actionClosePullRequest, Params: map[string]any{"repo": r.Name, "number": int64(pr.Number)}}
		return after, undo, nil
	}
	return p, nil
}

func (e *engine) readForEdit(ctx context.Context, w *treeWalker, rootTree, path string) (*fileChange, error) {
	entry, err := w.lookup(ctx, rootTree, path)
	if err != nil {
		return nil, err
	}
	switch {
	case entry.Type == submoduleType:
		return nil, fmt.Errorf("%s is a submodule, not a file", path)
	case entry.Type != "blob":
		return nil, fmt.Errorf("%s is a directory, not a file", path)
	case entry.Mode == symlinkMode:
		return nil, fmt.Errorf("%s is a symbolic link; edit the file it points at", path)
	case entry.Size > e.config.Actions.maxFileBytes():
		return nil, fmt.Errorf("%s is %d bytes, above this runner's actions.max_file_bytes of %d", path, entry.Size, e.config.Actions.maxFileBytes())
	}
	data, err := w.blob(ctx, entry.SHA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if isBinary(data) {
		return nil, fmt.Errorf("%s is binary", path)
	}
	return &fileChange{path: path, mode: entry.Mode, oldSHA: entry.SHA, oldText: string(data), newText: string(data)}, nil
}

// treeWalker finds paths one directory at a time, which reads only the
// trees on the way rather than the whole repository.
type treeWalker struct {
	client *client
	repo   string
	cache  map[string]*tree
}

func (w *treeWalker) tree(ctx context.Context, sha string, recursive bool) (*tree, error) {
	key := sha
	var query url.Values
	if recursive {
		key += "?recursive"
		query = url.Values{"recursive": {"1"}}
	}
	if t, ok := w.cache[key]; ok {
		return t, nil
	}
	var t tree
	if err := w.client.do(ctx, "GET", w.repo+"/git/trees/"+url.PathEscape(sha), query, nil, &t); err != nil {
		return nil, err
	}
	w.cache[key] = &t
	return &t, nil
}

// lookup returns the entry at path, or the root itself for "".
func (w *treeWalker) lookup(ctx context.Context, rootTree, path string) (treeEntry, error) {
	cur := treeEntry{Path: "", Type: "tree", SHA: rootTree, Mode: "040000"}
	if path == "" {
		return cur, nil
	}
	for _, seg := range strings.Split(path, "/") {
		if cur.Type != "tree" {
			return treeEntry{}, fmt.Errorf("%s does not exist: %s is not a directory: %w", path, cur.Path, errNoPath)
		}
		t, err := w.tree(ctx, cur.SHA, false)
		if err != nil {
			return treeEntry{}, fmt.Errorf("reading the tree of %s: %w", path, err)
		}
		i := slices.IndexFunc(t.Tree, func(e treeEntry) bool { return e.Path == seg })
		if i < 0 {
			return treeEntry{}, fmt.Errorf("%s does not exist: %w", path, errNoPath)
		}
		name := seg
		if cur.Path != "" {
			name = cur.Path + "/" + seg
		}
		cur = t.Tree[i]
		cur.Path = name
	}
	return cur, nil
}

var errNoPath = errors.New("no such path")

func (w *treeWalker) blob(ctx context.Context, sha string) ([]byte, error) {
	var b blob
	if err := w.client.do(ctx, "GET", w.repo+"/git/blobs/"+url.PathEscape(sha), nil, nil, &b); err != nil {
		return nil, err
	}
	if b.Encoding != "base64" {
		return []byte(b.Content), nil
	}
	return base64.StdEncoding.DecodeString(strings.ReplaceAll(b.Content, "\n", ""))
}

// blobSHA is git's object id for a blob, so the plan names the exact blob
// the commit will hold.
func blobSHA(content string) string {
	h := sha1.New() //nolint:gosec // G401: git's object id
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// branchName is the same for a plan and its execution, which carry the
// same origin and parameters, and differs for any other request, so
// executing twice finds the branch already there instead of opening a
// second pull request.
func branchName(req pluginsdk.ActionRequest) string {
	name := req.StringParam("branch")
	if name == "" {
		name = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(req.StringParam("title")), "-"), "-")
		if len(name) > 40 {
			name = strings.TrimRight(name[:40], "-")
		}
		if name == "" {
			name = "change"
		}
	}
	params, _ := json.Marshal(req.Params)
	sum := sha256.Sum256([]byte(req.Origin + "\x00" + string(params)))
	return name + "-" + hex.EncodeToString(sum[:])[:8]
}

// footer is appended to everything the plugin writes for people to read,
// naming who asked and from where. Identity and origin are printable ASCII
// without spaces; backticks are dropped so neither can close the code span.
func footer(verb string, req pluginsdk.ActionRequest) string {
	clean := func(s string) string { return strings.ReplaceAll(s, "`", "") }
	return fmt.Sprintf("%s%s by Marmot for `%s` (`%s`)._\n%s", footerRule, verb, clean(req.Identity), clean(req.Origin), commentMarker)
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// unifiedDiff renders a change that keeps the line count, as every edit
// open_pull_request makes does, since no parameter can hold a line break.
func unifiedDiff(path, before, after string) string {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(a) != len(b) {
		return fmt.Sprintf("--- a/%s\n+++ b/%s\n(line count changed from %d to %d)\n", path, path, len(a), len(b))
	}
	var changed []int
	for i := range a {
		if a[i] != b[i] {
			changed = append(changed, i)
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n", path, path)
	for i := 0; i < len(changed); {
		start := max(changed[i]-diffContext, 0)
		end := changed[i]
		j := i
		// Changes close enough to share context share a hunk.
		for j < len(changed) && changed[j] <= end+2*diffContext {
			end = changed[j]
			j++
		}
		end = min(end+diffContext, len(a)-1)
		n := end - start + 1
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", start+1, n, start+1, n)
		for k := start; k <= end; k++ {
			if a[k] == b[k] {
				fmt.Fprintf(&out, " %s\n", a[k])
				continue
			}
			fmt.Fprintf(&out, "-%s\n+%s\n", a[k], b[k])
		}
		i = j
	}
	return out.String()
}

// pullRequestPrecondition changes with any push to the pull request and
// any change to it at all, such as a comment, a label or a state change.
func pullRequestPrecondition(pr *pullRequest) string {
	return pr.Head.SHA + "@" + pr.UpdatedAt.UTC().Format(time.RFC3339)
}

func (e *engine) getPullRequest(ctx context.Context, r *repository, number int64) (*pullRequest, error) {
	var pr pullRequest
	if err := e.client.do(ctx, "GET", repoPath(e.config.Owner, r.Name)+"/pulls/"+strconv.FormatInt(number, 10), nil, nil, &pr); err != nil {
		return nil, fmt.Errorf("reading pull request %s#%d: %w", r.FullName, number, err)
	}
	return &pr, nil
}

// isOurs reports whether the plugin opened a pull request: its head is a
// branch under the prefix in this same repository. A fork can name a
// branch anything, so the head repository must be this one.
func (e *engine) isOurs(r *repository, pr *pullRequest) bool {
	return pr.Head.Repo != nil && strings.EqualFold(pr.Head.Repo.FullName, e.config.Owner+"/"+r.Name) &&
		strings.HasPrefix(pr.Head.Ref, e.config.Actions.branchPrefix())
}

func (e *engine) setPullRequestState(ctx context.Context, r *repository, req pluginsdk.ActionRequest, want string) (*prepared, error) {
	number, _ := req.IntParam("number")
	pr, err := e.getPullRequest(ctx, r, number)
	if err != nil {
		return nil, err
	}
	target := fmt.Sprintf("%s/%s#%d", e.config.Owner, r.Name, number)
	if !e.isOurs(r, pr) {
		return nil, fmt.Errorf("%s is %w: its head is not a %s branch in this repository", target, ErrNotOurs, e.config.Actions.branchPrefix())
	}
	if pr.MergedAt != nil {
		return nil, fmt.Errorf("%s is merged", target)
	}
	if pr.State == want {
		return nil, fmt.Errorf("%s is already %s", target, want)
	}
	if want == "open" {
		err := e.client.do(ctx, "GET", repoPath(e.config.Owner, r.Name)+"/git/ref/heads/"+escapePath(pr.Head.Ref), nil, nil, &gitRef{})
		if isNotFound(err) {
			return nil, fmt.Errorf("%s cannot be reopened: its branch %s has been deleted", target, pr.Head.Ref)
		}
		if err != nil {
			return nil, fmt.Errorf("checking branch %s: %w", pr.Head.Ref, err)
		}
	}

	verb, inverse := "Close", actionReopenPullRequest
	if want == "open" {
		verb, inverse = "Reopen", actionClosePullRequest
	}
	return &prepared{
		target:       target,
		kind:         kindPullRequest,
		summary:      fmt.Sprintf("%s %s from %s", verb, target, pr.Head.Ref),
		before:       map[string]string{"state": pr.State},
		after:        map[string]string{"state": want},
		diff:         fmt.Sprintf("- state: %s\n+ state: %s\n", pr.State, want),
		precondition: pullRequestPrecondition(pr),
		run: func(ctx context.Context) (any, *pluginsdk.ActionRequest, error) {
			var updated pullRequest
			if err := e.client.do(ctx, "PATCH", repoPath(e.config.Owner, r.Name)+"/pulls/"+strconv.FormatInt(number, 10), nil, map[string]string{"state": want}, &updated); err != nil {
				return nil, nil, fmt.Errorf("setting %s %s: %w", target, want, err)
			}
			undo := &pluginsdk.ActionRequest{Action: inverse, Params: map[string]any{"repo": r.Name, "number": number}}
			return map[string]string{"state": updated.State}, undo, nil
		},
	}, nil
}

func (e *engine) comment(ctx context.Context, r *repository, req pluginsdk.ActionRequest) (*prepared, error) {
	number, _ := req.IntParam("number")
	pr, err := e.getPullRequest(ctx, r, number)
	if err != nil {
		return nil, err
	}
	target := fmt.Sprintf("%s/%s#%d", e.config.Owner, r.Name, number)
	if pr.State != "open" {
		return nil, fmt.Errorf("%s is %s; only open pull requests take comments", target, pr.State)
	}
	text := req.StringParam("body")
	body := text + footer("Posted", req)
	return &prepared{
		target:  target,
		kind:    kindPullRequest,
		summary: fmt.Sprintf("Comment on %s", target),
		before:  map[string]any{"head_sha": pr.Head.SHA},
		after:   map[string]any{"head_sha": pr.Head.SHA, "comment": body},
		diff:    "+ comment: " + strconv.Quote(text) + "\n",
		// The comment is about the code as it stands; once someone pushes,
		// it may no longer be true.
		precondition: pr.Head.SHA,
		warnings:     []string{"people watching the pull request are notified when the comment is posted, and deleting it later does not recall that"},
		run: func(ctx context.Context) (any, *pluginsdk.ActionRequest, error) {
			var c issueComment
			if err := e.client.do(ctx, "POST", repoPath(e.config.Owner, r.Name)+"/issues/"+strconv.FormatInt(number, 10)+"/comments", nil, map[string]string{"body": body}, &c); err != nil {
				return nil, nil, fmt.Errorf("commenting on %s: %w", target, err)
			}
			undo := &pluginsdk.ActionRequest{Action: actionDeleteComment, Params: map[string]any{"repo": r.Name, "comment_id": c.ID}}
			return map[string]any{"head_sha": pr.Head.SHA, "comment_id": c.ID, "url": c.HTMLURL}, undo, nil
		},
	}, nil
}

var issueNumber = regexp.MustCompile(`/issues/([0-9]+)$`)

func (e *engine) deleteComment(ctx context.Context, r *repository, req pluginsdk.ActionRequest) (*prepared, error) {
	id, _ := req.IntParam("comment_id")
	repo := repoPath(e.config.Owner, r.Name)
	var c issueComment
	if err := e.client.do(ctx, "GET", repo+"/issues/comments/"+strconv.FormatInt(id, 10), nil, nil, &c); err != nil {
		return nil, fmt.Errorf("reading comment %d: %w", id, err)
	}
	target := fmt.Sprintf("%s/%s comment %d", e.config.Owner, r.Name, id)

	issueURL, err := url.Parse(c.IssueURL)
	m := issueNumber.FindStringSubmatch(c.IssueURL)
	if err != nil || m == nil || !strings.HasSuffix(issueURL.Path, repo+"/issues/"+m[1]) {
		return nil, fmt.Errorf("%s is not on an issue or pull request of this repository", target)
	}
	number, _ := strconv.ParseInt(m[1], 10, 64)
	if _, err := e.getPullRequest(ctx, r, number); err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%s is on an issue, not a pull request: %w", target, ErrNotOurs)
		}
		return nil, err
	}
	if !strings.HasSuffix(strings.TrimSpace(c.Body), commentMarker) {
		return nil, fmt.Errorf("%s is %w: it has no Marmot marker", target, ErrNotOurs)
	}
	// The marker is plain text anyone can paste, so the author must also be
	// the token's own account. An installation token has no /user; its
	// comments are posted by the app's bot, which GitHub records as such.
	var me account
	switch err := e.client.do(ctx, "GET", "/user", nil, nil, &me); {
	case err == nil:
		if !strings.EqualFold(me.Login, c.User.Login) {
			return nil, fmt.Errorf("%s is %w: it was written by %s", target, ErrNotOurs, c.User.Login)
		}
	case statusOf(err) == http.StatusForbidden || statusOf(err) == http.StatusUnauthorized:
		if c.PerformedViaGitHubApp == nil || c.User.Type != "Bot" {
			return nil, fmt.Errorf("%s is %w: it was written by %s, not a GitHub App", target, ErrNotOurs, c.User.Login)
		}
	default:
		return nil, fmt.Errorf("checking who the token is: %w", err)
	}

	// What was posted, less the footer, is what posting it again needs.
	text := c.Body
	if i := strings.LastIndex(text, footerRule); i >= 0 {
		text = text[:i]
	}
	return &prepared{
		target:       target,
		kind:         kindComment,
		summary:      fmt.Sprintf("Delete %s on pull request #%d", target, number),
		before:       map[string]any{"comment": c.Body},
		after:        map[string]any{},
		diff:         "- comment: " + strconv.Quote(text) + "\n",
		precondition: c.UpdatedAt.UTC().Format(time.RFC3339Nano),
		run: func(ctx context.Context) (any, *pluginsdk.ActionRequest, error) {
			if err := e.client.do(ctx, "DELETE", repo+"/issues/comments/"+strconv.FormatInt(id, 10), nil, nil, nil); err != nil {
				return nil, nil, fmt.Errorf("deleting %s: %w", target, err)
			}
			undo := &pluginsdk.ActionRequest{Action: actionComment, Params: map[string]any{"repo": r.Name, "number": number, "body": text}}
			return map[string]any{}, undo, nil
		},
	}, nil
}
