package sftp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// baseConfig is the smallest config that validates, so a test only has
// to say what it is actually about.
func baseConfig(overrides pluginsdk.RawConfig) pluginsdk.RawConfig {
	raw := pluginsdk.RawConfig{
		"host":     "sftp.example.com",
		"username": "marmot",
		"password": "secret",
	}
	for key, value := range overrides {
		raw[key] = value
	}
	return raw
}

func validate(t *testing.T, raw pluginsdk.RawConfig) (*Config, error) {
	t.Helper()

	s := &Source{}
	if _, err := s.Validate(raw); err != nil {
		return nil, err
	}
	return s.config, nil
}

// testConfig is a validated config pointed at a tree on disk.
func testConfig(t *testing.T, root string, overrides pluginsdk.RawConfig) *Config {
	t.Helper()

	raw := baseConfig(pluginsdk.RawConfig{"root_directories": []string{root}})
	for key, value := range overrides {
		raw[key] = value
	}

	config, err := validate(t, raw)
	require.NoError(t, err)
	return config
}

// newPrivateKey returns a fresh Ed25519 key in the PEM form a user would
// paste into the config.
func newPrivateKey(t *testing.T) string {
	t.Helper()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(private, "")
	require.NoError(t, err)

	return string(pem.EncodeToMemory(block))
}

func TestValidate_RequiresAHost(t *testing.T) {
	_, err := validate(t, pluginsdk.RawConfig{"username": "marmot", "password": "secret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestValidate_RequiresAUsername(t *testing.T) {
	raw := baseConfig(nil)
	delete(raw, "username")

	_, err := validate(t, raw)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "username is required")
}

func TestValidate_RequiresAPasswordOrAPrivateKey(t *testing.T) {
	raw := baseConfig(nil)
	delete(raw, "password")

	_, err := validate(t, raw)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "password or private_key")
}

func TestValidate_AcceptsAPrivateKeyWithoutAPassword(t *testing.T) {
	raw := baseConfig(pluginsdk.RawConfig{"private_key": newPrivateKey(t)})
	delete(raw, "password")

	config, err := validate(t, raw)

	require.NoError(t, err)
	assert.NotEmpty(t, config.PrivateKey)
}

func TestValidate_RejectsAnUnreadablePrivateKey(t *testing.T) {
	raw := baseConfig(pluginsdk.RawConfig{"private_key": "not a key"})

	_, err := validate(t, raw)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing private_key")
}

func TestValidate_RejectsAnEncryptedKeyWithNoPassphrase(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKeyWithPassphrase(private, "", []byte("hunter2"))
	require.NoError(t, err)

	_, err = validate(t, baseConfig(pluginsdk.RawConfig{"private_key": string(pem.EncodeToMemory(block))}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "private_key_passphrase")
}

func TestValidate_AcceptsAnEncryptedKeyWithItsPassphrase(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKeyWithPassphrase(private, "", []byte("hunter2"))
	require.NoError(t, err)

	_, err = validate(t, baseConfig(pluginsdk.RawConfig{
		"private_key":            string(pem.EncodeToMemory(block)),
		"private_key_passphrase": "hunter2",
	}))

	require.NoError(t, err)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	config, err := validate(t, baseConfig(nil))

	require.NoError(t, err)
	assert.Equal(t, 22, config.Port)
	assert.Equal(t, []string{"/"}, config.RootDirectories)
	assert.Equal(t, 10, config.MaxDepth)
	assert.Equal(t, 20000, config.MaxFiles)
	assert.Equal(t, 200, config.SampleRows)
	assert.Equal(t, int64(33554432), config.MaxReadBytes)
	assert.True(t, config.IncludeColumns)
	assert.True(t, config.IncludeStatistics)
	assert.False(t, config.FollowSymlinks)
	assert.False(t, config.StructuredOnly)
}

func TestValidate_KeepsAnExplicitFalse(t *testing.T) {
	// A default of true must not overwrite a user who wrote false.
	config, err := validate(t, baseConfig(pluginsdk.RawConfig{
		"include_columns":    false,
		"include_statistics": false,
	}))

	require.NoError(t, err)
	assert.False(t, config.IncludeColumns)
	assert.False(t, config.IncludeStatistics)
}

func TestValidate_StripsASchemeFromTheHost(t *testing.T) {
	config, err := validate(t, baseConfig(pluginsdk.RawConfig{"host": "sftp://files.example.com"}))

	require.NoError(t, err)
	assert.Equal(t, "files.example.com", config.Host)
}

func TestValidate_NormalisesRootDirectories(t *testing.T) {
	config, err := validate(t, baseConfig(pluginsdk.RawConfig{
		"root_directories": []string{"/data/incoming/", "/data/incoming"},
	}))

	require.NoError(t, err)
	assert.Equal(t, []string{"/data/incoming"}, config.RootDirectories)
}

func TestValidate_RejectsAPortOutOfRange(t *testing.T) {
	_, err := validate(t, baseConfig(pluginsdk.RawConfig{"port": 70000}))

	require.Error(t, err)
}

func TestValidate_RejectsMoreSampleRowsThanAllowed(t *testing.T) {
	_, err := validate(t, baseConfig(pluginsdk.RawConfig{"sample_rows": 50000}))

	require.Error(t, err)
}

func TestValidate_AcceptsAHostKeyInAuthorizedKeysForm(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewPublicKey(public)
	require.NoError(t, err)

	_, err = validate(t, baseConfig(pluginsdk.RawConfig{
		"host_key": string(ssh.MarshalAuthorizedKey(signer)),
	}))

	require.NoError(t, err)
}

func TestValidate_AcceptsAHostKeyInKnownHostsForm(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(public)
	require.NoError(t, err)

	// A known_hosts line is an authorized_keys line with the host in front.
	line := "files.example.com " + string(ssh.MarshalAuthorizedKey(key))

	_, err = validate(t, baseConfig(pluginsdk.RawConfig{"host_key": line}))

	require.NoError(t, err)
}

func TestValidate_RejectsAnUnreadableHostKey(t *testing.T) {
	_, err := validate(t, baseConfig(pluginsdk.RawConfig{"host_key": "ssh-ed25519 not-base64"}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing host_key")
}

func TestAuthMethods_OffersKeyboardInteractiveAlongsideThePassword(t *testing.T) {
	// Some servers only accept a password through keyboard-interactive.
	config, err := validate(t, baseConfig(nil))
	require.NoError(t, err)

	methods, err := authMethods(config)

	require.NoError(t, err)
	assert.Len(t, methods, 2)
}

func TestMeta_DeclaresWhatThePluginProduces(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "sftp", meta.ID)
	assert.Equal(t, "SFTP", meta.Name)
	assert.Equal(t, "storage", meta.Category)
	assert.Equal(t, "sftp", meta.Icon)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}
