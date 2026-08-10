package openmetadata

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// client talks to the OpenMetadata REST API. OpenMetadata authenticates
// machine clients with a JWT: a bot token or a user's personal access
// token, sent as a bearer token.
type client struct {
	baseURL string
	token   string
	http    *http.Client

	// fieldCache remembers the fields each endpoint accepts. Older
	// OpenMetadata releases 400 on fields newer ones added (`domains`,
	// `dataProducts`), so the plugin probes once per endpoint.
	fieldCache map[string]string
	fieldMu    sync.Mutex
}

func newClient(host, token string, timeout time.Duration, insecure bool) *client {
	transport := http.DefaultTransport
	if insecure {
		// Clone rather than replace, so proxy settings and connection
		// pooling behave the same with verification turned off.
		insecureTransport := http.DefaultTransport.(*http.Transport).Clone()
		insecureTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		transport = insecureTransport
	}

	return &client{
		baseURL: apiBaseURL(host),
		token:   token,
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		fieldCache: make(map[string]string),
	}
}

// apiBaseURL normalises whatever the user put in the host field to the
// API root. All three of these are accepted, because all three are what
// people copy out of a browser or an existing ingestion config:
// https://om.example.com, https://om.example.com/, https://om.example.com/api
func apiBaseURL(host string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(host), "/")
	if strings.HasSuffix(trimmed, "/api") {
		return trimmed
	}
	return trimmed + "/api"
}

// get performs a GET against a path below the API root and decodes the
// JSON response into out.
func (c *client) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &apiError{Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// apiError is a non-200 response from OpenMetadata. It is a distinct
// type so callers can tell an entity kind the server does not know
// about (404) from a credential or permission problem.
type apiError struct {
	Path   string
	Status int
	Body   string
}

func (e *apiError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: %s", e.Path, http.StatusText(e.Status))
	}
	return fmt.Sprintf("%s: %s: %s", e.Path, http.StatusText(e.Status), e.Body)
}

// unsupported reports whether the server does not have this endpoint.
// Some older OpenMetadata releases wrap a missing endpoint in a 500
// with "HTTP 404 Not Found" in the body instead of returning a clean
// 404. A plain 400 is not unsupported: it means the request itself was
// wrong, and treating it as absent would silently drop entity kinds.
func (e *apiError) unsupported() bool {
	if e.Status == http.StatusNotFound {
		return true
	}
	if e.Status == http.StatusInternalServerError && strings.Contains(e.Body, "HTTP 404 Not Found") {
		return true
	}
	return false
}

// listAll pages through an OpenMetadata list endpoint and returns every
// entity. OpenMetadata pages with an opaque `after` cursor rather than
// an offset, and only returns the fields asked for in `fields`.
func listAll[T any](ctx context.Context, c *client, path, fields string, pageSize int, includeDeleted bool) ([]T, error) {
	accepted, err := c.resolveFields(ctx, path, fields, includeDeleted)
	if err != nil {
		return nil, err
	}

	var all []T
	after := ""

	for {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(pageSize))
		if accepted != "" {
			query.Set("fields", accepted)
		}
		if includeDeleted {
			query.Set("include", "all")
		}
		if after != "" {
			query.Set("after", after)
		}

		var page listResponse[T]
		if err := c.get(ctx, path, query, &page); err != nil {
			return all, err
		}

		all = append(all, page.Data...)

		// A cursor that does not move would page forever.
		if page.Paging.After == "" || page.Paging.After == after {
			return all, nil
		}
		after = page.Paging.After
	}
}

// unknownFieldRe matches e.g. `{"message":"Invalid field name domains"}`.
var unknownFieldRe = regexp.MustCompile(`Invalid field name (\w+)`)

// resolveFields returns the subset of wanted that path accepts,
// probing with limit=1 and dropping any field OpenMetadata names in a
// 400. The result is cached per (path, wanted). Errors other than an
// unknown-field 400 are surfaced so the caller sees the real problem.
func (c *client) resolveFields(ctx context.Context, path, wanted string, includeDeleted bool) (string, error) {
	if wanted == "" {
		return "", nil
	}

	key := path + "|" + wanted
	c.fieldMu.Lock()
	if cached, ok := c.fieldCache[key]; ok {
		c.fieldMu.Unlock()
		return cached, nil
	}
	c.fieldMu.Unlock()

	current := wanted
	for i := 0; i < 10; i++ {
		query := url.Values{}
		query.Set("limit", "1")
		if current != "" {
			query.Set("fields", current)
		}
		if includeDeleted {
			query.Set("include", "all")
		}

		var probe listResponse[json.RawMessage]
		err := c.get(ctx, path, query, &probe)
		if err == nil {
			break
		}

		var apiErr *apiError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
			return "", err
		}

		m := unknownFieldRe.FindStringSubmatch(apiErr.Body)
		if len(m) < 2 {
			return "", err
		}

		next := dropField(current, m[1])
		if next == current {
			// Server named a field we did not ask for; nothing to strip.
			return "", err
		}

		log.Debug().Str("path", path).Str("field", m[1]).Msg("OpenMetadata rejected field, dropping and retrying")
		current = next
	}

	c.fieldMu.Lock()
	c.fieldCache[key] = current
	c.fieldMu.Unlock()
	return current, nil
}

func dropField(list, name string) string {
	parts := strings.Split(list, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == name {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

// pipelineRuns returns a pipeline's recent executions, newest first.
// Unlike the other endpoints this one addresses the pipeline by fully
// qualified name; passing an id returns an empty list rather than an
// error, so it must be the name.
func (c *client) pipelineRuns(ctx context.Context, fqn string, since time.Time, limit int) ([]pipelineStatus, error) {
	query := url.Values{}
	query.Set("startTs", strconv.FormatInt(since.UnixMilli(), 10))
	query.Set("endTs", strconv.FormatInt(time.Now().UnixMilli(), 10))
	query.Set("limit", strconv.Itoa(limit))

	var resp listResponse[pipelineStatus]
	if err := c.get(ctx, "/v1/pipelines/"+url.PathEscape(fqn)+"/status", query, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// listOptional is listAll for entity kinds OpenMetadata gained in a
// later release: a server that does not know the kind answers 404, and
// an import from that server should carry on without it rather than
// fail. It reports whether the endpoint existed.
func listOptional[T any](ctx context.Context, c *client, path, fields string, pageSize int, includeDeleted bool) ([]T, bool, error) {
	entities, err := listAll[T](ctx, c, path, fields, pageSize, includeDeleted)
	if err == nil {
		return entities, true, nil
	}

	// Only a 404 on the very first page means the endpoint is absent.
	// One partway through is a real failure, and quietly returning the
	// pages already read would drop the rest without saying so.
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.unsupported() && len(entities) == 0 {
		return nil, false, nil
	}
	return nil, true, err
}

// lineageOf returns the immediate upstream and downstream edges of one
// entity. Depth 1 is enough to see every edge in an OpenMetadata
// instance: asking each entity for its own neighbours visits every edge
// from both ends, and the caller deduplicates.
func (c *client) lineageOf(ctx context.Context, entityType, id string) (*lineageResponse, error) {
	query := url.Values{}
	query.Set("upstreamDepth", "1")
	query.Set("downstreamDepth", "1")

	var resp lineageResponse
	if err := c.get(ctx, "/v1/lineage/"+entityType+"/"+id, query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
