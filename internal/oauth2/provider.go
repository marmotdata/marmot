package oauth2

import (
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
)

type Provider struct {
	fosite.OAuth2Provider
	Store *Store
	// Grants records which clients each person has allowed. Nil leaves tokens untied to a grant, as before grants existed.
	Grants GrantRepository
}

func NewProvider(hmacSecret []byte, repo Repository) *Provider {
	store := NewStore(repo)

	config := &fosite.Config{
		GlobalSecret:                   hmacSecret,
		AuthorizeCodeLifespan:          authorizeCodeLifespan,
		AccessTokenLifespan:            24 * time.Hour,
		EnforcePKCE:                    true,
		EnablePKCEPlainChallengeMethod: false,
		SendDebugMessagesToClients:     false,
	}

	provider := compose.Compose(
		config,
		store,
		compose.NewOAuth2HMACStrategy(config),
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2PKCEFactory,
	)

	return &Provider{
		OAuth2Provider: provider,
		Store:          store,
	}
}
