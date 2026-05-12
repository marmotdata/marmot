package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CachedToken returns the OAuth token written by `marmot login` for the
// named context, or nil if missing or expired.
func CachedToken(context string) Credential {
	if context == "" {
		return nil
	}
	dir, err := ConfigDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		return nil
	}
	var store struct {
		Tokens map[string]struct {
			AccessToken string    `json:"access_token"`
			TokenType   string    `json:"token_type"`
			ExpiresAt   time.Time `json:"expires_at,omitempty"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil
	}
	entry, ok := store.Tokens[context]
	if !ok || entry.AccessToken == "" {
		return nil
	}
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		return nil
	}
	return &bearerCred{token: entry.AccessToken, source: fmt.Sprintf("cached:%s", context)}
}
