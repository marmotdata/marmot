package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Discovery reads what it needs about each repository through the GraphQL
// API, a batch of repositories per request, instead of four or more REST
// requests per repository. Anything a batch could not answer falls back to
// REST for that repository alone.
const (
	// graphqlBatch is how many repositories one query asks about. Larger
	// batches run into GitHub's ten second limit on a single query.
	graphqlBatch = 10
	// graphqlWorkers is how many queries are in flight at once, far below
	// GitHub's limit of 100 concurrent requests and 2,000 GraphQL points a
	// minute, and backed off whenever GitHub asks.
	graphqlWorkers = 4
)

// graphqlFirst is the page size of each list in a query. A repository with
// more branches, pull requests or issues than this has that list read
// through REST instead. A variable so tests can make lists overflow.
var graphqlFirst = 100

// readmeCandidates are where the README is looked for, in the order GitHub
// itself picks one: the .github directory, the root, then docs.
var readmeCandidates = []string{
	".github/README.md",
	"README.md", "README", "README.rst", "README.txt", "README.markdown", "README.adoc", "readme.md", "Readme.md",
	"docs/README.md",
}

// repoDetails is what one query learned about a repository. A list is used
// only when it is complete; otherwise discovery reads it through REST.
type repoDetails struct {
	readme, codeowners       string
	hasReadme, hasCodeowners bool
	branches                 []branch
	pulls                    []openPullRequest
	issues                   []issue
	branchesDone, pullsDone  bool
	issuesDone               bool
}

type gqlBlob struct {
	Text     *string `json:"text"`
	IsBinary bool    `json:"isBinary"`
}

type gqlLogins struct {
	Nodes []struct {
		Login string `json:"login"`
	} `json:"nodes"`
}

type gqlLabels struct {
	Nodes []label `json:"nodes"`
}

type gqlPage struct {
	HasNextPage bool `json:"hasNextPage"`
}

type gqlRepository struct {
	Name string `json:"name"`
	Refs *struct {
		PageInfo gqlPage `json:"pageInfo"`
		Nodes    []struct {
			Name   string `json:"name"`
			Target struct {
				OID string `json:"oid"`
			} `json:"target"`
		} `json:"nodes"`
	} `json:"refs"`
	PullRequests *struct {
		PageInfo gqlPage `json:"pageInfo"`
		Nodes    []struct {
			Number         int       `json:"number"`
			Title          string    `json:"title"`
			Body           string    `json:"body"`
			URL            string    `json:"url"`
			IsDraft        bool      `json:"isDraft"`
			CreatedAt      time.Time `json:"createdAt"`
			UpdatedAt      time.Time `json:"updatedAt"`
			Author         *account  `json:"author"`
			HeadRefName    string    `json:"headRefName"`
			BaseRefName    string    `json:"baseRefName"`
			Labels         gqlLabels `json:"labels"`
			Assignees      gqlLogins `json:"assignees"`
			ReviewRequests struct {
				Nodes []struct {
					RequestedReviewer *account `json:"requestedReviewer"`
				} `json:"nodes"`
			} `json:"reviewRequests"`
		} `json:"nodes"`
	} `json:"pullRequests"`
	Issues *struct {
		PageInfo gqlPage `json:"pageInfo"`
		Nodes    []struct {
			Number    int       `json:"number"`
			Title     string    `json:"title"`
			Body      string    `json:"body"`
			URL       string    `json:"url"`
			CreatedAt time.Time `json:"createdAt"`
			UpdatedAt time.Time `json:"updatedAt"`
			Author    *account  `json:"author"`
			Labels    gqlLabels `json:"labels"`
			Assignees gqlLogins `json:"assignees"`
			Comments  struct {
				TotalCount int `json:"totalCount"`
			} `json:"comments"`
		} `json:"nodes"`
	} `json:"issues"`
	// Blobs holds the README and CODEOWNERS candidates, keyed by alias.
	Blobs map[string]*gqlBlob `json:"-"`
}

// graphqlURL is where the GraphQL API lives: beside /api/v3 on GitHub
// Enterprise Server, and at /graphql on api.github.com.
func (c *client) graphqlURL() string {
	path := c.base.Path
	if strings.HasSuffix(path, "/api/v3") {
		path = strings.TrimSuffix(path, "/v3") + "/graphql"
	} else {
		path += "/graphql"
	}
	return c.base.Scheme + "://" + c.base.Host + path
}

