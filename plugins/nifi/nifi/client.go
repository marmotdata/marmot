package nifi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// apiRoot is where NiFi serves its REST API, below the UI base URL.
const apiRoot = "/nifi-api"

// client talks to one NiFi instance.
type client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func newClient(config *Config) (*client, error) {
	tlsConfig, err := buildTLSConfig(config)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	return &client{
		baseURL:    config.Host,
		httpClient: &http.Client{Timeout: 30 * time.Second, Transport: transport},
		token:      config.Token,
	}, nil
}

// buildTLSConfig applies the certificate settings. NiFi ships with a
// self-signed certificate, so verify_ssl=false is the common case for
// test installs; a CA file covers the private-CA case and a client
// certificate pair enables mutual TLS, which NiFi accepts instead of a
// login.
func buildTLSConfig(config *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: !config.VerifySSL, //nolint:gosec // G402: user opted out of verification
	}

	if config.CACert != "" {
		pem, err := os.ReadFile(config.CACert)
		if err != nil {
			return nil, fmt.Errorf("reading ca_cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca_cert %s contains no PEM certificates", config.CACert)
		}
		tlsConfig.RootCAs = pool
	}

	if config.ClientCert != "" {
		cert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// login exchanges a username and password for a bearer token. NiFi
// answers the form post with the raw JWT as the body, not with JSON.
func (c *client) login(ctx context.Context, username, password string) error {
	form := url.Values{"username": {username}, "password": {password}}

	body, err := c.do(ctx, http.MethodPost, "/access/token", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return fmt.Errorf("logging in: %w", err)
	}

	token := strings.TrimSpace(string(body))
	if token == "" {
		return fmt.Errorf("logging in: NiFi returned an empty token")
	}

	c.token = token
	return nil
}

// do performs one request against the API root and returns the raw
// body. Error bodies are trimmed so a NiFi HTML error page does not
// flood the log.
func (c *client) do(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+apiRoot+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json, text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(data))
		if len(message) > 200 {
			message = message[:200]
		}
		return nil, fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, message)
	}

	return data, nil
}

