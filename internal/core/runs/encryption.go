package runs

import (
	"errors"
	"fmt"

	"github.com/marmotdata/marmot/pkg/config"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
)

var (
	ErrEncryptionNotConfigured = errors.New("encryption not configured: MARMOT_SERVER_ENCRYPTION_KEY must be set")
)

// GetEncryptor creates an encryptor from the config
func GetEncryptor(cfg *config.Config) (*crypto.Encryptor, error) {
	if cfg.Server.EncryptionKey == "" {
		return nil, ErrEncryptionNotConfigured
	}

	key, err := crypto.DecodeKey(cfg.Server.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}

	return crypto.NewEncryptor(key)
}

// EncryptScheduleConfig encrypts sensitive fields in a schedule's config
func EncryptScheduleConfig(schedule *Schedule, encryptor *crypto.Encryptor) error {
	if schedule.Config == nil || encryptor == nil {
		return nil
	}

	return plugin.EncryptConfigForPlugin(schedule.PluginID, schedule.Config, encryptor)
}

// DecryptScheduleConfig decrypts sensitive fields in a schedule's config
func DecryptScheduleConfig(schedule *Schedule, encryptor *crypto.Encryptor) error {
	if schedule.Config == nil || encryptor == nil {
		return nil
	}

	return plugin.DecryptConfigForPlugin(schedule.PluginID, schedule.Config, encryptor)
}

// MaskScheduleConfig replaces sensitive fields in a schedule's config with plugin.SensitiveMask so stored secrets never leave the server, dropping the config entirely if the plugin's spec is unknown
func MaskScheduleConfig(schedule *Schedule) {
	if schedule.Config == nil {
		return
	}

	entry, err := plugin.GetRegistry().Get(schedule.PluginID)
	if err != nil {
		schedule.Config = nil
		return
	}

	schedule.Config = plugin.MaskSensitiveFieldsFromSpec(schedule.Config, entry.Meta.ConfigSpec)
}

// RestoreMaskedScheduleConfig fills sensitive fields left as plugin.SensitiveMask in an incoming config from the previous decrypted config
func RestoreMaskedScheduleConfig(pluginID string, config, previous map[string]interface{}) error {
	entry, err := plugin.GetRegistry().Get(pluginID)
	if err != nil {
		return fmt.Errorf("getting plugin: %w", err)
	}

	plugin.RestoreMaskedFieldsFromSpec(config, previous, entry.Meta.ConfigSpec)
	return nil
}
