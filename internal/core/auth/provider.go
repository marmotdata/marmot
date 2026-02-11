package auth

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/user"
)

// extractGroups extracts group names from a userInfo claims map using the specified claim key.
func extractGroups(userInfo map[string]interface{}, groupClaim string) []string {
	groups := []string{}

	groupsRaw, ok := userInfo[groupClaim]
	if !ok {
		return groups
	}

	switch v := groupsRaw.(type) {
	case []interface{}:
		for _, g := range v {
			if groupStr, ok := g.(string); ok {
				groups = append(groups, groupStr)
			}
		}
	case []string:
		groups = v
	case string:
		groups = []string{v}
	}

	return groups
}

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

func (m *OAuthManager) GetProviderNames() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}
