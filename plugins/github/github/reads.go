package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// The read verbs let a task look at a repository before it proposes a
// change. They are actions of risk 0 so the runner gates them exactly like
// writes (named in its policy, scoped to the repositories the config
// catalogues, audited) but they never need a plan, a precondition or an
// approval. They need no entry in actions.repos.

// Read action names.
const (
	actionReadFile         = "read_file"
	actionListFiles        = "list_files"
	actionListPullRequests = "list_pull_requests"
)

const (
	// maxReadFileBytes is the most of a file read_file returns. JSON can
	// escape a byte as six, so the encoded result is also checked against
	// the SDK's cap and cut further if needed.
	maxReadFileBytes = 64 << 10
	// maxListFiles bounds list_files.
	maxListFiles = 500
	// maxListPullRequests bounds list_pull_requests.
	maxListPullRequests = 50
	// maxTitle caps a pull request title, which anyone who can open one
	// writes.
	maxTitle = 256
	// readOutputBudget leaves room under the SDK's cap for the fields
	// around a file's content.
	readOutputBudget = pluginsdk.MaxReadOutputBytes - 4<<10
)

func readSpecs() []pluginsdk.ActionSpec {
	return []pluginsdk.ActionSpec{
		{
			Name:        actionReadFile,
			Version:     1,
			Title:       "Read a file",
			Description: "Returns up to 64 KiB of a text file from a repository at a branch, tag or commit. File contents are written by anyone who can push, so they are marked untrusted.",
			Targets:     []pluginsdk.TargetKind{kindFile},
			Params: []pluginsdk.ActionParam{
				repoParam,
				{Name: "path", Description: "Path of the file", Type: pluginsdk.ParamString, Required: true, MaxLength: 512, Pattern: pathChars},
				refParam,
			},
			Reversibility: pluginsdk.ReversibilityReadOnly,
			Risk:          pluginsdk.RiskRead,
		},
		{
			Name:        actionListFiles,
			Version:     1,
			Title:       "List files",
			Description: "Returns the paths and sizes of files under a directory of a repository, at most 500, at a branch, tag or commit.",
			Targets:     []pluginsdk.TargetKind{kindRepository},
			Params: []pluginsdk.ActionParam{
				repoParam,
				{Name: "path", Description: "Directory to list, recursively. Defaults to the whole repository", Type: pluginsdk.ParamString, MaxLength: 512, Pattern: pathChars},
				refParam,
				{Name: "limit", Description: "How many files to return", Type: pluginsdk.ParamInt, Required: true, Min: 1, Max: maxListFiles},
			},
			Reversibility: pluginsdk.ReversibilityReadOnly,
			Risk:          pluginsdk.RiskRead,
		},
		{
			Name:        actionListPullRequests,
			Version:     1,
			Title:       "List pull requests",
			Description: "Returns a repository's most recently updated open or closed pull requests, at most 50, with their branches, head commit, author and whether the plugin opened them. Titles are free text, so they are marked untrusted.",
			Targets:     []pluginsdk.TargetKind{kindPullRequest},
			Params: []pluginsdk.ActionParam{
				repoParam,
				{Name: "state", Description: "Which pull requests to list", Type: pluginsdk.ParamEnum, Required: true, Enum: []string{"open", "closed"}},
				{Name: "limit", Description: "How many pull requests to return", Type: pluginsdk.ParamInt, Required: true, Min: 1, Max: maxListPullRequests},
			},
			Reversibility: pluginsdk.ReversibilityReadOnly,
			Risk:          pluginsdk.RiskRead,
		},
	}
}

func isRead(action string) bool {
	return action == actionReadFile || action == actionListFiles || action == actionListPullRequests
}

