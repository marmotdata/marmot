package unitycatalog

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// apiPrefix is the path every Unity Catalog REST endpoint hangs off.
const apiPrefix = "/api/2.1/unity-catalog"

// maxPages bounds how many pages one listing follows. A server that keeps
// handing out page tokens would otherwise never let discovery finish.
const maxPages = 1000

// catalogInfo is the subset of a catalog the plugin reads.
type catalogInfo struct {
	Name       string            `json:"name"`
	Comment    string            `json:"comment"`
	Properties map[string]string `json:"properties"`
	Owner      string            `json:"owner"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
	ID         string            `json:"id"`
}

// schemaInfo is the subset of a schema the plugin reads. Schemas are not
// assets, so only the name is needed to list what they hold.
type schemaInfo struct {
	Name        string `json:"name"`
	CatalogName string `json:"catalog_name"`
}

// tableInfo is the subset of a table the plugin reads.
type tableInfo struct {
	Name             string            `json:"name"`
	CatalogName      string            `json:"catalog_name"`
	SchemaName       string            `json:"schema_name"`
	TableType        string            `json:"table_type"`
	DataSourceFormat string            `json:"data_source_format"`
	Columns          []columnInfo      `json:"columns"`
	StorageLocation  string            `json:"storage_location"`
	Comment          string            `json:"comment"`
	Properties       map[string]string `json:"properties"`
	Owner            string            `json:"owner"`
	CreatedAt        int64             `json:"created_at"`
	UpdatedAt        int64             `json:"updated_at"`
	TableID          string            `json:"table_id"`
	ViewDefinition   string            `json:"view_definition"`
	ViewDependencies *dependencyList   `json:"view_dependencies"`
	TableConstraints []tableConstraint `json:"table_constraints"`
}

// columnInfo is one column of a table. PartitionIndex is a pointer because
// the API sends null for columns that are not partition columns, and 0 is
// a valid index for the first one.
type columnInfo struct {
	Name           string `json:"name"`
	TypeText       string `json:"type_text"`
	TypeName       string `json:"type_name"`
	Position       int    `json:"position"`
	Comment        string `json:"comment"`
	Nullable       bool   `json:"nullable"`
	PartitionIndex *int   `json:"partition_index"`
}

// dependencyList is the view_dependencies block: the tables and functions
// a view reads, as recorded by the server that parsed its definition.
type dependencyList struct {
	Dependencies []dependency `json:"dependencies"`
}

type dependency struct {
	Table *tableDependency `json:"table"`
}

type tableDependency struct {
	TableFullName string `json:"table_full_name"`
}

// tableConstraint holds exactly one of the constraint kinds, matching the
// shape Databricks returns. Each entry in table_constraints carries one
// keyed object.
type tableConstraint struct {
	PrimaryKey *primaryKeyConstraint `json:"primary_key_constraint"`
	ForeignKey *foreignKeyConstraint `json:"foreign_key_constraint"`
}

type primaryKeyConstraint struct {
	Name         string   `json:"name"`
	ChildColumns []string `json:"child_columns"`
}

type foreignKeyConstraint struct {
	Name          string   `json:"name"`
	ChildColumns  []string `json:"child_columns"`
	ParentTable   string   `json:"parent_table"`
	ParentColumns []string `json:"parent_columns"`
}

// volumeInfo is the subset of a volume the plugin reads.
type volumeInfo struct {
	Name            string `json:"name"`
	CatalogName     string `json:"catalog_name"`
	SchemaName      string `json:"schema_name"`
	VolumeType      string `json:"volume_type"`
	StorageLocation string `json:"storage_location"`
	Comment         string `json:"comment"`
	Owner           string `json:"owner"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	VolumeID        string `json:"volume_id"`
}

// functionInfo is the subset of a function the plugin reads.
type functionInfo struct {
	Name              string         `json:"name"`
	CatalogName       string         `json:"catalog_name"`
	SchemaName        string         `json:"schema_name"`
	InputParams       functionParams `json:"input_params"`
	DataType          string         `json:"data_type"`
	FullDataType      string         `json:"full_data_type"`
	RoutineBody       string         `json:"routine_body"`
	RoutineDefinition string         `json:"routine_definition"`
	ExternalLanguage  string         `json:"external_language"`
	IsDeterministic   bool           `json:"is_deterministic"`
	SQLDataAccess     string         `json:"sql_data_access"`
	Comment           string         `json:"comment"`
	Owner             string         `json:"owner"`
	CreatedAt         int64          `json:"created_at"`
	UpdatedAt         int64          `json:"updated_at"`
	FunctionID        string         `json:"function_id"`
}

type functionParams struct {
	Parameters []functionParameter `json:"parameters"`
}

type functionParameter struct {
	Name     string `json:"name"`
	TypeText string `json:"type_text"`
	Position int    `json:"position"`
}

