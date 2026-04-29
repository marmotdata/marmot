package oauth2

import (
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
)

type Provider struct {
	fosite.OAuth2Provider
	Store *Store
}

func NewProvider(hmacSecret []byte) *Provider {
	store := NewStore()

	config := &fosite.Config{
		GlobalSecret:                hmacSecret,
		AuthorizeCodeLifespan:       10 * time.Minute,
		AccessTokenLifespan:         24 * time.Hour,
		EnforcePKCE:                 true,
		EnablePKCEPlainChallengeMethod: false,
		SendDebugMessagesToClients:  false,
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