func (c *client) get(ctx context.Context, path string, out any) error {
	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// version returns the NiFi release the instance runs.
func (c *client) version(ctx context.Context) (string, error) {
	var resp aboutEntity
	if err := c.get(ctx, "/flow/about", &resp); err != nil {
		return "", err
	}
	return resp.About.Version, nil
}

// processGroupFlow returns a group's contents: child groups, processors,
// ports and the connections between them. "root" is accepted as an id.
func (c *client) processGroupFlow(ctx context.Context, id string) (*processGroupFlow, error) {
	var resp processGroupFlowEntity
	if err := c.get(ctx, "/flow/process-groups/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	return &resp.ProcessGroupFlow, nil
}

// processGroup returns a group's own component: name, comments, counts
// and parameter context. The flow listing carries the same for child
// groups, so this is only needed for the group discovery starts from.
func (c *client) processGroup(ctx context.Context, id string) (*processGroupEntity, error) {
	var resp processGroupEntity
	if err := c.get(ctx, "/process-groups/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// controllerService returns one controller service, used to read the
// JDBC URL of a connection pool a database processor points at.
func (c *client) controllerService(ctx context.Context, id string) (*controllerServiceEntity, error) {
	var resp controllerServiceEntity
	if err := c.get(ctx, "/controller-services/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// The response shapes below carry only the fields discovery reads.

type aboutEntity struct {
	About struct {
		Version string `json:"version"`
	} `json:"about"`
}

type processGroupFlowEntity struct {
	ProcessGroupFlow processGroupFlow `json:"processGroupFlow"`
}

type processGroupFlow struct {
	ID         string           `json:"id"`
	Breadcrumb breadcrumbEntity `json:"breadcrumb"`
	Flow       flowContents     `json:"flow"`
}

// breadcrumbEntity is one link of the path from the root group down to
// the group listed. NiFi nests the parent's breadcrumb inside each one.
type breadcrumbEntity struct {
	Breadcrumb       breadcrumb        `json:"breadcrumb"`
	ParentBreadcrumb *breadcrumbEntity `json:"parentBreadcrumb"`
}

type breadcrumb struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type flowContents struct {
	ProcessGroups []processGroupEntity `json:"processGroups"`
	Processors    []processorEntity    `json:"processors"`
	Connections   []connectionEntity   `json:"connections"`
	InputPorts    []portEntity         `json:"inputPorts"`
	OutputPorts   []portEntity         `json:"outputPorts"`
}

type processGroupEntity struct {
	ID        string                `json:"id"`
	Component processGroupComponent `json:"component"`
}

type processGroupComponent struct {
	ID               string                     `json:"id"`
	ParentGroupID    string                     `json:"parentGroupId"`
	Name             string                     `json:"name"`
	Comments         string                     `json:"comments"`
	RunningCount     int                        `json:"runningCount"`
	StoppedCount     int                        `json:"stoppedCount"`
	InvalidCount     int                        `json:"invalidCount"`
	DisabledCount    int                        `json:"disabledCount"`
	ParameterContext *parameterContextReference `json:"parameterContext"`
}

type parameterContextReference struct {
	ID        string `json:"id"`
	Component struct {
		Name string `json:"name"`
	} `json:"component"`
}

type processorEntity struct {
	ID        string             `json:"id"`
	Component processorComponent `json:"component"`
	Status    processorStatus    `json:"status"`
}

type processorComponent struct {
	ID               string          `json:"id"`
	ParentGroupID    string          `json:"parentGroupId"`
	Name             string          `json:"name"`
	Type             string          `json:"type"`
	State            string          `json:"state"`
	ValidationStatus string          `json:"validationStatus"`
	Config           processorConfig `json:"config"`
	Relationships    []relationship  `json:"relationships"`
}

// processorConfig holds the property values keyed by property name. A
// value is null when the property is unset, and NiFi already replaces
// sensitive values with asterisks before they leave the server; the
// descriptors say which ones those are.
type processorConfig struct {
	Properties         map[string]*string            `json:"properties"`
	Descriptors        map[string]propertyDescriptor `json:"descriptors"`
	SchedulingPeriod   string                        `json:"schedulingPeriod"`
	SchedulingStrategy string                        `json:"schedulingStrategy"`
	Comments           string                        `json:"comments"`
}

type propertyDescriptor struct {
	Sensitive bool `json:"sensitive"`
}

type relationship struct {
	Name string `json:"name"`
}

type processorStatus struct {
	RunStatus string `json:"runStatus"`
}

type connectionEntity struct {
	ID        string              `json:"id"`
	Component connectionComponent `json:"component"`
}

type connectionComponent struct {
	Source                connectable `json:"source"`
	Destination           connectable `json:"destination"`
	SelectedRelationships []string    `json:"selectedRelationships"`
}

// connectable is either end of a connection. Type is PROCESSOR,
// INPUT_PORT, OUTPUT_PORT, FUNNEL or one of the remote port kinds, and
// GroupID is the group the component itself lives in, which for a port
// differs from the group the connection is listed under.
type connectable struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	GroupID string `json:"groupId"`
	Name    string `json:"name"`
}

type portEntity struct {
	ID        string        `json:"id"`
	Component portComponent `json:"component"`
}

type portComponent struct {
	ID            string `json:"id"`
	ParentGroupID string `json:"parentGroupId"`
	Name          string `json:"name"`
	State         string `json:"state"`
	Type          string `json:"type"`
	Comments      string `json:"comments"`
}

type controllerServiceEntity struct {
	ID        string                     `json:"id"`
	Component controllerServiceComponent `json:"component"`
}

type controllerServiceComponent struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Properties map[string]*string `json:"properties"`
}
