package runs

import (
	"errors"
	"fmt"

	"github.com/marmotdata/marmot/internal/config"
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
