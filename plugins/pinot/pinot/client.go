package pinot

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// tableConfig is the part of a Pinot table config the plugin reads. A
// logical table has one config per type (OFFLINE, REALTIME), each with
// its own segments, tenants and indexing settings.
type tableConfig struct {
	TableName        string           `json:"tableName"`
	TableType        string           `json:"tableType"`
	IsDimTable       bool             `json:"isDimTable"`
	SegmentsConfig   segmentsConfig   `json:"segmentsConfig"`
	Tenants          tenants          `json:"tenants"`
	TableIndexConfig tableIndexConfig `json:"tableIndexConfig"`
	IngestionConfig  *ingestionConfig `json:"ingestionConfig"`
}

type segmentsConfig struct {
	TimeColumnName string `json:"timeColumnName"`
	TimeType       string `json:"timeType"`
	Replication    string `json:"replication"`
	// ReplicasPerPartition is the realtime table's name for replication.
	ReplicasPerPartition string `json:"replicasPerPartition"`
	RetentionTimeUnit    string `json:"retentionTimeUnit"`
	RetentionTimeValue   string `json:"retentionTimeValue"`
	SchemaName           string `json:"schemaName"`
}

type tenants struct {
	Broker string `json:"broker"`
	Server string `json:"server"`
}

type tableIndexConfig struct {
	LoadMode             string   `json:"loadMode"`
	InvertedIndexColumns []string `json:"invertedIndexColumns"`
	SortedColumn         []string `json:"sortedColumn"`
	// StreamConfigs is where Pinot versions before 0.12 keep the stream
	// settings of a realtime table. Newer versions moved them under
	// ingestionConfig; both are read.
	StreamConfigs map[string]string `json:"streamConfigs"`
}

type ingestionConfig struct {
	BatchIngestionConfig  *batchIngestionConfig  `json:"batchIngestionConfig"`
	StreamIngestionConfig *streamIngestionConfig `json:"streamIngestionConfig"`
}

type batchIngestionConfig struct {
	SegmentIngestionType      string `json:"segmentIngestionType"`
	SegmentIngestionFrequency string `json:"segmentIngestionFrequency"`
}

type streamIngestionConfig struct {
	StreamConfigMaps []map[string]string `json:"streamConfigMaps"`
}

// streamConfig returns the stream settings of a realtime table, from
// whichever location this Pinot version wrote them to, or nil for tables
// that are not stream fed.
func (t *tableConfig) streamConfig() map[string]string {
	if t.IngestionConfig != nil && t.IngestionConfig.StreamIngestionConfig != nil {
		for _, m := range t.IngestionConfig.StreamIngestionConfig.StreamConfigMaps {
			if len(m) > 0 {
				return m
			}
		}
	}
	if len(t.TableIndexConfig.StreamConfigs) > 0 {
		return t.TableIndexConfig.StreamConfigs
	}
	return nil
}

// tableSchema is the Pinot schema attached to a table. Pinot groups
// fields by role; the plugin flattens them into one column list.
type tableSchema struct {
	SchemaName          string      `json:"schemaName"`
	DimensionFieldSpecs []fieldSpec `json:"dimensionFieldSpecs"`
	MetricFieldSpecs    []fieldSpec `json:"metricFieldSpecs"`
	DateTimeFieldSpecs  []fieldSpec `json:"dateTimeFieldSpecs"`
	PrimaryKeyColumns   []string    `json:"primaryKeyColumns"`
}

type fieldSpec struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	// SingleValueField is absent from the JSON when true, so a nil pointer
	// means single valued.
	SingleValueField *bool `json:"singleValueField"`
	DefaultNullValue any   `json:"defaultNullValue"`
	NotNull          bool  `json:"notNull"`
	// Format and Granularity are only set on dateTime fields.
	Format      string `json:"format"`
	Granularity string `json:"granularity"`
}

// tableSize is the controller's size report for a logical table.
type tableSize struct {
	TableName            string        `json:"tableName"`
	ReportedSizeInBytes  int64         `json:"reportedSizeInBytes"`
	EstimatedSizeInBytes int64         `json:"estimatedSizeInBytes"`
	OfflineSegments      *segmentsSize `json:"offlineSegments"`
	RealtimeSegments     *segmentsSize `json:"realtimeSegments"`
}

type segmentsSize struct {
	ReportedSizeInBytes  int64 `json:"reportedSizeInBytes"`
	EstimatedSizeInBytes int64 `json:"estimatedSizeInBytes"`
}

// sqlResponse is the broker's query response, whether it was reached
// directly or proxied through the controller.
type sqlResponse struct {
	ResultTable *resultTable   `json:"resultTable"`
	Exceptions  []sqlException `json:"exceptions"`
}

type resultTable struct {
	DataSchema dataSchema `json:"dataSchema"`
	Rows       [][]any    `json:"rows"`
}

type dataSchema struct {
	ColumnNames     []string `json:"columnNames"`
	ColumnDataTypes []string `json:"columnDataTypes"`
}

type sqlException struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
}

// ClientConfig holds what the client needs to reach a Pinot cluster.
type ClientConfig struct {
	ControllerURL string
	// BrokerURL is optional. When empty, queries go through the
	// controller, which forwards them to a broker.
	BrokerURL string
	Username  string
	Password  string
	Token     string
	Database  string
	VerifySSL bool
	Timeout   time.Duration
}

// Client talks to the Pinot controller REST API and, for queries, to a
// broker.
type Client struct {
	controllerURL string
	brokerURL     string
	httpClient    *http.Client
	username      string
	password      string
	token         string
	database      string
}