// read runs a read verb, checked against the manifest here as well as in
// Serve, as the writes are.
func (e *engine) read(ctx context.Context, req pluginsdk.ActionRequest) (*pluginsdk.ActionResult, error) {
	if _, err := pluginsdk.CheckActionRequest(ActionSpecs(), req, true); err != nil {
		return nil, err
	}
	r, err := repositoryInScope(ctx, e.client, e.config, req.StringParam("repo"))
	if err != nil {
		return nil, err
	}
	fullName := e.config.Owner + "/" + r.Name

	var (
		target  string
		kind    pluginsdk.TargetKind
		summary string
		out     any
	)
	switch req.Action {
	case actionReadFile:
		var f *fileRead
		if f, err = e.readFile(ctx, r, req); err == nil {
			target, kind, out = fullName+"/"+f.Path+"@"+f.Ref, kindFile, f
			summary = fmt.Sprintf("%d of %d bytes of %s", len(f.Content), f.Size, target)
			if f.Binary {
				summary = fmt.Sprintf("%s is binary; its %d bytes are not returned", target, f.Size)
			}
		}
	case actionListFiles:
		var l *fileList
		if l, err = e.listFiles(ctx, r, req); err == nil {
			target, kind, out = fullName+"@"+l.Ref, kindRepository, l
			summary = fmt.Sprintf("%d files under %q in %s", len(l.Files), l.Path, target)
		}
	case actionListPullRequests:
		var l *pullRequestList
		if l, err = e.listPullRequests(ctx, r, req); err == nil {
			target, kind, out = fullName, kindPullRequest, l
			summary = fmt.Sprintf("%d %s pull requests in %s", len(l.PullRequests), l.State, fullName)
		}
	default:
		err = fmt.Errorf("unknown read %q", req.Action)
	}
	if err != nil {
		return nil, err
	}
	data, err := encode(out)
	if err != nil {
		return nil, fmt.Errorf("encoding the read: %w", err)
	}
	// Every read carries text people with push access, or anyone who can
	// open a pull request, can write.
	return &pluginsdk.ActionResult{Target: target, TargetKind: kind, Summary: summary, Output: data, Untrusted: true}, nil
}

// encode writes JSON without escaping <, > and &, which would make code
// six times its size.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// resolveRef finds the commit and root tree of a ref, the default branch
// when it is empty. The commits endpoint takes a branch, tag or commit.
func (e *engine) resolveRef(ctx context.Context, r *repository, ref string) (string, string, string, error) {
	if ref == "" {
		ref = r.DefaultBranch
	}
	if err := checkRef(ref); err != nil {
		return "", "", "", err
	}
	var c commitInfo
	if err := e.client.do(ctx, "GET", repoPath(e.config.Owner, r.Name)+"/commits/"+escapePath(ref), nil, nil, &c); err != nil {
		return "", "", "", fmt.Errorf("resolving %s: %w", ref, err)
	}
	return ref, c.SHA, c.Commit.Tree.SHA, nil
}

