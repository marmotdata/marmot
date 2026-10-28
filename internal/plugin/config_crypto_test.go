package plugin

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/crypto"
)

var nestedSpec = []pluginsdk.ConfigField{
	{Name: "host", Type: pluginsdk.FieldTypeString},
	{Name: "password", Type: pluginsdk.FieldTypeString, Sensitive: true},
	{Name: "authentication", Type: pluginsdk.FieldTypeObject, Fields: []pluginsdk.ConfigField{
		{Name: "username", Type: pluginsdk.FieldTypeString},
		{Name: "password", Type: pluginsdk.FieldTypeString, Sensitive: true},
	}},
}

func newTestEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	enc, err := crypto.NewEncryptor(key)
	require.NoError(t, err)
	return enc
}

func nestedConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":     "kafka:9092",
		"password": "top-secret",
		"authentication": map[string]interface{}{
			"username": "marmot",
			"password": "nested-secret",
		},
	}
}

func TestEncryptConfigEncryptsNestedFields(t *testing.T) {
	enc := newTestEncryptor(t)
	config := nestedConfig()

	require.NoError(t, EncryptConfig(config, nestedSpec, enc))

	auth := config["authentication"].(map[string]interface{})
	assert.NotEqual(t, "top-secret", config["password"])
	assert.NotEqual(t, "nested-secret", auth["password"])
	assert.True(t, crypto.LooksEncrypted(auth["password"].(string)))
	assert.Equal(t, "marmot", auth["username"], "non-sensitive nested fields stay readable")
	assert.Equal(t, "kafka:9092", config["host"])

	require.NoError(t, DecryptConfig(config, nestedSpec, enc))
	assert.Equal(t, nestedConfig(), config)
}

func TestDecryptConfigDoesNotMutateShallowCopySource(t *testing.T) {
	enc := newTestEncryptor(t)
	stored := nestedConfig()
	require.NoError(t, EncryptConfig(stored, nestedSpec, enc))
	storedNested := stored["authentication"].(map[string]interface{})["password"]

	copied := make(map[string]interface{}, len(stored))
	for k, v := range stored {
		copied[k] = v
	}
	require.NoError(t, DecryptConfig(copied, nestedSpec, enc))

	assert.Equal(t, "nested-secret", copied["authentication"].(map[string]interface{})["password"])
	assert.Equal(t, storedNested, stored["authentication"].(map[string]interface{})["password"], "decrypting a shallow copy must leave the stored ciphertext alone")
}

func TestDecryptConfigFailsClosedOnPlaintext(t *testing.T) {
	enc := newTestEncryptor(t)
	err := DecryptConfig(nestedConfig(), nestedSpec, enc)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "top-secret", "errors must not echo secret values")
	assert.NotContains(t, err.Error(), "nested-secret")
}

func TestDecryptConfigRejectsOtherKey(t *testing.T) {
	config := nestedConfig()
	require.NoError(t, EncryptConfig(config, nestedSpec, newTestEncryptor(t)))
	assert.Error(t, DecryptConfig(config, nestedSpec, newTestEncryptor(t)))
}

func TestEncryptPlaintextFields(t *testing.T) {
	enc := newTestEncryptor(t)

	t.Run("encrypts legacy nested plaintext only", func(t *testing.T) {
		config := nestedConfig()
		topCipher, err := enc.EncryptString("top-secret")
		require.NoError(t, err)
		config["password"] = topCipher

		changed, err := EncryptPlaintextFields(config, nestedSpec, enc)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, topCipher, config["password"], "existing ciphertext is not re-encrypted")

		require.NoError(t, DecryptConfig(config, nestedSpec, enc))
		assert.Equal(t, nestedConfig(), config)
	})

	t.Run("is a no-op on encrypted config", func(t *testing.T) {
		config := nestedConfig()
		require.NoError(t, EncryptConfig(config, nestedSpec, enc))
		changed, err := EncryptPlaintextFields(config, nestedSpec, enc)
		require.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("never wraps ciphertext from another key", func(t *testing.T) {
		config := nestedConfig()
		require.NoError(t, EncryptConfig(config, nestedSpec, newTestEncryptor(t)))
		before := config["authentication"].(map[string]interface{})["password"]

		changed, err := EncryptPlaintextFields(config, nestedSpec, enc)
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, before, config["authentication"].(map[string]interface{})["password"])
	})

	t.Run("leaves masks alone", func(t *testing.T) {
		config := map[string]interface{}{"password": SensitiveMask}
		changed, err := EncryptPlaintextFields(config, nestedSpec, enc)
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, SensitiveMask, config["password"])
	})
}

func TestMaskSensitiveFieldsFromSpecDoesNotMutateSource(t *testing.T) {
	config := nestedConfig()

	masked := MaskSensitiveFieldsFromSpec(config, nestedSpec)

	assert.Equal(t, SensitiveMask, masked["password"])
	assert.Equal(t, SensitiveMask, masked["authentication"].(map[string]interface{})["password"])
	assert.Equal(t, nestedConfig(), map[string]interface{}(config), "masking must not change the caller's nested maps")
}