// NewClient builds a client from the plugin config.
func NewClient(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !config.VerifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opted in by verify_ssl: false
	}

	return &Client{
		controllerURL: strings.TrimSuffix(config.ControllerURL, "/"),
		brokerURL:     strings.TrimSuffix(config.BrokerURL, "/"),
		httpClient:    &http.Client{Timeout: timeout, Transport: transport},
		username:      config.Username,
		password:      config.Password,
		token:         config.Token,
		database:      config.Database,
	}
}

// do sends one request and decodes a JSON response into out. base is the
// controller or broker URL; path is appended to it as is.
func (c *Client) do(ctx context.Context, method, base, path string, query url.Values, body any, out any) error {
	reqURL := base + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	// Pinot 1.1 and later scope a request to a logical database through
	// this header; older versions ignore it.
	if c.database != "" {
		req.Header.Set("database", c.database)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, truncate(string(data), 300))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s response: %w", path, err)
	}
	return nil
}

// truncate keeps error messages readable when the controller answers with
// a full HTML page or stack trace.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Health checks the controller is up. The endpoint answers with the plain
// text OK once the controller has joined the cluster.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.controllerURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("controller health: status %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	if strings.TrimSpace(string(data)) != "OK" {
		return fmt.Errorf("controller health: %s", truncate(string(data), 300))
	}
	return nil
}

// Version returns the Pinot version reported by the controller, or an
// empty string when the endpoint does not list one.
func (c *Client) Version(ctx context.Context) (string, error) {
	var versions map[string]string
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/version", nil, nil, &versions); err != nil {
		return "", err
	}
	// The map is keyed by component (pinot-distribution, pinot-csv, ...)
	// and every component carries the same release version, so any entry
	// will do; the distribution is preferred for a stable answer.
	if v := versions["pinot-distribution"]; v != "" {
		return v, nil
	}
	keys := make([]string, 0, len(versions))
	for key := range versions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if versions[key] != "" {
			return versions[key], nil
		}
	}
	return "", nil
}

// ListTables returns the logical table names, without the _OFFLINE or
// _REALTIME suffix.
func (c *Client) ListTables(ctx context.Context) ([]string, error) {
	var resp struct {
		Tables []string `json:"tables"`
	}
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/tables", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Tables, nil
}

// GetTableConfigs returns the configs of a logical table keyed by type
// (OFFLINE, REALTIME). A hybrid table has both.
func (c *Client) GetTableConfigs(ctx context.Context, table string) (map[string]*tableConfig, error) {
	var resp map[string]*tableConfig
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/tables/"+url.PathEscape(table), nil, nil, &resp); err != nil {
		return nil, err
	}
	configs := make(map[string]*tableConfig, len(resp))
	for tableType, config := range resp {
		if config != nil {
			configs[tableType] = config
		}
	}
	return configs, nil
}

// GetTableSchema returns the schema attached to a logical table.
func (c *Client) GetTableSchema(ctx context.Context, table string) (*tableSchema, error) {
	var schema tableSchema
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/tables/"+url.PathEscape(table)+"/schema", nil, nil, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

// GetSchema returns a schema by its own name, for tables whose config
// points at a schema named differently from the table.
func (c *Client) GetSchema(ctx context.Context, name string) (*tableSchema, error) {
	var schema tableSchema
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/schemas/"+url.PathEscape(name), nil, nil, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

// GetTableSize returns the controller's size report for a logical table.
func (c *Client) GetTableSize(ctx context.Context, table string) (*tableSize, error) {
	query := url.Values{"verbose": {"false"}}
	var size tableSize
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/tables/"+url.PathEscape(table)+"/size", query, nil, &size); err != nil {
		return nil, err
	}
	return &size, nil
}

// ListSegments returns the segment names of a logical table keyed by
// table type. The controller answers with a list of single-key objects,
// one per type.
func (c *Client) ListSegments(ctx context.Context, table string) (map[string][]string, error) {
	var resp []map[string][]string
	if err := c.do(ctx, http.MethodGet, c.controllerURL, "/segments/"+url.PathEscape(table), nil, nil, &resp); err != nil {
		return nil, err
	}
	segments := make(map[string][]string)
	for _, entry := range resp {
		for tableType, names := range entry {
			segments[tableType] = append(segments[tableType], names...)
		}
	}
	return segments, nil
}

// Query runs one SQL statement. It goes to the broker when one is
// configured and through the controller's /sql proxy otherwise. Pinot
// reports query failures inside the body with a 200 status, so a
// non-empty exceptions list is treated as an error.
func (c *Client) Query(ctx context.Context, sql string) (*resultTable, error) {
	base, path := c.controllerURL, "/sql"
	if c.brokerURL != "" {
		base, path = c.brokerURL, "/query/sql"
	}

	var resp sqlResponse
	if err := c.do(ctx, http.MethodPost, base, path, nil, map[string]string{"sql": sql}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Exceptions) > 0 {
		e := resp.Exceptions[0]
		return nil, fmt.Errorf("query failed (error %d): %s", e.ErrorCode, truncate(e.Message, 300))
	}
	if resp.ResultTable == nil {
		return nil, fmt.Errorf("query returned no result table")
	}
	return resp.ResultTable, nil
}

// quoteIdent double-quotes a Pinot identifier so table names that clash
// with SQL keywords interpolate safely. Pinot cannot bind an identifier as
// a query parameter, so the name has to be quoted by hand.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
