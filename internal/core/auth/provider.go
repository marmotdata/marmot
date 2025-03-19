package auth

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/user"
)

type OAuthProvider interface {
	GetAuthURL(state string) string
	HandleCallback(ctx context.Context, code string) (*user.User, error)
	Name() string
	Type() string
}

type OAuthManager struct {
	providers map[string]OAuthProvider
}

func NewOAuthManager() *OAuthManager {
	return &OAuthManager{
		providers: make(map[string]OAuthProvider),
	}
}

func (m *OAuthManager) RegisterProvider(provider OAuthProvider) {
	m.providers[provider.Type()] = provider
}

func (m *OAuthManager) GetProvider(providerType string) (OAuthProvider, bool) {
	provider, exists := m.providers[providerType]
	return provider, exists
}
