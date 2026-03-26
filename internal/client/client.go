package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	apiPrefix      = "/api/v1"
)

// APIError represents an error response from the Marmot API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error (HTTP %d)", e.StatusCode)
}

// Client is a lightweight HTTP client for the Marmot API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Marmot API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// get performs a GET request and decodes the response into v.
func (c *Client) get(ctx context.Context, path string, query url.Values, v interface{}) error {
	return c.do(ctx, http.MethodGet, path, query, nil, v)
}

// post performs a POST request with a JSON body and decodes the response into v.
func (c *Client) post(ctx context.Context, path string, body, v interface{}) error {
	return c.do(ctx, http.MethodPost, path, nil, body, v)
}

// put performs a PUT request with a JSON body and decodes the response into v.
func (c *Client) put(ctx context.Context, path string, body, v interface{}) error {
	return c.do(ctx, http.MethodPut, path, nil, body, v)
}

// del performs a DELETE request and decodes the response into v.
func (c *Client) del(ctx context.Context, path string, query url.Values, v interface{}) error {
	return c.do(ctx, http.MethodDelete, path, query, nil, v)
}

// delWithBody performs a DELETE request with a JSON body and decodes the response into v.
func (c *Client) delWithBody(ctx context.Context, path string, body, v interface{}) error {
	return c.do(ctx, http.MethodDelete, path, nil, body, v)
}

// getRaw performs a GET request and returns the raw response body as bytes.
func (c *Client) getRaw(ctx context.Context, path string, query url.Values) ([]byte, error) {
	u := c.baseURL + apiPrefix + path
	if query != nil {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, parseError(resp.StatusCode, data)
	}

	return data, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, v interface{}) error {
	u := c.baseURL + apiPrefix + path
	if query != nil {
		u += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return parseError(resp.StatusCode, data)
	}

	if v != nil && len(data) > 0 {
		return json.Unmarshal(data, v)
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}

func parseError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode}

	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		apiErr.Message = errResp.Error
	} else if len(body) > 0 {
		apiErr.Message = string(body)
	}

	return apiErr
}
