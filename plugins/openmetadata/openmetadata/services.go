package openmetadata

import (
	"context"
	"net"
	"strings"

	"github.com/rs/zerolog/log"
)

// A service in OpenMetadata is a system someone registered by address, so its connection is what tells Marmot which zone of a technology an imported asset belongs to. The addresses are read once per run and travel on every MRN built for the service as a host hint, unless service_map names the zone outright.

// serviceListPaths are the OpenMetadata service endpoints whose connection holds an address.
var serviceListPaths = []string{
	"/v1/services/databaseServices",
	"/v1/services/messagingServices",
	"/v1/services/searchServices",
	"/v1/services/dashboardServices",
	"/v1/services/pipelineServices",
	"/v1/services/mlmodelServices",
}

// service is an OpenMetadata service with its connection.
type service struct {
	Name        string `json:"name"`
	ServiceType string `json:"serviceType"`
	Connection  struct {
		Config map[string]interface{} `json:"config"`
	} `json:"connection"`
}

// connectionAddressKeys are the connection settings that hold a service's address, in the order they are tried.
var connectionAddressKeys = []string{"hostPort", "bootstrapServers", "connectionURI", "trackingUri", "host"}

// defaultPorts are the ports a service type listens on when its connection leaves the port out.
var defaultPorts = map[string]string{
	"Postgres":      "5432",
	"Timescale":     "5432",
	"Cockroach":     "26257",
	"Mysql":         "3306",
	"MariaDB":       "3306",
	"Doris":         "9030",
	"StarRocks":     "9030",
	"Mssql":         "1433",
	"Oracle":        "1521",
	"Clickhouse":    "8123",
	"MongoDB":       "27017",
	"Cassandra":     "9042",
	"Trino":         "8080",
	"Presto":        "8080",
	"Hive":          "10000",
	"Kafka":         "9092",
	"Redpanda":      "9092",
	"ElasticSearch": "9200",
	"OpenSearch":    "9200",
	"Airflow":       "8080",
	"Mlflow":        "5000",
}

// loadServiceHosts reads the address of every service OpenMetadata exposes one for. A server that hides connections, or an account that cannot read them, leaves the map empty and assets fall back to no zone.
func (c *collector) loadServiceHosts(ctx context.Context, client *client) {
	for _, path := range serviceListPaths {
		services, supported, err := listOptional[service](ctx, client, path, "connection", c.config.PageSize, false)
		if err != nil {
			log.Debug().Err(err).Str("path", path).Msg("Could not read service connections, assets of these services have no zone")
			continue
		}
		if !supported {
			continue
		}
		for _, svc := range services {
			if host := serviceHost(svc); host != "" {
				c.serviceHosts[svc.Name] = host
			}
		}
	}
}

// serviceHost is the host:port a service's connection points at, lowercase and with the service type's default port when it names none. It is empty when the connection holds no address.
func serviceHost(svc service) string {
	for _, key := range connectionAddressKeys {
		raw, _ := svc.Connection.Config[key].(string)
		if host := hostFromAddress(raw, defaultPorts[svc.ServiceType]); host != "" {
			return host
		}
	}
	return ""
}

// hostFromAddress reads the first host out of an address that may be a URL, a host:port pair or a comma separated list of either.
func hostFromAddress(raw, defaultPort string) string {
	scheme := ""
	if i := strings.Index(raw, "://"); i >= 0 {
		scheme, raw = strings.ToLower(raw[:i]), raw[i+3:]
	}
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		raw = raw[at+1:]
	}
	if i := strings.IndexAny(raw, "/?"); i >= 0 {
		raw = raw[:i]
	}
	first, _, _ := strings.Cut(raw, ",")
	first = strings.ToLower(strings.TrimSpace(first))
	if first == "" {
		return ""
	}
	if host, port, err := net.SplitHostPort(first); err == nil {
		return net.JoinHostPort(host, port)
	}
	port := defaultPort
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return ""
		}
	}
	return net.JoinHostPort(strings.Trim(first, "[]"), port)
}
