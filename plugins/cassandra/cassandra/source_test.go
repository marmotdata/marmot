package cassandra

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"hosts": []interface{}{"cassandra-1"}})
	require.NoError(t, err)
}

func TestValidate_MissingHostsFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hosts")
}

func TestValidate_EmptyHostsFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"hosts": []interface{}{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hosts")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"hosts": []interface{}{"cassandra-1"}})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, 9042, s.config.Port)
	assert.Equal(t, 10, s.config.ConnectTimeoutSeconds)
	assert.False(t, s.config.SSL)
	assert.False(t, s.config.SSLSkipVerify)
	assert.True(t, s.config.ExcludeSystemKeyspaces)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeIndexes)
	assert.True(t, s.config.IncludeStatistics)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"hosts":                    []interface{}{"cassandra-1"},
		"exclude_system_keyspaces": false,
		"include_views":            false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.ExcludeSystemKeyspaces)
	assert.False(t, s.config.IncludeViews)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeIndexes)
	assert.True(t, s.config.IncludeStatistics)
}

func TestValidate_NormalisesHostsToHostPort(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"hosts": []interface{}{"cassandra-1", "cassandra-2:9142"},
		"port":  9042,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"cassandra-1:9042", "cassandra-2:9142"}, s.config.Hosts)
}

func TestValidate_PortOutOfRangeFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"hosts": []interface{}{"cassandra-1"},
		"port":  70000,
	})
	require.Error(t, err)
}

func TestValidate_PasswordWithoutUsernameFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"hosts":    []interface{}{"cassandra-1"},
		"password": "secret",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_CACertWithoutSSLFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"hosts":       []interface{}{"cassandra-1"},
		"ssl_ca_cert": "/etc/ssl/ca.pem",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssl")
}

func TestValidate_AcceptsKeyspacesAndFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"hosts":     []interface{}{"cassandra-1"},
		"keyspaces": []interface{}{"shop", "billing"},
		"filter": map[string]interface{}{
			"include": []interface{}{"^shop.*"},
			"exclude": []interface{}{".*_tmp$"},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"shop", "billing"}, s.config.Keyspaces)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "cassandra", meta.ID)
	assert.Equal(t, "Cassandra", meta.Name)
	assert.Equal(t, "cassandra", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestSelectKeyspaces_ExcludesSystemKeyspacesByDefault(t *testing.T) {
	s := &Source{config: &Config{ExcludeSystemKeyspaces: true}}

	selected := s.selectKeyspaces([]keyspaceInfo{
		{Name: "shop"},
		{Name: "system"},
		{Name: "system_auth"},
		{Name: "system_schema"},
		{Name: "system_distributed"},
		{Name: "system_traces"},
	})

	require.Len(t, selected, 1)
	assert.Equal(t, "shop", selected[0].Name)
}

func TestSelectKeyspaces_KeepsSystemKeyspacesWhenConfigured(t *testing.T) {
	s := &Source{config: &Config{ExcludeSystemKeyspaces: false}}

	selected := s.selectKeyspaces([]keyspaceInfo{{Name: "shop"}, {Name: "system"}})

	assert.Len(t, selected, 2)
}

func TestSelectKeyspaces_ExplicitListWinsOverSystemExclusion(t *testing.T) {
	s := &Source{config: &Config{ExcludeSystemKeyspaces: true, Keyspaces: []string{"system", "shop"}}}

	selected := s.selectKeyspaces([]keyspaceInfo{{Name: "billing"}, {Name: "shop"}, {Name: "system"}})

	require.Len(t, selected, 2)
	assert.Equal(t, "system", selected[0].Name)
	assert.Equal(t, "shop", selected[1].Name)
}

func TestSelectKeyspaces_SkipsUnknownConfiguredKeyspace(t *testing.T) {
	s := &Source{config: &Config{Keyspaces: []string{"missing", "shop"}}}

	selected := s.selectKeyspaces([]keyspaceInfo{{Name: "shop"}})

	require.Len(t, selected, 1)
	assert.Equal(t, "shop", selected[0].Name)
}
