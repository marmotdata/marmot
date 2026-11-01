package nifi

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "nifi", meta.ID)
	assert.Equal(t, "Apache NiFi", meta.Name)
	assert.Equal(t, "nifi", meta.Icon)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_RequiresHost(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"username": "marmot", "password": "secret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "nifi.example.com:8443"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://nifi.example.com:8443"})
	require.NoError(t, err)

	assert.True(t, source.config.VerifySSL)
	assert.True(t, source.config.IncludeProcessors)
	assert.False(t, source.config.IncludePorts)
	assert.True(t, source.config.DiscoverLineage)
	assert.Empty(t, source.config.RootProcessGroup)
}

func TestValidate_KeepsExplicitFalse(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{
		"host": "https://nifi.example.com:8443", "verify_ssl": false, "include_processors": false, "discover_lineage": false, "include_ports": true,
	})
	require.NoError(t, err)

	assert.False(t, source.config.VerifySSL)
	assert.False(t, source.config.IncludeProcessors)
	assert.False(t, source.config.DiscoverLineage)
	assert.True(t, source.config.IncludePorts)
}

func TestValidate_TrimsTheTrailingSlashFromHost(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://nifi.example.com:8443/"})
	require.NoError(t, err)

	assert.Equal(t, "https://nifi.example.com:8443", source.config.Host)
}

func TestValidate_RequiresPasswordWithUsername(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://nifi.example.com:8443", "username": "marmot"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestValidate_RequiresUsernameWithPassword(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://nifi.example.com:8443", "password": "secret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_RequiresClientKeyWithClientCert(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://nifi.example.com:8443", "client_cert": "/etc/nifi/client.pem"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_key")
}

func TestValidate_AcceptsATokenOnItsOwn(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://nifi.example.com:8443", "token": "eyJ.fake.jwt"})
	require.NoError(t, err)

	assert.Equal(t, "eyJ.fake.jwt", source.config.Token)
}

func TestValidate_AcceptsNoCredentialsForAnUnsecuredInstall(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "http://nifi.example.com:8080"})

	require.NoError(t, err)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "https://nifi.example.com:8443",
		"filter": map[string]interface{}{
			"include": []interface{}{"^NiFi Flow/Ingest.*"},
			"exclude": []interface{}{".*Scratch.*"},
		},
	})

	require.NoError(t, err)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{Host: "https://nifi.example.com:8443"}

	assert.False(t, config.VerifySSL)
	assert.False(t, config.IncludeProcessors)
	assert.False(t, config.DiscoverLineage)
}
