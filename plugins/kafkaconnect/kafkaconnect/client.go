package kafkaconnect

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// serverInfo is the root document of a Connect worker.
type serverInfo struct {
	Version        string `json:"version"`
	Commit         string `json:"commit"`
	KafkaClusterID string `json:"kafka_cluster_id"`
}

// connectorInfo is a connector's definition: its config and the tasks
// the worker split it into.
type connectorInfo struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
	// Configs is the shape Confluent Cloud uses instead of Config.
	Configs []configEntry `json:"configs"`
	Tasks   []taskRef     `json:"tasks"`
	Type    string        `json:"type"`
}

type configEntry struct {
	Config string `json:"config"`
	Value  string `json:"value"`
}

type taskRef struct {
	Connector string `json:"connector"`
	Task      int    `json:"task"`
}

// connectorStatus is a connector's runtime state on the workers.
type connectorStatus struct {
	Name      string       `json:"name"`
	Connector instanceInfo `json:"connector"`
	Tasks     []taskStatus `json:"tasks"`
	Type      string       `json:"type"`
}

type instanceInfo struct {
	State    string `json:"state"`
	WorkerID string `json:"worker_id"`
	Trace    string `json:"trace"`
}

type taskStatus struct {
	ID       int    `json:"id"`
	State    string `json:"state"`
	WorkerID string `json:"worker_id"`
	Trace    string `json:"trace"`
}

// connector is one connector with both halves the REST API exposes.
type connector struct {
	Info   connectorInfo   `json:"info"`
	Status connectorStatus `json:"status"`
}

// connectorPlugin is one installed connector class.
type connectorPlugin struct {
	Class   string `json:"class"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

// config returns the connector's config as a flat map. Confluent Cloud
// returns a list of {config, value} pairs where self-hosted Connect
// returns a map, so both are folded into one shape here.
func (i connectorInfo) config() map[string]string {
	if len(i.Config) > 0 || len(i.Configs) == 0 {
		return i.Config
	}
	folded := make(map[string]string, len(i.Configs))
	for _, entry := range i.Configs {
		folded[entry.Config] = entry.Value
	}
	return folded
}

// client talks to the Kafka Connect REST API.
type client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func newClient(host, username, password string, verifySSL bool) *client {
	transport := http.DefaultTransport
	if !verifySSL {
		insecure := http.DefaultTransport.(*http.Transport).Clone()
		insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		transport = insecure
	}

	return &client{
		baseURL:  strings.TrimSuffix(host, "/"),
		username: username,
		password: password,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// apiError is a non-2xx response from Connect. It is a distinct type so
// callers can react to the status code, for example to notice that an
// endpoint does not exist on this version.
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

// do performs one request against a path below the base URL and decodes
// the JSON response into out when out is not nil.
func (c *client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request for %s: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &apiError{Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

func (c *client) serverInfo(ctx context.Context) (*serverInfo, error) {
	var info serverInfo
	if err := c.do(ctx, http.MethodGet, "/", nil, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// connectors returns every connector with its config and status, sorted
// by name.
//
// Connect 2.3 and later answer one request for everything when asked to
// expand both. A worker that rejects the double expand still answers the
// status-only form, and one that predates expand altogether returns a
// bare list of names; both are followed up connector by connector.
func (c *client) connectors(ctx context.Context) ([]connector, error) {
	query := url.Values{"expand": {"status", "info"}}

	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, "/connectors", query, nil, &raw)
	if err != nil {
		var apiErr *apiError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
			return nil, err
		}
		query = url.Values{"expand": {"status"}}
		if err := c.do(ctx, http.MethodGet, "/connectors", query, nil, &raw); err != nil {
			return nil, err
		}
	}

	if isJSONArray(raw) {
		var names []string
		if err := json.Unmarshal(raw, &names); err != nil {
			return nil, fmt.Errorf("decoding connector names: %w", err)
		}
		return c.connectorsByName(ctx, names)
	}

	var expanded map[string]struct {
		Info   *connectorInfo   `json:"info"`
		Status *connectorStatus `json:"status"`
	}
	if err := json.Unmarshal(raw, &expanded); err != nil {
		return nil, fmt.Errorf("decoding connectors: %w", err)
	}

	names := make([]string, 0, len(expanded))
	for name := range expanded {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]connector, 0, len(names))
	for _, name := range names {
		entry := expanded[name]
		conn := connector{}
		if entry.Info != nil {
			conn.Info = *entry.Info
		} else {
			info, err := c.connectorInfo(ctx, name)
			if err != nil {
				return nil, err
			}
			conn.Info = *info
		}
		if entry.Status != nil {
			conn.Status = *entry.Status
		} else {
			status, err := c.connectorStatus(ctx, name)
			if err != nil {
				return nil, err
			}
			conn.Status = *status
		}
		if conn.Info.Name == "" {
			conn.Info.Name = name
		}
		result = append(result, conn)
	}
	return result, nil
}

func (c *client) connectorsByName(ctx context.Context, names []string) ([]connector, error) {
	sort.Strings(names)
	result := make([]connector, 0, len(names))
	for _, name := range names {
		info, err := c.connectorInfo(ctx, name)
		if err != nil {
			return nil, err
		}
		status, err := c.connectorStatus(ctx, name)
		if err != nil {
			return nil, err
		}
		if info.Name == "" {
			info.Name = name
		}
		result = append(result, connector{Info: *info, Status: *status})
	}
	return result, nil
}

func (c *client) connectorInfo(ctx context.Context, name string) (*connectorInfo, error) {
	var info connectorInfo
	if err := c.do(ctx, http.MethodGet, "/connectors/"+url.PathEscape(name), nil, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *client) connectorStatus(ctx context.Context, name string) (*connectorStatus, error) {
	var status connectorStatus
	if err := c.do(ctx, http.MethodGet, "/connectors/"+url.PathEscape(name)+"/status", nil, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// errTopicTrackingUnavailable reports that the worker cannot answer the
// active topics endpoint at all, either because it predates KIP-558
// (Kafka 2.5) or because topic tracking is switched off, so there is no
// point asking it again for another connector.
var errTopicTrackingUnavailable = errors.New("topic tracking unavailable")

// connectorTopics returns the topics a connector has actually read from
// or written to since its last reset.
func (c *client) connectorTopics(ctx context.Context, name string) ([]string, error) {
	var resp map[string]struct {
		Topics []string `json:"topics"`
	}
	err := c.do(ctx, http.MethodGet, "/connectors/"+url.PathEscape(name)+"/topics", nil, nil, &resp)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && topicTrackingUnavailable(apiErr) {
			return nil, fmt.Errorf("%w: %v", errTopicTrackingUnavailable, err)
		}
		return nil, err
	}

	// The worker keys the answer by the connector name it was asked for.
	entry := resp[name]
	topics := append([]string(nil), entry.Topics...)
	sort.Strings(topics)
	return topics, nil
}

func topicTrackingUnavailable(err *apiError) bool {
	switch err.Status {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	case http.StatusForbidden:
		return strings.Contains(strings.ToLower(err.Body), "topic tracking is disabled")
	}
	return false
}

func (c *client) connectorPlugins(ctx context.Context) ([]connectorPlugin, error) {
	var plugins []connectorPlugin
	if err := c.do(ctx, http.MethodGet, "/connector-plugins", nil, nil, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

func isJSONArray(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '['
}