// registeredModelInfo is the subset of a registered model the plugin reads.
type registeredModelInfo struct {
	Name            string `json:"name"`
	CatalogName     string `json:"catalog_name"`
	SchemaName      string `json:"schema_name"`
	StorageLocation string `json:"storage_location"`
	Comment         string `json:"comment"`
	Owner           string `json:"owner"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	ID              string `json:"id"`
}

// modelVersionInfo is the subset of a model version the plugin reads.
type modelVersionInfo struct {
	Version int    `json:"version"`
	Status  string `json:"status"`
}

// apiError is the body Unity Catalog returns with a 4xx or 5xx status.
type apiError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// httpError is a non-2xx response, kept typed so callers can tell a
// missing endpoint apart from a broken server.
type httpError struct {
	Status  int
	Message string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.Status, e.Message)
}

// isNotFound reports whether err is a 404 from the server.
func isNotFound(err error) bool {
	var he *httpError
	return errors.As(err, &he) && he.Status == http.StatusNotFound
}

// client talks to one Unity Catalog server.
type client struct {
	baseURL    string
	token      string
	pageSize   int
	httpClient *http.Client
}

func newClient(host, token string, pageSize int, verifySSL bool) *client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !verifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user opted out of verification
	}

	return &client{
		baseURL:  host + apiPrefix,
		token:    token,
		pageSize: pageSize,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// do performs one request and decodes the JSON body into out.
func (c *client) do(ctx context.Context, method, path string, query url.Values, out any) error {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr apiError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			return &httpError{Status: resp.StatusCode, Message: apiErr.Message}
		}
		return &httpError{Status: resp.StatusCode, Message: string(body)}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	return nil
}

// list follows a paged listing to its end. Every list endpoint returns the
// items under its own key ("tables", "volumes", ...) next to a
// next_page_token, so the page is decoded generically and the items
// pulled out by key. The token is opaque: the loop stops when it is null,
// when a page comes back empty, or when the server repeats the token it
// was just given.
func list[T any](ctx context.Context, c *client, path, key string, query url.Values) ([]T, error) {
	var all []T
	token := ""

	for pages := 0; pages < maxPages; pages++ {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("max_results", strconv.Itoa(c.pageSize))
		if token != "" {
			q.Set("page_token", token)
		}

		var page map[string]json.RawMessage
		if err := c.do(ctx, http.MethodGet, path, q, &page); err != nil {
			return nil, err
		}

		var items []T
		if raw, ok := page[key]; ok && len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", key, err)
			}
		}
		all = append(all, items...)

		next := ""
		if raw, ok := page["next_page_token"]; ok {
			_ = json.Unmarshal(raw, &next)
		}
		if next == "" || next == token || len(items) == 0 {
			return all, nil
		}
		token = next
	}

	log.Warn().Str("path", path).Int("pages", maxPages).Msg("Stopped following pages at the cap; results may be incomplete")
	return all, nil
}

func (c *client) listCatalogs(ctx context.Context) ([]catalogInfo, error) {
	return list[catalogInfo](ctx, c, "/catalogs", "catalogs", nil)
}

func (c *client) listSchemas(ctx context.Context, catalog string) ([]schemaInfo, error) {
	return list[schemaInfo](ctx, c, "/schemas", "schemas", url.Values{"catalog_name": {catalog}})
}

func (c *client) listTables(ctx context.Context, catalog, schema string) ([]tableInfo, error) {
	return list[tableInfo](ctx, c, "/tables", "tables", url.Values{"catalog_name": {catalog}, "schema_name": {schema}})
}

// getTable loads one table by its three-part name, for servers whose
// listing leaves the columns out.
func (c *client) getTable(ctx context.Context, fullName string) (*tableInfo, error) {
	var table tableInfo
	if err := c.do(ctx, http.MethodGet, "/tables/"+url.PathEscape(fullName), nil, &table); err != nil {
		return nil, err
	}
	return &table, nil
}

func (c *client) listVolumes(ctx context.Context, catalog, schema string) ([]volumeInfo, error) {
	return list[volumeInfo](ctx, c, "/volumes", "volumes", url.Values{"catalog_name": {catalog}, "schema_name": {schema}})
}

func (c *client) listFunctions(ctx context.Context, catalog, schema string) ([]functionInfo, error) {
	return list[functionInfo](ctx, c, "/functions", "functions", url.Values{"catalog_name": {catalog}, "schema_name": {schema}})
}

func (c *client) listModels(ctx context.Context, catalog, schema string) ([]registeredModelInfo, error) {
	return list[registeredModelInfo](ctx, c, "/models", "registered_models", url.Values{"catalog_name": {catalog}, "schema_name": {schema}})
}

func (c *client) listModelVersions(ctx context.Context, fullName string) ([]modelVersionInfo, error) {
	return list[modelVersionInfo](ctx, c, "/models/"+url.PathEscape(fullName)+"/versions", "model_versions", nil)
}