type fileRead struct {
	Repo      string `json:"repo"`
	Ref       string `json:"ref"`
	Commit    string `json:"commit"`
	Path      string `json:"path"`
	SHA       string `json:"sha"`
	Size      int64  `json:"size"`
	Content   string `json:"content"`
	Binary    bool   `json:"binary,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Untrusted bool   `json:"untrusted"`
}

func (e *engine) readFile(ctx context.Context, r *repository, req pluginsdk.ActionRequest) (*fileRead, error) {
	path := req.StringParam("path")
	if err := checkPath(path); err != nil {
		return nil, err
	}
	ref, commit, root, err := e.resolveRef(ctx, r, req.StringParam("ref"))
	if err != nil {
		return nil, err
	}
	w := &treeWalker{client: e.client, repo: repoPath(e.config.Owner, r.Name), cache: map[string]*tree{}}
	entry, err := w.lookup(ctx, root, path)
	if err != nil {
		return nil, err
	}
	switch {
	case entry.Type == submoduleType:
		return nil, fmt.Errorf("%s is a submodule, not a file", path)
	case entry.Type != "blob":
		return nil, fmt.Errorf("%s is a directory; list it with list_files", path)
	case entry.Mode == symlinkMode:
		return nil, fmt.Errorf("%s is a symbolic link; read the file it points at", path)
	}
	out := &fileRead{Repo: r.Name, Ref: ref, Commit: commit, Path: path, SHA: entry.SHA, Size: entry.Size, Untrusted: true}
	// A file far larger than what is returned is not fetched whole just to
	// be cut; GitHub serves blobs up to 100 MB.
	if entry.Size > maxFileBytesCeiling {
		return nil, fmt.Errorf("%s is %d bytes, too large to read", path, entry.Size)
	}
	data, err := w.blob(ctx, entry.SHA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if isBinary(data) {
		out.Binary = true
		return out, nil
	}
	out.Content = string(data)
	if len(out.Content) > maxReadFileBytes {
		out.Content, out.Truncated = capText(out.Content, maxReadFileBytes), true
	}
	// Control characters encode as six bytes each; cut until it fits.
	for {
		data, err := encode(out)
		if err != nil {
			return nil, err
		}
		if len(data) <= readOutputBudget {
			return out, nil
		}
		out.Content, out.Truncated = capText(out.Content, len(out.Content)/2), true
	}
}

type fileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type fileList struct {
	Repo      string      `json:"repo"`
	Ref       string      `json:"ref"`
	Commit    string      `json:"commit"`
	Path      string      `json:"path"`
	Files     []fileEntry `json:"files"`
	Total     int         `json:"total"`
	Truncated bool        `json:"truncated,omitempty"`
	Untrusted bool        `json:"untrusted"`
}

func (e *engine) listFiles(ctx context.Context, r *repository, req pluginsdk.ActionRequest) (*fileList, error) {
	path := req.StringParam("path")
	if path != "" {
		if err := checkPath(path); err != nil {
			return nil, err
		}
	}
	ref, commit, root, err := e.resolveRef(ctx, r, req.StringParam("ref"))
	if err != nil {
		return nil, err
	}
	w := &treeWalker{client: e.client, repo: repoPath(e.config.Owner, r.Name), cache: map[string]*tree{}}
	dir, err := w.lookup(ctx, root, path)
	if err != nil {
		return nil, err
	}
	if dir.Type != "tree" {
		return nil, fmt.Errorf("%s is not a directory; read it with read_file", path)
	}
	// Only the subtree is fetched, so listing one directory of a large
	// repository does not read all of it.
	t, err := w.tree(ctx, dir.SHA, true)
	if err != nil {
		return nil, fmt.Errorf("listing %q: %w", path, err)
	}
	limit, _ := req.IntParam("limit")
	out := &fileList{Repo: r.Name, Ref: ref, Commit: commit, Path: path, Files: []fileEntry{}, Truncated: t.Truncated, Untrusted: true}
	for _, entry := range t.Tree {
		if entry.Type != "blob" {
			continue
		}
		out.Total++
		if int64(len(out.Files)) >= limit {
			out.Truncated = true
			continue
		}
		full := entry.Path
		if path != "" {
			full = path + "/" + entry.Path
		}
		out.Files = append(out.Files, fileEntry{Path: stripControl(full), Size: entry.Size})
	}
	return out, nil
}

type pullRequestSummary struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	State          string     `json:"state"`
	Draft          bool       `json:"draft"`
	Merged         bool       `json:"merged"`
	Author         string     `json:"author"`
	Head           string     `json:"head"`
	HeadRepo       string     `json:"head_repo,omitempty"`
	HeadSHA        string     `json:"head_sha"`
	Base           string     `json:"base"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
	URL            string     `json:"url"`
	OpenedByMarmot bool       `json:"opened_by_marmot"`
}

type pullRequestList struct {
	Repo         string               `json:"repo"`
	State        string               `json:"state"`
	PullRequests []pullRequestSummary `json:"pull_requests"`
	Untrusted    bool                 `json:"untrusted"`
}

func (e *engine) listPullRequests(ctx context.Context, r *repository, req pluginsdk.ActionRequest) (*pullRequestList, error) {
	state := req.StringParam("state")
	limit, _ := req.IntParam("limit")
	prs, err := getPaged[pullRequest](ctx, e.client, repoPath(e.config.Owner, r.Name)+"/pulls",
		url.Values{"state": {state}, "sort": {"updated"}, "direction": {"desc"}}, int(limit))
	if err != nil {
		return nil, fmt.Errorf("listing pull requests of %s: %w", r.FullName, err)
	}
	out := &pullRequestList{Repo: r.Name, State: state, PullRequests: []pullRequestSummary{}, Untrusted: true}
	for i := range prs {
		pr := &prs[i]
		// The API's filter is the server's; this is the plugin's own.
		if pr.State != state {
			continue
		}
		s := pullRequestSummary{
			Number: pr.Number, Title: capText(stripControl(pr.Title), maxTitle), State: pr.State, Draft: pr.Draft,
			Merged: pr.MergedAt != nil, Author: pr.User.Login, Head: pr.Head.Ref, HeadSHA: pr.Head.SHA, Base: pr.Base.Ref,
			CreatedAt: pr.CreatedAt, UpdatedAt: pr.UpdatedAt, ClosedAt: pr.ClosedAt, URL: pr.HTMLURL,
			OpenedByMarmot: e.isOurs(r, pr),
		}
		if pr.Head.Repo != nil {
			s.HeadRepo = pr.Head.Repo.FullName
		}
		out.PullRequests = append(out.PullRequests, s)
	}
	return out, nil
}

// stripControl drops control characters other than tab and newline, such
// as terminal escapes, which could restyle or hide text where it is shown.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
}

// capText cuts s to at most n bytes on a character boundary.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
