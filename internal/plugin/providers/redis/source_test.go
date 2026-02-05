package redis

import (
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  plugin.RawPluginConfig
		wantErr string
	}{
		{
			name: "valid host config",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"port": 6379,
			},
		},
		{
			name:    "missing host",
			config:  plugin.RawPluginConfig{},
			wantErr: "host",
		},
		{
			name: "invalid port",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"port": 99999,
			},
			wantErr: "port",
		},
		{
			name: "with tls",
			config: plugin.RawPluginConfig{
				"host":         "localhost",
				"tls":          true,
				"tls_insecure": true,
			},
		},
		{
			name: "with filter",
			config: plugin.RawPluginConfig{
				"host": "localhost",
				"filter": map[string]interface{}{
					"include": []interface{}{"^db0$"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(tt.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_ValidateDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"host": "localhost",
	})
	require.NoError(t, err)
	assert.Equal(t, 6379, s.config.Port)
	assert.True(t, s.config.DiscoverAllDatabases)
}

func TestParseKeyspaceEntry(t *testing.T) {
	result := parseKeyspaceEntry("keys=100,expires=10,avg_ttl=1000")

	assert.Equal(t, "100", result["keys"])
	assert.Equal(t, "10", result["expires"])
	assert.Equal(t, "1000", result["avg_ttl"])
}

func TestParseKeyspaceEntry_Empty(t *testing.T) {
	result := parseKeyspaceEntry("")
	assert.Empty(t, result)
}

func TestParseInfoSection(t *testing.T) {
	info := `# Server
redis_version:7.2.4
redis_mode:standalone
os:Linux 6.1.0
uptime_in_seconds:86400

# Clients
connected_clients:10
blocked_clients:0

# Memory
used_memory_human:1.50M
maxmemory_policy:noeviction

# Replication
role:master

# Keyspace
db0:keys=100,expires=10,avg_ttl=1000
db1:keys=50,expires=5,avg_ttl=2000
`

	t.Run("Server section", func(t *testing.T) {
		server := parseInfoSection(info, "Server")
		assert.Equal(t, "7.2.4", server["redis_version"])
		assert.Equal(t, "standalone", server["redis_mode"])
		assert.Equal(t, "86400", server["uptime_in_seconds"])
	})

	t.Run("Clients section", func(t *testing.T) {
		clients := parseInfoSection(info, "Clients")
		assert.Equal(t, "10", clients["connected_clients"])
		assert.Equal(t, "0", clients["blocked_clients"])
	})

	t.Run("Memory section", func(t *testing.T) {
		memory := parseInfoSection(info, "Memory")
		assert.Equal(t, "1.50M", memory["used_memory_human"])
		assert.Equal(t, "noeviction", memory["maxmemory_policy"])
	})

	t.Run("Replication section", func(t *testing.T) {
		replication := parseInfoSection(info, "Replication")
		assert.Equal(t, "master", replication["role"])
	})

	t.Run("Keyspace section", func(t *testing.T) {
		keyspace := parseInfoSection(info, "Keyspace")
		assert.Equal(t, "keys=100,expires=10,avg_ttl=1000", keyspace["db0"])
		assert.Equal(t, "keys=50,expires=5,avg_ttl=2000", keyspace["db1"])
	})

	t.Run("nonexistent section", func(t *testing.T) {
		result := parseInfoSection(info, "Nonexistent")
		assert.Empty(t, result)
	})
}
