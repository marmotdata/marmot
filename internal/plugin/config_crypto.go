package plugin

import (
	"fmt"

	"github.com/marmotdata/marmot/internal/crypto"
)

// GetSensitiveFields returns a map of sensitive field names from ConfigSpec
func GetSensitiveFields(configSpec []ConfigField) map[string]bool {
	sensitive := make(map[string]bool)
	for _, field := range configSpec {
		if field.Sensitive {
			sensitive[field.Name] = true
		}
	}
	return sensitive
}

// EncryptConfig encrypts sensitive fields in a plugin config using the ConfigSpec
func EncryptConfig(config map[string]interface{}, configSpec []ConfigField, encryptor *crypto.Encryptor) error {
	sensitiveFields := GetSensitiveFields(configSpec)
	if len(sensitiveFields) == 0 {
		return nil
	}

	return encryptor.EncryptMap(config, sensitiveFields)
}

// DecryptConfig decrypts sensitive fields in a plugin config using the ConfigSpec
func DecryptConfig(config map[string]interface{}, configSpec []ConfigField, encryptor *crypto.Encryptor) error {
	sensitiveFields := GetSensitiveFields(configSpec)
	if len(sensitiveFields) == 0 {
		return nil
	}

	return encryptor.DecryptMap(config, sensitiveFields)
}

// EncryptConfigForPlugin encrypts sensitive fields for a specific plugin by ID
func EncryptConfigForPlugin(pluginID string, config map[string]interface{}, encryptor *crypto.Encryptor) error {
	entry, err := GetRegistry().Get(pluginID)
	if err != nil {
		return fmt.Errorf("getting plugin: %w", err)
	}

	return EncryptConfig(config, entry.Meta.ConfigSpec, encryptor)
}

// DecryptConfigForPlugin decrypts sensitive fields for a specific plugin by ID
func DecryptConfigForPlugin(pluginID string, config map[string]interface{}, encryptor *crypto.Encryptor) error {
	entry, err := GetRegistry().Get(pluginID)
	if err != nil {
		return fmt.Errorf("getting plugin: %w", err)
	}

	return DecryptConfig(config, entry.Meta.ConfigSpec, encryptor)
}
