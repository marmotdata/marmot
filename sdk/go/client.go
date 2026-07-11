package marmot

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/marmotdata/marmot/sdk/go/auth"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client"
)

const (
	DefaultHost      = "http://localhost:8080"
	DefaultBasePath  = "/api/v1"
	DefaultUserAgent = "marmot-sdk-go"
)

// ClientOptions configures a Client. All fields are optional.
type ClientOptions struct {
	// Host is the Marmot server URL. Falls back to MARMOT_HOST, the active
	// context, then DefaultHost.
	Host string

	// BasePath overrides the URL prefix appended to Host (default DefaultBasePath).
	BasePath string

	APIKey string
	Token  string

	// Context selects a named context from credentials.json. Falls back to
	// the active context from config.yaml.
	Context string

	// Credential bypasses the auth resolution chain entirely.
	Credential auth.Credential

	HTTPClient *http.Client
	UserAgent  string
}

// Client is the entry point. Use NewClient to construct one.
type Client struct {
	Admin           *AdminService
	APIKeys         *APIKeysService
	Assets          *AssetsService
	DataProducts    *DataProductsService
	Glossary        *GlossaryService
	Lineage         *LineageService
	Metrics         *MetricsService
	Owners          *OwnersService
	Runs            *RunsService
	Search          *SearchService
	ServiceAccounts *ServiceAccountsService
	Teams           *TeamsService
	Users           *UsersService

	host string
	cred auth.Credential
}

// NewClient resolves credentials and host, and returns a ready-to-use Client.
func NewClient(opts ClientOptions) (*Client, error) {
	cfg, err := loadConfigFile()
	if err != nil {
		return nil, err
	}

	host, err := resolveHost(opts.Host, cfg.activeHost())
	if err != nil {
		return nil, err
	}

	cred, err := resolveCredential(opts, cfg)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse host %q: %w", host, err)
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}

	basePath := opts.BasePath
	if basePath == "" {
		basePath = DefaultBasePath
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	httpClient = withUserAgent(httpClient, ua)

	transport := httptransport.NewWithClient(u.Host, basePath, []string{scheme}, httpClient)
	transport.DefaultAuthentication = cred.AuthInfo()
	gen := client.New(transport, strfmt.Default)

	return &Client{
		Admin:           &AdminService{gen: gen},
		APIKeys:         &APIKeysService{gen: gen},
		Assets:          &AssetsService{gen: gen},
		DataProducts:    &DataProductsService{gen: gen},
		Glossary:        &GlossaryService{gen: gen},
		Lineage:         &LineageService{gen: gen},
		Metrics:         &MetricsService{gen: gen},
		Owners:          &OwnersService{gen: gen},
		Runs:            &RunsService{gen: gen},
		Search:          &SearchService{gen: gen},
		ServiceAccounts: &ServiceAccountsService{gen: gen},
		Teams:           &TeamsService{gen: gen},
		Users:           &UsersService{gen: gen},
		host:            host,
		cred:            cred,
	}, nil
}

// Host returns the resolved server URL.
func (c *Client) Host() string { return c.host }

// Credential returns the credential the client is using.
func (c *Client) Credential() auth.Credential { return c.cred }

func resolveHost(explicit, fromConfig string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if v := os.Getenv("MARMOT_HOST"); v != "" {
		return v, nil
	}
	if fromConfig != "" {
		return fromConfig, nil
	}
	return DefaultHost, nil
}

func resolveCredential(opts ClientOptions, cfg *configFile) (auth.Credential, error) {
	if opts.Credential != nil {
		return opts.Credential, nil
	}
	if opts.APIKey != "" {
		return auth.APIKey(opts.APIKey), nil
	}
	if opts.Token != "" {
		return auth.Bearer(opts.Token), nil
	}
	if v := os.Getenv("MARMOT_API_KEY"); v != "" {
		return auth.APIKey(v), nil
	}
	if v := os.Getenv("MARMOT_TOKEN"); v != "" {
		return auth.Bearer(v), nil
	}
	ctx := opts.Context
	if ctx == "" {
		ctx = cfg.CurrentContext
	}
	if c := auth.CachedToken(ctx); c != nil {
		return c, nil
	}
	if cfg.APIKey != "" {
		return auth.APIKey(cfg.APIKey), nil
	}
	if c := auth.WorkloadCredential(); c != nil {
		return c, nil
	}
	return nil, errors.New("marmot: no credentials available (set APIKey/Token, MARMOT_API_KEY, run `marmot login`, or run in a Kubernetes pod)")
}

func withUserAgent(c *http.Client, ua string) *http.Client {
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone := *c
	clone.Transport = &userAgentTransport{base: base, userAgent: ua}
	return &clone
}

type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !hasUserAgent(req) {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.userAgent)
	}
	return t.base.RoundTrip(req)
}

func hasUserAgent(req *http.Request) bool {
	for _, v := range req.Header.Values("User-Agent") {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}
