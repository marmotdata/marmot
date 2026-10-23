package plugin

import (
	"fmt"
	"maps"
	"strings"

	"github.com/marmotdata/marmot/internal/crypto"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// EncryptConfig encrypts sensitive fields in a plugin config using the ConfigSpec, including fields nested inside object fields
func EncryptConfig(config map[string]interface{}, configSpec []pluginsdk.ConfigField, encryptor *crypto.Encryptor) error {
	return transformSensitiveFields(config, configSpec, func(path, value string) (string, error) {
		encrypted, err := encryptor.EncryptString(value)
		if err != nil {
			return "", fmt.Errorf("encrypting field %s: %w", path, err)
		}
		return encrypted, nil
	})
}

// DecryptConfig decrypts sensitive fields in a plugin config using the ConfigSpec, including fields nested inside object fields
func DecryptConfig(config map[string]interface{}, configSpec []pluginsdk.ConfigField, encryptor *crypto.Encryptor) error {
	return transformSensitiveFields(config, configSpec, func(path, value string) (string, error) {
		decrypted, err := encryptor.DecryptString(value)
		if err != nil {
			return "", fmt.Errorf("decrypting field %s: %w", path, err)
		}
		return decrypted, nil
	})
}

// EncryptPlaintextFields encrypts sensitive fields that are not already ciphertext and reports whether any changed, healing configs stored before nested fields were encrypted or while encryption was off
func EncryptPlaintextFields(config map[string]interface{}, configSpec []pluginsdk.ConfigField, encryptor *crypto.Encryptor) (bool, error) {
	changed := false
	err := transformSensitiveFields(config, configSpec, func(path, value string) (string, error) {
		// Values shaped like ciphertext are left alone even if they do not decrypt, so a wrong key can never wrap real ciphertext a second time.
		if value == SensitiveMask || crypto.LooksEncrypted(value) {
			return value, nil
		}
		encrypted, err := encryptor.EncryptString(value)
		if err != nil {
			return "", fmt.Errorf("encrypting field %s: %w", path, err)
		}
		changed = true
		return encrypted, nil
	})
	return changed, err
}

// transformSensitiveFields replaces every non-empty string at a sensitive path in config with the result of transform, copying nested maps on the way down so a shallow copy of config never changes its source
func transformSensitiveFields(config map[string]interface{}, configSpec []pluginsdk.ConfigField, transform func(path, value string) (string, error)) error {
	for _, path := range extractSensitiveFieldsFromSpec(configSpec, "") {
		parent, key := writableParentMap(config, path)
		if parent == nil {
			continue
		}

		value, ok := parent[key].(string)
		if !ok || value == "" {
			continue
		}

		transformed, err := transform(path, value)
		if err != nil {
			return err
		}
		parent[key] = transformed
	}

	return nil
}

// writableParentMap is parentMap but replaces each nested map along the path with a copy before descending into it
func writableParentMap(m map[string]interface{}, path string) (map[string]interface{}, string) {
	parts := strings.Split(path, ".")

	current := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return nil, ""
		}
		next = maps.Clone(next)
		current[part] = next
		current = next
	}

	return current, parts[len(parts)-1]
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
