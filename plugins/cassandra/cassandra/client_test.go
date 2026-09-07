package cassandra

import (
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHosts_AddsDefaultPortToBareHost(t *testing.T) {
	hosts, err := parseHosts([]string{"cassandra-1"}, 9042)
	require.NoError(t, err)

	assert.Equal(t, []string{"cassandra-1:9042"}, hosts)
}

func TestParseHosts_KeepsExplicitPort(t *testing.T) {
	hosts, err := parseHosts([]string{"cassandra-1:9142"}, 9042)
	require.NoError(t, err)

	assert.Equal(t, []string{"cassandra-1:9142"}, hosts)
}

func TestParseHosts_HandlesIPv6Literals(t *testing.T) {
	hosts, err := parseHosts([]string{"[::1]", "[fe80::1]:9142", "::1"}, 9042)
	require.NoError(t, err)

	assert.Equal(t, []string{"[::1]:9042", "[fe80::1]:9142", "[::1]:9042"}, hosts)
}

func TestParseHosts_TrimsWhitespaceAndDropsBlanks(t *testing.T) {
	hosts, err := parseHosts([]string{" cassandra-1 ", "", "  "}, 9042)
	require.NoError(t, err)

	assert.Equal(t, []string{"cassandra-1:9042"}, hosts)
}

func TestParseHosts_RejectsNonNumericPort(t *testing.T) {
	_, err := parseHosts([]string{"cassandra-1:cql"}, 9042)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")
}

func TestParseHosts_RejectsMissingHostname(t *testing.T) {
	_, err := parseHosts([]string{":9042"}, 9042)
	require.Error(t, err)
}

func TestParseHosts_RejectsAllBlank(t *testing.T) {
	_, err := parseHosts([]string{"", " "}, 9042)
	require.Error(t, err)
}

func TestClusterConfig_UsesHostsAndTimeouts(t *testing.T) {
	cluster := clusterConfig(&Config{
		Hosts:                 []string{"cassandra-1:9042", "cassandra-2:9042"},
		Port:                  9042,
		ConnectTimeoutSeconds: 7,
	})

	assert.Equal(t, []string{"cassandra-1:9042", "cassandra-2:9042"}, cluster.Hosts)
	assert.Equal(t, 9042, cluster.Port)
	assert.Equal(t, "7s", cluster.ConnectTimeout.String())
	assert.Equal(t, queryTimeout, cluster.Timeout)
	assert.Equal(t, gocql.LocalOne, cluster.Consistency)
	// Zero lets the driver negotiate the highest protocol the cluster supports.
	assert.Equal(t, 0, cluster.ProtoVersion)
	assert.False(t, cluster.DisableInitialHostLookup)
}

func TestClusterConfig_NoAuthenticatorWithoutUsername(t *testing.T) {
	cluster := clusterConfig(&Config{Hosts: []string{"cassandra-1:9042"}})

	assert.Nil(t, cluster.Authenticator)
}

func TestClusterConfig_PasswordAuthenticatorWhenUsernameSet(t *testing.T) {
	cluster := clusterConfig(&Config{
		Hosts:    []string{"cassandra-1:9042"},
		Username: "marmot",
		Password: "secret",
	})

	auth, ok := cluster.Authenticator.(gocql.PasswordAuthenticator)
	require.True(t, ok)
	assert.Equal(t, "marmot", auth.Username)
	assert.Equal(t, "secret", auth.Password)
}

func TestClusterConfig_DatacenterPinsHostFilterAndPolicy(t *testing.T) {
	cluster := clusterConfig(&Config{Hosts: []string{"cassandra-1:9042"}, Datacenter: "dc1"})

	assert.NotNil(t, cluster.HostFilter)
	assert.NotNil(t, cluster.PoolConfig.HostSelectionPolicy)
}

func TestClusterConfig_NoHostFilterWithoutDatacenter(t *testing.T) {
	cluster := clusterConfig(&Config{Hosts: []string{"cassandra-1:9042"}})

	assert.Nil(t, cluster.HostFilter)
	assert.Nil(t, cluster.PoolConfig.HostSelectionPolicy)
}

func TestClusterConfig_NoTLSByDefault(t *testing.T) {
	cluster := clusterConfig(&Config{Hosts: []string{"cassandra-1:9042"}})

	assert.Nil(t, cluster.SslOpts)
}

func TestClusterConfig_TLSVerifiesHostUnlessSkipped(t *testing.T) {
	verifying := clusterConfig(&Config{Hosts: []string{"cassandra-1:9042"}, SSL: true, SSLCACert: "/etc/ssl/ca.pem"})
	require.NotNil(t, verifying.SslOpts)
	assert.Equal(t, "/etc/ssl/ca.pem", verifying.SslOpts.CaPath)
	assert.True(t, verifying.SslOpts.EnableHostVerification)

	skipping := clusterConfig(&Config{Hosts: []string{"cassandra-1:9042"}, SSL: true, SSLSkipVerify: true})
	require.NotNil(t, skipping.SslOpts)
	assert.False(t, skipping.SslOpts.EnableHostVerification)
}