// graphql runs one query. Any error GitHub reports fails the whole query,
// so a partial answer is never taken for a complete one.
func (c *client) graphql(ctx context.Context, query string, variables map[string]any, out any) error {
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if _, err := c.doURL(ctx, http.MethodPost, c.graphqlURL(), "/graphql", map[string]any{"query": query, "variables": variables}, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return &apiError{Status: http.StatusOK, Method: http.MethodPost, Path: "/graphql", Message: capText(stripControl(resp.Errors[0].Message), maxErrorMessage)}
	}
	return json.Unmarshal(resp.Data, out)
}

// detailsQuery asks for everything discovery reads about a repository.
func detailsQuery(config *Config) string {
	var b strings.Builder
	b.WriteString("query($ids:[ID!]!){nodes(ids:$ids){... on Repository{id name")
	if config.IncludeReadme {
		for i, p := range readmeCandidates {
			fmt.Fprintf(&b, ` readme%d:object(expression:%q){... on Blob{text isBinary}}`, i, "HEAD:"+p)
		}
	}
	if config.IncludeOwners {
		for i, p := range codeownersPaths {
			fmt.Fprintf(&b, ` codeowners%d:object(expression:%q){... on Blob{text isBinary}}`, i, "HEAD:"+p)
		}
	}
	n := graphqlFirst
	if config.IncludeBranches {
		fmt.Fprintf(&b, ` refs(refPrefix:"refs/heads/",first:%d){pageInfo{hasNextPage} nodes{name target{oid}}}`, n)
	}
	if config.IncludePullRequests {
		fmt.Fprintf(&b, ` pullRequests(states:OPEN,first:%d,orderBy:{field:UPDATED_AT,direction:DESC}){pageInfo{hasNextPage} nodes{number title body url isDraft createdAt updatedAt author{login} headRefName baseRefName labels(first:20){nodes{name}} assignees(first:20){nodes{login}} reviewRequests(first:20){nodes{requestedReviewer{... on User{login}}}}}}`, n)
	}
	if config.IncludeIssues {
		fmt.Fprintf(&b, ` issues(states:OPEN,first:%d,orderBy:{field:UPDATED_AT,direction:DESC}){pageInfo{hasNextPage} nodes{number title body url createdAt updatedAt author{login} labels(first:20){nodes{name}} assignees(first:20){nodes{login}} comments{totalCount}}}`, n)
	}
	b.WriteString("}}}")
	return b.String()
}

// fetchDetails reads the repositories in batches, a few batches at a time.
// A repository missing from the result is read through REST.
func fetchDetails(ctx context.Context, c *client, config *Config, repos []*repository) map[string]*repoDetails {
	out := map[string]*repoDetails{}
	if !config.IncludeReadme && !config.IncludeOwners && !config.IncludeBranches && !config.IncludePullRequests && !config.IncludeIssues {
		return out
	}
	var batches [][]*repository
	for start := 0; start < len(repos); start += graphqlBatch {
		batches = append(batches, repos[start:min(start+graphqlBatch, len(repos))])
	}
	query := detailsQuery(config)

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		work     = make(chan []*repository)
		disabled bool
	)
	for range min(graphqlWorkers, len(batches)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range work {
				mu.Lock()
				skip := disabled
				mu.Unlock()
				if skip || ctx.Err() != nil {
					continue
				}
				got, err := fetchBatch(ctx, c, config, query, batch)
				mu.Lock()
				if err != nil {
					// An API without GraphQL, such as an old GitHub
					// Enterprise Server, answers 404: stop asking.
					if isNotFound(err) {
						disabled = true
					}
					log.Warn().Err(err).Int("repositories", len(batch)).Msg("GraphQL query failed, reading these repositories through REST")
				}
				for name, d := range got {
					out[name] = d
				}
				mu.Unlock()
			}
		}()
	}
	for _, b := range batches {
		work <- b
	}
	close(work)
	wg.Wait()
	return out
}

