package cassandra

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/rs/zerolog/log"
)

// queryTimeout bounds every single CQL query, like the other database
// plugins' 30 second per-query limit.
const queryTimeout = 30 * time.Second

// parseHosts normalises contact points to host:port, filling in the
// default port for entries given without one. Whitespace is trimmed and
// blank entries dropped so a stray newline in a YAML list does not become
// a host.
func parseHosts(hosts []string, defaultPort int) ([]string, error) {
	var out []string
	for _, raw := range hosts {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		host, port, err := net.SplitHostPort(entry)
		if err != nil {
			// No port: a bare hostname, IPv4 or bracketed IPv6 literal.
			host = strings.Trim(entry, "[]")
			port = strconv.Itoa(defaultPort)
		} else if _, err := strconv.Atoi(port); err != nil {
			return nil, fmt.Errorf("host %q: invalid port %q", entry, port)
		}
		if host == "" {
			return nil, fmt.Errorf("host %q: missing hostname", entry)
		}

		out = append(out, net.JoinHostPort(host, port))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no hosts configured")
	}
	return out, nil
}

// clusterConfig turns the validated config into a gocql cluster config.
func clusterConfig(config *Config) *gocql.ClusterConfig {
	cluster := gocql.NewCluster(config.Hosts...)
	cluster.Port = config.Port
	cluster.ConnectTimeout = time.Duration(config.ConnectTimeoutSeconds) * time.Second
	cluster.Timeout = queryTimeout
	// LocalOne keeps schema reads working when a remote datacenter is down.
	cluster.Consistency = gocql.LocalOne
	// Read-only metadata queries need no more than one connection per node.
	cluster.NumConns = 1
	cluster.Logger = driverLogger{}

	if config.Username != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: config.Username,
			Password: config.Password,
		}
	}

	if config.Datacenter != "" {
		cluster.HostFilter = gocql.DataCentreHostFilter(config.Datacenter)
		cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.DCAwareRoundRobinPolicy(config.Datacenter))
	}

	if config.SSL {
		cluster.SslOpts = &gocql.SslOptions{
			CaPath:                 config.SSLCACert,
			EnableHostVerification: !config.SSLSkipVerify,
		}
	}

	return cluster
}

// connect opens a session against the configured contact points.
func connect(config *Config) (*gocql.Session, error) {
	session, err := clusterConfig(config).CreateSession()
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	log.Debug().Strs("hosts", config.Hosts).Msg("Successfully connected to Cassandra")

	return session, nil
}

// driverLogger routes gocql's own messages through zerolog at debug level,
// so the host's log stays readable while driver chatter is still available
// when debugging a connection.
type driverLogger struct{}

func (driverLogger) Print(v ...interface{}) { log.Debug().Msg(strings.TrimSpace(fmt.Sprint(v...))) }
func (driverLogger) Printf(format string, v ...interface{}) {
	log.Debug().Msg(strings.TrimSpace(fmt.Sprintf(format, v...)))
}
func (driverLogger) Println(v ...interface{}) { log.Debug().Msg(strings.TrimSpace(fmt.Sprintln(v...))) }
