package elasticsearch

import (
	"context"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/marmotdata/marmot/internal/config"
)

// Client wraps the Elasticsearch client with connection lifecycle management.
type Client struct {
	es       *elasticsearch.TypedClient
	index    string
	shards   *int
	replicas *int
}

// NewClient creates a new Elasticsearch client from configuration.
func NewClient(cfg *config.ElasticsearchConfig) (*Client, error) {
	esCfg := elasticsearch.Config{
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
			esCfg.Transport = &http.Transport{
				TLSClientConfig: tlsCfg,
			}
		}
	}

	es, err := elasticsearch.NewTypedClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("creating elasticsearch client: %w", err)
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

// Healthy checks if the Elasticsearch cluster is reachable.
func (c *Client) Healthy(ctx context.Context) bool {
	ok, err := c.es.Ping().Do(ctx)
	if err != nil {
		return false
	}
	return ok
}

// Close is a no-op for the HTTP-based ES client.
func (c *Client) Close() error {
	return nil
}
