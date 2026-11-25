package github

import (
	"crypto/sha1" //nolint:gosec // G505: fake object ids
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

const testToken = "ghp_test-token-never-logged"

// fakeGitHub is an in-memory GitHub with just enough git underneath that a
// pull request really is a tree, a commit and a ref: the plugin's writes
// can be read back the way GitHub would serve them.
type fakeGitHub struct {
	t   *testing.T
	srv *httptest.Server

	mu        sync.Mutex
	owner     string
	ownerType string
	// login is the token's user; empty makes /user fail as it does for an
	// installation token.
	login    string
	repos    map[string]*fakeRepo
	order    []string
	blobs    map[string]string
	trees    map[string]map[string]fakeFile
	commits  map[string]fakeCommit
	comments map[int64]*fakeComment
	nextID   int
	clock    time.Time
	pageSize int
	// requests records every call as "METHOD /path".
	requests   []string
	userAgents []string
	failPulls  bool
	// foreignNext makes list pages link to another host.
	foreignNext bool
	// noGraphQL answers the GraphQL API with 404, as an old GitHub
	// Enterprise Server does; graphqlError answers every query with an
	// error instead.
	noGraphQL    bool
	graphqlError bool
}

type fakeRepo struct {
	meta     map[string]any
	branches map[string]string
	pulls    map[int]*fakePR
	// issues are listed by the issues API as they are given; nil makes it
	// answer 410, as for a repository with issues turned off.
	issues []map[string]any
}

type fakeFile struct {
	mode string
	sha  string
}

type fakeCommit struct {
	tree    string
	parents []string
	message string
}

type fakePR struct {
	number              int
	title, body, state  string
	head, base, headSHA string
	headRepo            string
	user                string
	merged              bool
	created, updated    time.Time
}

type fakeComment struct {
	id      int64
	repo    string
	issue   int
	body    string
	user    string
	bot     bool
	created time.Time
	updated time.Time
}

func newFake(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{
		t: t, owner: "acme", ownerType: "Organization", login: "marmot-bot",
		repos: map[string]*fakeRepo{}, blobs: map[string]string{}, trees: map[string]map[string]fakeFile{},
		commits: map[string]fakeCommit{}, comments: map[int64]*fakeComment{},
		clock: time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC), pageSize: 100,
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGitHub) config(extra map[string]any) pluginsdk.RawConfig {
	c := pluginsdk.RawConfig{"owner": f.owner, "base_url": f.srv.URL, "token": testToken, "include_private": true}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

func (f *fakeGitHub) tick() time.Time {
	f.clock = f.clock.Add(time.Minute)
	return f.clock
}

func gitSHA(parts ...string) string {
	h := sha1.New() //nolint:gosec // fake object ids
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// addRepo creates a repository whose default branch main holds files.
func (f *fakeGitHub) addRepo(name string, meta map[string]any, files map[string]string) *fakeRepo {
	m := map[string]any{
		"name": name, "node_id": "R_" + name, "full_name": f.owner + "/" + name, "owner": map[string]any{"login": f.owner},
		"default_branch": "main", "visibility": "public", "html_url": "https://github.com/" + f.owner + "/" + name,
	}
	for k, v := range meta {
		m[k] = v
	}
	r := &fakeRepo{meta: m, branches: map[string]string{}, pulls: map[int]*fakePR{}, issues: []map[string]any{}}
	f.repos[name] = r
	f.order = append(f.order, name)
	f.commit(name, "main", files, nil)
	return r
}

// commit makes a commit on branch with files changed (an empty mode
// default of 100644), the way a push would.
func (f *fakeGitHub) commit(repo, branch string, files map[string]string, modes map[string]string) string {
	r := f.repos[repo]
	tree := map[string]fakeFile{}
	var parents []string
	if head, ok := r.branches[branch]; ok {
		parents = []string{head}
		for p, file := range f.trees[f.commits[head].tree] {
			tree[p] = file
		}
	}
	for p, content := range files {
		mode := "100644"
		if m, ok := modes[p]; ok {
			mode = m
		}
		sha := blobSHA(content)
		f.blobs[sha] = content
		tree[p] = fakeFile{mode: mode, sha: sha}
	}
	treeID := f.storeTree(tree)
	sha := gitSHA("commit", treeID, strings.Join(parents, ","), branch, strconv.Itoa(len(f.commits)))
	f.commits[sha] = fakeCommit{tree: treeID, parents: parents, message: "push"}
	r.branches[branch] = sha
	return sha
}

func (f *fakeGitHub) storeTree(tree map[string]fakeFile) string {
	f.nextID++
	id := fmt.Sprintf("T%d", f.nextID)
	f.trees[id] = tree
	return id
}

// fileAt returns a file's content at a branch's head.
func (f *fakeGitHub) fileAt(repo, branch, path string) (string, string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	head, ok := f.repos[repo].branches[branch]
	if !ok {
		return "", "", false
	}
	file, ok := f.trees[f.commits[head].tree][path]
	if !ok {
		return "", "", false
	}
	return f.blobs[file.sha], file.mode, true
}

func (f *fakeGitHub) writes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if !strings.HasPrefix(r, "GET ") {
			n++
		}
	}
	return n
}

func (f *fakeGitHub) requested(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.ContainsFunc(f.requests, func(r string) bool { return strings.HasPrefix(r, prefix) })
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}

func (f *fakeGitHub) serve(w http.ResponseWriter, req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req.Method+" "+req.URL.Path)
	f.userAgents = append(f.userAgents, req.UserAgent())
	if req.Header.Get("Authorization") != "Bearer "+testToken {
		fail(w, http.StatusUnauthorized, "Bad credentials")
		return
	}
	if req.Header.Get("X-GitHub-Api-Version") == "" {
		fail(w, http.StatusBadRequest, "no API version")
		return
	}
	var body map[string]any
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&body)
	}
	seg := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	route := req.Method + " " + seg[0]

	switch {
	case route == "POST graphql" && len(seg) == 1:
		f.serveGraphQL(w, body)
	case route == "GET user" && len(seg) == 1:
		if f.login == "" {
			fail(w, http.StatusForbidden, "Resource not accessible by integration")
			return
		}
		writeJSON(w, 200, map[string]any{"login": f.login, "type": "User"})
	case route == "GET users" && len(seg) == 2:
		if seg[1] != f.owner {
			fail(w, 404, "Not Found")
			return
		}
		writeJSON(w, 200, map[string]any{"login": f.owner, "type": f.ownerType})
	case route == "GET orgs" && len(seg) == 3, route == "GET users" && len(seg) == 3, route == "GET user" && len(seg) == 2:
		f.listRepos(w, req, seg)
	case seg[0] == "repos" && len(seg) >= 3:
		if seg[1] != f.owner {
			fail(w, 404, "Not Found")
			return
		}
		r, ok := f.repos[seg[2]]
		if !ok {
			fail(w, 404, "Not Found")
			return
		}
		f.serveRepo(w, req, seg[2], r, seg[3:], body)
	default:
		fail(w, 404, "Not Found")
	}
}

