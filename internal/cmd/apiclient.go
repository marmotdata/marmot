package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	apiclient "github.com/marmotdata/marmot/client/client"
	"github.com/spf13/viper"
)

// newSwaggerClient creates a go-swagger Marmot client configured from viper.
func newSwaggerClient() *apiclient.Marmot {
	host := viper.GetString("host")
	apiKey := viper.GetString("api_key")

	u, err := url.Parse(host)
	if err != nil {
		u = &url.URL{Host: "localhost:8080", Scheme: "http"}
	}

	hostWithPort := u.Host
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}

	cfg := apiclient.DefaultTransportConfig().
		WithHost(hostWithPort).
		WithBasePath("/api/v1").
		WithSchemes([]string{scheme})

	transport := httptransport.New(cfg.Host, cfg.BasePath, cfg.Schemes)
	if apiKey != "" {
		transport.DefaultAuthentication = httptransport.APIKeyAuth("X-API-Key", "header", apiKey)
	}

	return apiclient.New(transport, strfmt.Default)
}

// marshalPayload marshals any go-swagger payload to JSON bytes for raw output.
func marshalPayload(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshalling response: %w", err)
	}
	return data, nil
}