func fetchBatch(ctx context.Context, c *client, config *Config, query string, batch []*repository) (map[string]*repoDetails, error) {
	ids := make([]string, 0, len(batch))
	for _, r := range batch {
		if r.NodeID != "" {
			ids = append(ids, r.NodeID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var data struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	if err := c.graphql(ctx, query, map[string]any{"ids": ids}, &data); err != nil {
		return nil, err
	}
	out := map[string]*repoDetails{}
	for _, node := range data.Nodes {
		if len(node) == 0 || bytes.Equal(node, []byte("null")) {
			continue // a repository the token cannot see
		}
		var r gqlRepository
		if err := json.Unmarshal(node, &r); err != nil {
			return nil, fmt.Errorf("decoding a GraphQL repository: %w", err)
		}
		// README and CODEOWNERS candidates come back under numbered aliases.
		var r2 struct{ All map[string]json.RawMessage }
		if err := json.Unmarshal(node, &r2.All); err != nil {
			return nil, fmt.Errorf("decoding a GraphQL repository: %w", err)
		}
		r.Blobs = map[string]*gqlBlob{}
		for key, raw := range r2.All {
			if strings.HasPrefix(key, "readme") || strings.HasPrefix(key, "codeowners") {
				var blob *gqlBlob
				if err := json.Unmarshal(raw, &blob); err != nil {
					return nil, fmt.Errorf("decoding a GraphQL blob: %w", err)
				}
				r.Blobs[key] = blob
			}
		}
		out[r.Name] = r.details(config)
	}
	return out, nil
}

// firstText is the first candidate that exists as a text file.
func (r *gqlRepository) firstText(prefix string, n int) (string, bool) {
	for i := range n {
		if b := r.Blobs[fmt.Sprintf("%s%d", prefix, i)]; b != nil && b.Text != nil && !b.IsBinary {
			return *b.Text, true
		}
	}
	return "", false
}

func (r *gqlRepository) details(config *Config) *repoDetails {
	d := &repoDetails{}
	if config.IncludeReadme {
		text, ok := r.firstText("readme", len(readmeCandidates))
		d.readme, d.hasReadme = capText(text, maxReadmeBytes), ok
	}
	if config.IncludeOwners {
		text, ok := r.firstText("codeowners", len(codeownersPaths))
		d.codeowners, d.hasCodeowners = capText(text, maxCodeownersBytes), ok
	}
	if r.Refs != nil {
		d.branchesDone = !r.Refs.PageInfo.HasNextPage
		for _, n := range r.Refs.Nodes {
			var b branch
			b.Name, b.Commit.SHA = n.Name, n.Target.OID
			d.branches = append(d.branches, b)
		}
	}
	if r.PullRequests != nil {
		d.pullsDone = !r.PullRequests.PageInfo.HasNextPage
		for _, n := range r.PullRequests.Nodes {
			var pr openPullRequest
			pr.Number, pr.Title, pr.Body, pr.HTMLURL, pr.Draft = n.Number, n.Title, n.Body, n.URL, n.IsDraft
			pr.State, pr.CreatedAt, pr.UpdatedAt = "open", n.CreatedAt, n.UpdatedAt
			pr.Head.Ref, pr.Base.Ref = n.HeadRefName, n.BaseRefName
			if n.Author != nil {
				pr.User = *n.Author
			}
			pr.Labels = n.Labels.Nodes
			pr.Assignees = logins(n.Assignees)
			for _, rr := range n.ReviewRequests.Nodes {
				if rr.RequestedReviewer != nil && rr.RequestedReviewer.Login != "" {
					pr.RequestedReviewers = append(pr.RequestedReviewers, *rr.RequestedReviewer)
				}
			}
			d.pulls = append(d.pulls, pr)
		}
	}
	if r.Issues != nil {
		d.issuesDone = !r.Issues.PageInfo.HasNextPage
		for _, n := range r.Issues.Nodes {
			is := issue{Number: n.Number, Title: n.Title, Body: n.Body, HTMLURL: n.URL, Labels: n.Labels.Nodes,
				Assignees: logins(n.Assignees), Comments: n.Comments.TotalCount, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}
			if n.Author != nil {
				is.User = *n.Author
			}
			d.issues = append(d.issues, is)
		}
	}
	return d
}

func logins(l gqlLogins) []account {
	var out []account
	for _, n := range l.Nodes {
		out = append(out, account{Login: n.Login})
	}
	return out
}
