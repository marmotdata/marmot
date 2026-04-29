package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// TokenEntry holds a cached OAuth token for a context.
type TokenEntry struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// CredentialStore holds all cached tokens, keyed by context name.
type CredentialStore struct {
	Tokens map[string]*TokenEntry `json:"tokens"`
}

func credentialsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func loadCredentials() (*CredentialStore, error) {
	p, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &CredentialStore{Tokens: make(map[string]*TokenEntry)}, nil
		}
		return nil, err
	}

	var store CredentialStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	if store.Tokens == nil {
		store.Tokens = make(map[string]*TokenEntry)
	}
	return &store, nil
}

func saveCredentials(store *CredentialStore) error {
	p, err := credentialsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p, data, 0o600)
}

// getCachedToken returns the cached access token for a context if it exists
// and is not expired (with a 30-second safety margin).
func getCachedToken(contextName string) (string, bool) {
	store, err := loadCredentials()
	if err != nil {
		return "", false
	}

	entry, ok := store.Tokens[contextName]
	if !ok {
		return "", false
	}

	if time.Now().Add(30 * time.Second).After(entry.ExpiresAt) {
		return "", false
	}

	return entry.AccessToken, true
}

// setCachedToken stores a token for the given context.
func setCachedToken(contextName, token, tokenType string, expiresIn int) error {
	store, err := loadCredentials()
	if err != nil {
		return err
	}

	store.Tokens[contextName] = &TokenEntry{
		AccessToken: token,
		TokenType:   tokenType,
		ExpiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}

	return saveCredentials(store)
}

// deleteCachedToken removes the cached token for the given context.
func deleteCachedToken(contextName string) error {
	store, err := loadCredentials()
	if err != nil {
		return err
	}

	delete(store.Tokens, contextName)
	return saveCredentials(store)
}
