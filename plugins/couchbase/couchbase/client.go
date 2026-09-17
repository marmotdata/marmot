package couchbase

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/rs/zerolog/log"
)

// connect opens the cluster connection and waits until the SDK has a
// cluster map, so a wrong host or bad credentials fail here instead of on
// the first query.
func (s *Source) connect(ctx context.Context) error {
	s.close()

	connectTimeout := time.Duration(s.config.ConnectTimeoutSeconds) * time.Second

	cluster, err := gocb.Connect(s.config.ConnectionString, gocb.ClusterOptions{
		Username: s.config.Username,
		Password: s.config.Password,
		TimeoutsConfig: gocb.TimeoutsConfig{
			ConnectTimeout:    connectTimeout,
			KVTimeout:         30 * time.Second,
			QueryTimeout:      30 * time.Second,
			ManagementTimeout: 30 * time.Second,
		},
		SecurityConfig: gocb.SecurityConfig{
			TLSSkipVerify: s.config.SSLSkipVerify,
		},
	})
	if err != nil {
		return fmt.Errorf("opening cluster connection: %w", err)
	}

	if err := cluster.WaitUntilReady(connectTimeout, &gocb.WaitUntilReadyOptions{Context: ctx}); err != nil {
		_ = cluster.Close(nil)
		return fmt.Errorf("waiting for cluster: %w", err)
	}

	log.Debug().Str("connection_string", s.config.ConnectionString).Msg("Connected to Couchbase")

	s.cluster = cluster
	return nil
}

func (s *Source) close() {
	if s.cluster != nil {
		_ = s.cluster.Close(nil)
		s.cluster = nil
	}
}

// query runs a read-only N1QL statement and returns its rows as raw JSON.
func (s *Source) query(ctx context.Context, statement string, params map[string]interface{}) ([]json.RawMessage, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := s.cluster.Query(statement, &gocb.QueryOptions{
		Context:         queryCtx,
		Timeout:         30 * time.Second,
		Readonly:        true,
		Adhoc:           true,
		NamedParameters: params,
	})
	if err != nil {
		return nil, err
	}
	defer result.Close()

	var rows []json.RawMessage
	for result.Next() {
		var row json.RawMessage
		if err := result.Row(&row); err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}
		rows = append(rows, row)
	}
	if err := result.Err(); err != nil {
		return nil, err
	}

	return rows, nil
}

func decodeRow(row json.RawMessage, out interface{}) error {
	return json.Unmarshal(row, out)
}

// isIndexError reports whether a query failed because the keyspace has no
// index the query service can use, which is the normal state of a
// collection nobody queries with N1QL.
func isIndexError(err error) bool {
	if errors.Is(err, gocb.ErrIndexNotFound) || errors.Is(err, gocb.ErrIndexFailure) || errors.Is(err, gocb.ErrPlanningFailure) {
		return true
	}
	// Older servers report the missing index as a planning failure whose
	// message is the only reliable signal.
	return strings.Contains(strings.ToLower(err.Error()), "no index available")
}

// managementClient talks to the cluster management REST API, which the SDK
// does not expose. It reaches the first host of the connection string on
// the standard management port, over TLS for couchbases://.
type managementClient struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func newManagementClient(config *Config) *managementClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if config.SSLSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // G402: opt-in via ssl_skip_verify
	}

	return &managementClient{
		baseURL:  managementURL(config.ConnectionString),
		username: config.Username,
		password: config.Password,
		http:     &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

// managementURL derives the management API base URL from a connection
// string: the first host, without any port the user gave for the SDK, on
// 8091 for plain connections and 18091 for TLS.
func managementURL(connectionString string) string {
	scheme, rest, _ := strings.Cut(connectionString, "://")
	if rest == "" {
		rest = scheme
		scheme = "couchbase"
	}

	// Drop the ?options and any /bucket path the SDK accepts.
	rest, _, _ = strings.Cut(rest, "?")
	rest, _, _ = strings.Cut(rest, "/")
	host, _, _ := strings.Cut(rest, ",")
	host = strings.TrimSpace(host)

	// Strip a port, keeping IPv6 literals intact.
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end != -1 {
			host = host[:end+1]
		}
	} else if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}

	if scheme == "couchbases" {
		return "https://" + host + ":18091"
	}
	return "http://" + host + ":8091"
}

// poolsResponse is the part of GET /pools this plugin reads.
type poolsResponse struct {
	ImplementationVersion string `json:"implementationVersion"`
}

// bucketDetails is the part of GET /pools/default/buckets/{name} this
// plugin reads.
type bucketDetails struct {
	Name                   string `json:"name"`
	ConflictResolutionType string `json:"conflictResolutionType"`
	BasicStats             struct {
		ItemCount int64 `json:"itemCount"`
		DiskUsed  int64 `json:"diskUsed"`
		DataUsed  int64 `json:"dataUsed"`
		MemUsed   int64 `json:"memUsed"`
	} `json:"basicStats"`
}

func (c *managementClient) clusterVersion(ctx context.Context) (string, error) {
	var pools poolsResponse
	if err := c.do(ctx, "/pools", &pools); err != nil {
		return "", err
	}
	return pools.ImplementationVersion, nil
}

func (c *managementClient) bucket(ctx context.Context, name string) (*bucketDetails, error) {
	var details bucketDetails
	if err := c.do(ctx, "/pools/default/buckets/"+url.PathEscape(name), &details); err != nil {
		return nil, err
	}
	return &details, nil
}

func (c *managementClient) do(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", path, resp.StatusCode)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}

	return nil
}