func (f *fakeGitHub) listRepos(w http.ResponseWriter, req *http.Request, seg []string) {
	var all []map[string]any
	for _, name := range f.order {
		r := f.repos[name]
		// Only /user/repos and an organisation list private repositories,
		// as on GitHub.
		if seg[0] == "users" && r.meta["private"] == true {
			continue
		}
		all = append(all, r.meta)
	}
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	page = max(page, 1)
	start, end := (page-1)*f.pageSize, page*f.pageSize
	if start > len(all) {
		start = len(all)
	}
	if end < len(all) {
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page+1))
		host := "http://" + req.Host
		if f.foreignNext {
			host = "http://evil.example"
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, host, req.URL.Path, q.Encode()))
	} else {
		end = len(all)
	}
	writeJSON(w, 200, all[start:end])
}

func (f *fakeGitHub) serveRepo(w http.ResponseWriter, req *http.Request, name string, r *fakeRepo, rest []string, body map[string]any) {
	route := req.Method
	if len(rest) > 0 {
		route += " " + rest[0]
	}
	if len(rest) > 1 && (rest[0] == "git" || rest[0] == "issues") {
		route += " " + rest[1]
	}
	switch {
	case route == "GET":
		writeJSON(w, 200, r.meta)
	case route == "GET readme":
		f.serveContents(w, r, "README.md")
	case route == "GET contents":
		f.serveContents(w, r, strings.Join(rest[1:], "/"))
	case route == "GET commits":
		ref := strings.Join(rest[1:], "/")
		sha, ok := r.branches[ref]
		if !ok {
			if _, isCommit := f.commits[ref]; !isCommit {
				fail(w, 422, "No commit found for SHA: "+ref)
				return
			}
			sha = ref
		}
		writeJSON(w, 200, map[string]any{"sha": sha, "commit": map[string]any{"tree": map[string]any{"sha": f.commits[sha].tree}}})
	case route == "GET git ref":
		branch := strings.Join(rest[3:], "/")
		sha, ok := r.branches[branch]
		if rest[2] != "heads" || !ok {
			fail(w, 404, "Not Found")
			return
		}
		writeJSON(w, 200, map[string]any{"ref": "refs/heads/" + branch, "object": map[string]any{"sha": sha}})
	case route == "GET git commits":
		c, ok := f.commits[rest[2]]
		if !ok {
			fail(w, 404, "Not Found")
			return
		}
		writeJSON(w, 200, map[string]any{"sha": rest[2], "tree": map[string]any{"sha": c.tree}, "message": c.message})
	case route == "GET git trees":
		f.serveTree(w, rest[2], req.URL.Query().Get("recursive") == "1")
	case route == "GET git blobs":
		content, ok := f.blobs[rest[2]]
		if !ok {
			fail(w, 404, "Not Found")
			return
		}
		writeJSON(w, 200, map[string]any{"sha": rest[2], "size": len(content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
	case route == "POST git trees":
		base := f.trees[body["base_tree"].(string)]
		tree := map[string]fakeFile{}
		for p, file := range base {
			tree[p] = file
		}
		for _, e := range body["tree"].([]any) {
			entry := e.(map[string]any)
			content := entry["content"].(string)
			sha := blobSHA(content)
			f.blobs[sha] = content
			tree[entry["path"].(string)] = fakeFile{mode: entry["mode"].(string), sha: sha}
		}
		writeJSON(w, 201, map[string]any{"sha": f.storeTree(tree)})
	case route == "POST git commits":
		var parents []string
		for _, p := range body["parents"].([]any) {
			parents = append(parents, p.(string))
		}
		sha := gitSHA("commit", body["tree"].(string), strings.Join(parents, ","), body["message"].(string))
		f.commits[sha] = fakeCommit{tree: body["tree"].(string), parents: parents, message: body["message"].(string)}
		writeJSON(w, 201, map[string]any{"sha": sha, "tree": map[string]any{"sha": body["tree"]}})
	case route == "POST git refs":
		branch := strings.TrimPrefix(body["ref"].(string), "refs/heads/")
		if _, exists := r.branches[branch]; exists {
			fail(w, 422, "Reference already exists")
			return
		}
		r.branches[branch] = body["sha"].(string)
		writeJSON(w, 201, map[string]any{"ref": body["ref"]})
	case route == "DELETE git refs":
		delete(r.branches, strings.Join(rest[3:], "/"))
		w.WriteHeader(204)
	case route == "GET branches" && len(rest) == 1:
		names := make([]string, 0, len(r.branches))
		for b := range r.branches {
			names = append(names, b)
		}
		slices.Sort(names)
		var out []map[string]any
		for _, b := range names {
			out = append(out, map[string]any{"name": b, "protected": b == "main", "commit": map[string]any{"sha": r.branches[b]}})
		}
		writeJSON(w, 200, out)
	case route == "GET issues" && len(rest) == 1:
		if r.issues == nil {
			fail(w, 410, "Issues are disabled for this repo")
			return
		}
		writeJSON(w, 200, r.issues)
	case route == "GET pulls" && len(rest) == 1:
		var out []map[string]any
		for n := len(r.pulls); n >= 1; n-- {
			if pr := r.pulls[n]; req.URL.Query().Get("state") == "all" || pr.state == req.URL.Query().Get("state") {
				out = append(out, f.prJSON(name, pr))
			}
		}
		writeJSON(w, 200, out)
	case route == "POST pulls":
		if f.failPulls {
			fail(w, 422, "Validation Failed")
			return
		}
		head := body["head"].(string)
		sha, ok := r.branches[head]
		if !ok {
			fail(w, 422, "head does not exist")
			return
		}
		now := f.tick()
		pr := &fakePR{number: len(r.pulls) + 1, title: body["title"].(string), body: body["body"].(string), state: "open",
			head: head, base: body["base"].(string), headSHA: sha, headRepo: f.owner + "/" + name, user: f.login, created: now, updated: now}
		r.pulls[pr.number] = pr
		writeJSON(w, 201, f.prJSON(name, pr))
	case (route == "GET pulls" || route == "PATCH pulls") && len(rest) == 2:
		n, _ := strconv.Atoi(rest[1])
		pr, ok := r.pulls[n]
		if !ok {
			fail(w, 404, "Not Found")
			return
		}
		if req.Method == "PATCH" {
			pr.state = body["state"].(string)
			pr.updated = f.tick()
		}
		writeJSON(w, 200, f.prJSON(name, pr))
	case strings.HasPrefix(route, "POST issues ") && len(rest) == 3 && rest[2] == "comments":
		n, _ := strconv.Atoi(rest[1])
		f.nextID++
		now := f.tick()
		c := &fakeComment{id: int64(1000 + f.nextID), repo: name, issue: n, body: body["body"].(string), user: f.login, created: now, updated: now}
		if f.login == "" {
			c.user, c.bot = "marmot-app[bot]", true
		}
		f.comments[c.id] = c
		if pr, ok := r.pulls[n]; ok {
			pr.updated = now
		}
		writeJSON(w, 201, f.commentJSON(req, c))
	case route == "GET issues comments", route == "DELETE issues comments":
		id, _ := strconv.ParseInt(rest[2], 10, 64)
		c, ok := f.comments[id]
		if !ok || c.repo != name {
			fail(w, 404, "Not Found")
			return
		}
		if req.Method == "DELETE" {
			delete(f.comments, id)
			w.WriteHeader(204)
			return
		}
		writeJSON(w, 200, f.commentJSON(req, c))
	default:
		fail(w, 404, "Not Found: "+route)
	}
}

func (f *fakeGitHub) serveContents(w http.ResponseWriter, r *fakeRepo, path string) {
	file, ok := f.trees[f.commits[r.branches["main"]].tree][path]
	if !ok {
		fail(w, 404, "Not Found")
		return
	}
	writeJSON(w, 200, map[string]any{"type": "file", "path": path, "encoding": "base64",
		"content": base64.StdEncoding.EncodeToString([]byte(f.blobs[file.sha]))})
}

// serveTree lists a tree. A root tree's id is T<n>; a directory under it
// is T<n>|dir|sub, so directories need no storage of their own.
func (f *fakeGitHub) serveTree(w http.ResponseWriter, id string, recursive bool) {
	parts := strings.Split(id, "|")
	flat, ok := f.trees[parts[0]]
	if !ok {
		fail(w, 404, "Not Found")
		return
	}
	dir := strings.Join(parts[1:], "/")
	var entries []map[string]any
	seen := map[string]bool{}
	paths := make([]string, 0, len(flat))
	for p := range flat {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		rel := p
		if dir != "" {
			var found bool
			if rel, found = strings.CutPrefix(p, dir+"/"); !found {
				continue
			}
		}
		file := flat[p]
		segs := strings.Split(rel, "/")
		for i := 1; i < len(segs); i++ {
			sub := strings.Join(segs[:i], "/")
			if seen[sub] || (!recursive && i > 1) {
				continue
			}
			seen[sub] = true
			entries = append(entries, map[string]any{"path": sub, "mode": "040000", "type": "tree", "sha": id + "|" + strings.ReplaceAll(sub, "/", "|")})
		}
		if !recursive && len(segs) > 1 {
			continue
		}
		typ := "blob"
		if file.mode == "160000" {
			typ = "commit"
		}
		entries = append(entries, map[string]any{"path": rel, "mode": file.mode, "type": typ, "sha": file.sha, "size": len(f.blobs[file.sha])})
	}
	writeJSON(w, 200, map[string]any{"sha": id, "tree": entries, "truncated": false})
}

func (f *fakeGitHub) prJSON(repo string, pr *fakePR) map[string]any {
	out := map[string]any{
		"number": pr.number, "title": pr.title, "body": pr.body, "state": pr.state,
		"html_url": fmt.Sprintf("https://github.com/%s/%s/pull/%d", f.owner, repo, pr.number),
		"user":     map[string]any{"login": pr.user}, "created_at": pr.created, "updated_at": pr.updated,
		"head": map[string]any{"ref": pr.head, "sha": pr.headSHA, "repo": map[string]any{"full_name": pr.headRepo}},
		"base": map[string]any{"ref": pr.base},
	}
	if pr.merged {
		out["merged_at"] = pr.updated
	}
	return out
}

func (f *fakeGitHub) commentJSON(req *http.Request, c *fakeComment) map[string]any {
	out := map[string]any{
		"id": c.id, "body": c.body, "created_at": c.created, "updated_at": c.updated,
		"user":      map[string]any{"login": c.user, "type": "User"},
		"issue_url": fmt.Sprintf("http://%s/repos/%s/%s/issues/%d", req.Host, f.owner, c.repo, c.issue),
		"html_url":  fmt.Sprintf("https://github.com/%s/%s/pull/%d#issuecomment-%d", f.owner, c.repo, c.issue, c.id),
	}
	if c.bot {
		out["user"] = map[string]any{"login": c.user, "type": "Bot"}
		out["performed_via_github_app"] = map[string]any{"slug": "marmot-app"}
	}
	return out
}
