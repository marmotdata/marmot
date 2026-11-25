package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultBaseURL = "https://api.github.com"
	apiVersion     = "2022-11-28"
	pageSize       = 100
	// maxResponseBytes bounds any one response body, so a huge tree or a
	// misbehaving proxy cannot exhaust the plugin's memory.
	maxResponseBytes = 32 << 20
	// maxErrorMessage caps the message quoted from a GitHub error.
	maxErrorMessage = 300
	requestTimeout  = 30 * time.Second
	// maxRetries and maxBackoff bound how long a read waits out a rate
	// limit before giving up: a primary limit that resets in an hour is
	// reported, not slept through.
	maxRetries = 3
	maxBackoff = 60 * time.Second
)

// apiError is a GitHub response outside 2xx. Its message is GitHub's own,
// never the request, so the token cannot end up in it.
type apiError struct {
	Status  int
	Method  string
	Path    string
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("GitHub %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
}

func statusOf(err error) int {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.Status
	}
	return 0
}

func isNotFound(err error) bool { return statusOf(err) == http.StatusNotFound }

// client is a small typed wrapper over the REST API: the plugin needs a
// dozen endpoints, and go-github would bring a large dependency for them.
type client struct {
	base      *url.URL
	token     string
	userAgent string
	http      *http.Client
}

