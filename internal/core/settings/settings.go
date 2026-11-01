// Package settings keeps the instance settings an administrator changes from the UI rather than the config file.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// KeySignIn holds SignIn.
	KeySignIn = "auth.sign_in"
	// KeySSOProviders holds which configured single sign-on providers an administrator has switched off.
	KeySSOProviders = "auth.sso_providers"
	// KeyLastFullReindex holds when a full search reindex last finished.
	KeyLastFullReindex = "search.last_full_reindex"
)

// SignIn is what happens when someone signs in, and what a new member starts with.
type SignIn struct {
	// CreateOnFirstSSO lets anyone the identity provider vouches for become a member by signing in; when off only invited people can.
	CreateOnFirstSSO bool `json:"create_on_first_sso"`
	// DefaultRole is given to a new member when none of their groups is mapped to a role.
	DefaultRole string `json:"default_role"`
	// DefaultTeamID is the team a new single sign-on member joins, empty for none.
	DefaultTeamID string `json:"default_team_id"`
	// AllowPasswordSignIn lets members sign in with a password. Administrators always can, so an outage at the identity provider cannot lock everyone out.
	AllowPasswordSignIn bool `json:"allow_password_sign_in"`
} // @name SignInSettings

// DefaultSignIn is how Marmot behaved before these settings existed, so an instance that never saves them keeps working the same way.
func DefaultSignIn() SignIn {
	return SignIn{
		CreateOnFirstSSO:    true,
		DefaultRole:         "user",
		AllowPasswordSignIn: true,
	}
}

// Store reads and writes settings, one JSON document per key.
type Store interface {
	Get(ctx context.Context, key string, dst any) (bool, error)
	Set(ctx context.Context, key string, value any, updatedBy string) error
}

type postgresStore struct {
	db *pgxpool.Pool
}

// NewPostgresStore returns a Store backed by the instance_settings table.
func NewPostgresStore(db *pgxpool.Pool) Store {
	return &postgresStore{db: db}
}

func (s *postgresStore) Get(ctx context.Context, key string, dst any) (bool, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT value FROM instance_settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading setting %s: %w", key, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false, fmt.Errorf("decoding setting %s: %w", key, err)
	}
	return true, nil
}

func (s *postgresStore) Set(ctx context.Context, key string, value any, updatedBy string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding setting %s: %w", key, err)
	}
	var by *string
	if updatedBy != "" {
		by = &updatedBy
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO instance_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, raw, by)
	if err != nil {
		return fmt.Errorf("writing setting %s: %w", key, err)
	}
	return nil
}

// GetSignIn returns the sign-in settings, or the defaults when none were saved.
func GetSignIn(ctx context.Context, s Store) (SignIn, error) {
	out := DefaultSignIn()
	if s == nil {
		return out, nil
	}
	if _, err := s.Get(ctx, KeySignIn, &out); err != nil {
		return DefaultSignIn(), err
	}
	return out, nil
}

// DisabledProviders returns the provider types an administrator switched off.
func DisabledProviders(ctx context.Context, s Store) (map[string]bool, error) {
	out := map[string]bool{}
	if s == nil {
		return out, nil
	}
	var stored struct {
		Disabled []string `json:"disabled"`
	}
	if _, err := s.Get(ctx, KeySSOProviders, &stored); err != nil {
		return out, err
	}
	for _, t := range stored.Disabled {
		out[t] = true
	}
	return out, nil
}

// SetProviderEnabled switches one provider on or off, leaving the others as they were.
func SetProviderEnabled(ctx context.Context, s Store, providerType string, enabled bool, updatedBy string) error {
	disabled, err := DisabledProviders(ctx, s)
	if err != nil {
		return err
	}
	if enabled {
		delete(disabled, providerType)
	} else {
		disabled[providerType] = true
	}
	list := make([]string, 0, len(disabled))
	for t := range disabled {
		list = append(list, t)
	}
	return s.Set(ctx, KeySSOProviders, map[string][]string{"disabled": list}, updatedBy)
}

// LastFullReindex returns when a full search reindex last finished, or nil if none has.
func LastFullReindex(ctx context.Context, s Store) (*time.Time, error) {
	var t time.Time
	found, err := s.Get(ctx, KeyLastFullReindex, &t)
	if err != nil || !found {
		return nil, err
	}
	return &t, nil
}

// Switches adapts a Store to what the sign-in code consults to learn which providers are switched off.
type Switches struct {
	Store Store
}

func (s Switches) DisabledProviders(ctx context.Context) (map[string]bool, error) {
	return DisabledProviders(ctx, s.Store)
}
