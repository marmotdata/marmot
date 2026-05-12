package auth

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the Marmot CLI config directory (MARMOT_CONFIG_DIR or ~/.config/marmot).
func ConfigDir() (string, error) {
	if v := os.Getenv("MARMOT_CONFIG_DIR"); v != "" {
		return v, nil
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "marmot"), nil
}