func newClient(baseURL, token, userAgent string) (*client, error) {
	if token == "" {
		return nil, errors.New("no GitHub token: set token in the config or GITHUB_TOKEN in the environment")
	}
	base, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &client{
		base:      base,
		token:     token,
		userAgent: userAgent,
		http: &http.Client{
			Timeout: requestTimeout,
			// A redirect to another host would carry the request on
			// without its token (Go strips it), and nothing the plugin
			// calls should redirect anyway, so it is refused outright.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// parseBaseURL accepts https anywhere and plain http only on a loopback
// address, so the token never crosses a network in the clear.
func parseBaseURL(raw string) (*url.URL, error) {
	if raw == "" {
		raw = defaultBaseURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("base_url %q is not a URL", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, fmt.Errorf("base_url %q must not carry a query, fragment or credentials", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("base_url %q must use https", raw)
		}
	default:
		return nil, fmt.Errorf("base_url %q must use https", raw)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

// escapePath escapes each segment of a slash-separated path on its own, so
// slashes in branch names and file paths stay path separators as GitHub
// expects. Callers have already refused "." and ".." segments.
func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

func (c *client) url(path string, query url.Values) string {
	u := c.base.Scheme + "://" + c.base.Host + c.base.Path + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// do sends one request and decodes a JSON response into out, when given.
func (c *client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	_, err := c.doURL(ctx, method, c.url(path, query), path, body, out)
	return err
}

// doURL sends one request. A read, or a GraphQL query, that GitHub turns
// away for a rate limit is retried after the wait GitHub asks for; a write
// never is, so it can never be sent twice.
func (c *client) doURL(ctx context.Context, method, target, label string, body, out any) (http.Header, error) {
	var data []byte
	if body != nil {
		var err error
		if data, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("encoding the request to %s: %w", label, err)
		}
	}
	retryable := method == http.MethodGet || label == "/graphql"
	for attempt := 0; ; attempt++ {
		header, wait, err := c.send(ctx, method, target, label, data, out)
		if wait <= 0 || !retryable || attempt == maxRetries {
			return header, err
		}
		log.Warn().Str("path", label).Dur("wait", wait).Msg("GitHub rate limit, waiting before retrying")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// rateLimitWait is how long GitHub asks a turned-away request to wait, or
// zero when the response is not a rate limit or the wait is too long.
func rateLimitWait(resp *http.Response) time.Duration {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return 0
	}
	var wait time.Duration
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s >= 0 {
		wait = time.Duration(s) * time.Second
	} else if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
		if err != nil {
			return 0
		}
		wait = time.Until(time.Unix(reset, 0))
	} else if resp.StatusCode == http.StatusTooManyRequests {
		wait = time.Minute
	} else {
		return 0
	}
	if wait > maxBackoff {
		return 0
	}
	return max(wait, time.Second)
}

func (c *client) send(ctx context.Context, method, target, label string, data []byte, out any) (http.Header, time.Duration, error) {
	var reader io.Reader
	if data != nil {
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("building the request to %s: %w", label, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// net/http errors quote the URL, which holds no credentials.
		return nil, 0, fmt.Errorf("GitHub %s %s: %w", method, label, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		var msg struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &msg)
		if msg.Message == "" {
			msg.Message = http.StatusText(resp.StatusCode)
		}
		return nil, rateLimitWait(resp), &apiError{Status: resp.StatusCode, Method: method, Path: label, Message: capText(stripControl(msg.Message), maxErrorMessage)}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return resp.Header, 0, nil
	}
	respData, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("reading GitHub %s %s: %w", method, label, err)
	}
	if len(respData) > maxResponseBytes {
		return nil, 0, fmt.Errorf("GitHub %s %s: the response is over %d MiB", method, label, maxResponseBytes>>20)
	}
	if err := json.Unmarshal(respData, out); err != nil {
		return nil, 0, fmt.Errorf("decoding GitHub %s %s: %w", method, label, err)
	}
	return resp.Header, 0, nil
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// getPaged follows Link headers until limit items are read or the pages
// run out. A next link is followed only on the configured API host, so a
// forged header cannot send the token elsewhere.
func getPaged[T any](ctx context.Context, c *client, path string, query url.Values, limit int) ([]T, error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("per_page", fmt.Sprint(min(limit, pageSize)))
	target := c.url(path, query)
	var out []T
	for target != "" && len(out) < limit {
		var page []T
		header, err := c.doURL(ctx, http.MethodGet, target, path, nil, &page)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		target = ""
		if m := nextLink.FindStringSubmatch(header.Get("Link")); m != nil {
			next, err := url.Parse(m[1])
			if err == nil && next.Scheme == c.base.Scheme && next.Host == c.base.Host && strings.HasPrefix(next.Path, c.base.Path+"/") {
				target = next.String()
			}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// API types: only the fields the plugin reads.

type account struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type repository struct {
	NodeID          string     `json:"node_id"`
	Name            string     `json:"name"`
	FullName        string     `json:"full_name"`
	Owner           account    `json:"owner"`
	Description     string     `json:"description"`
	DefaultBranch   string     `json:"default_branch"`
	Language        string     `json:"language"`
	Visibility      string     `json:"visibility"`
	Private         bool       `json:"private"`
	Archived        bool       `json:"archived"`
	Disabled        bool       `json:"disabled"`
	Fork            bool       `json:"fork"`
	Topics          []string   `json:"topics"`
	HTMLURL         string     `json:"html_url"`
	StargazersCount int        `json:"stargazers_count"`
	ForksCount      int        `json:"forks_count"`
	OpenIssuesCount int        `json:"open_issues_count"`
	CreatedAt       *time.Time `json:"created_at"`
	PushedAt        *time.Time `json:"pushed_at"`
	License         *struct {
		SPDXID string `json:"spdx_id"`
	} `json:"license"`
}

// isPublic is false for private and internal repositories alike: an
// internal one is visible to the whole enterprise, not the world.
func (r *repository) isPublic() bool {
	return !r.Private && (r.Visibility == "" || r.Visibility == "public")
}

type contentFile struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

type gitRef struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type gitCommit struct {
	SHA  string `json:"sha"`
	Tree struct {
		SHA string `json:"sha"`
	} `json:"tree"`
}

// commitInfo is the REST commits endpoint's shape, which resolves any ref.
type commitInfo struct {
	SHA    string `json:"sha"`
	Commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	} `json:"commit"`
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type tree struct {
	SHA       string      `json:"sha"`
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

type blob struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	Size     int64  `json:"size"`
}

type pullRequest struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	Draft     bool       `json:"draft"`
	Locked    bool       `json:"locked"`
	HTMLURL   string     `json:"html_url"`
	User      account    `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	MergedAt  *time.Time `json:"merged_at"`
	Head      prBranch   `json:"head"`
	Base      prBranch   `json:"base"`
}

type prBranch struct {
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Repo *struct {
		FullName string `json:"full_name"`
	} `json:"repo"`
}

type issueComment struct {
	ID                    int64     `json:"id"`
	Body                  string    `json:"body"`
	User                  account   `json:"user"`
	IssueURL              string    `json:"issue_url"`
	HTMLURL               string    `json:"html_url"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	PerformedViaGitHubApp *struct {
		Slug string `json:"slug"`
	} `json:"performed_via_github_app"`
}

func repoPath(owner, repo string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}
