package elasticsearch

import (
	"context"
	"fmt"
	"net/http"

	"github.com/marmotdata/marmot/pkg/config"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// Client wraps the opensearch-go client. The package is still named
// "elasticsearch" because it targets both engines — opensearch-go is a leaner
// HTTP client that speaks the shared ES/OS REST surface.
type Client struct {
	es       *opensearchapi.Client
	index    string
	shards   *int
	replicas *int
}

// NewClient creates a new search client from configuration.
func NewClient(cfg *config.ElasticsearchConfig) (*Client, error) {
	osCfg := opensearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
	}

	if cfg.TLS != nil {
		tlsCfg, err := cfg.TLS.ToTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("configuring TLS: %w", err)
		}
		if tlsCfg != nil {
			osCfg.Transport = &http.Transport{
				TLSClientConfig: tlsCfg,
			}
		}
	}

	es, err := opensearchapi.NewClient(opensearchapi.Config{Client: osCfg})
	if err != nil {
		return nil, fmt.Errorf("creating search client: %w", err)
	}

	index := cfg.Index
	if index == "" {
		index = "marmot"
	}

	return &Client{
		es:       es,
		index:    index,
		shards:   cfg.Shards,
		replicas: cfg.Replicas,
	}, nil
}

// Healthy checks if the search cluster is reachable.
func (c *Client) Healthy(ctx context.Context) bool {
	resp, err := c.es.Ping(ctx, nil)
	if err != nil {
		return false
	}
	return !resp.IsError()
}

// Close is a no-op for the HTTP-based client.
func (c *Client) Close() error {
	return nil
}
