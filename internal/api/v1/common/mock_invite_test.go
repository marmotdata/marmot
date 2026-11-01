package common

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/settings"
	"github.com/marmotdata/marmot/internal/core/user"
)

// The invitation and provisioning methods the mock needs to satisfy user.Service, kept apart from the mock so the rest of it stays focused on what its tests exercise.

func (m *mockUserService) Invite(_ context.Context, _ user.InviteInput, _ string) (*user.Invitation, error) {
	return nil, nil
}
func (m *mockUserService) RenewInvite(_ context.Context, _, _ string) (*user.Invitation, error) {
	return nil, nil
}
func (m *mockUserService) RevokeInvite(_ context.Context, _ string) error { return nil }
func (m *mockUserService) GetInvite(_ context.Context, _ string) (*user.InviteInfo, error) {
	return nil, user.ErrInviteNotFound
}
func (m *mockUserService) AcceptInvite(_ context.Context, _, _, _ string) (*user.User, error) {
	return nil, user.ErrInviteNotFound
}
func (m *mockUserService) SignInSettings(_ context.Context) settings.SignIn {
	return settings.DefaultSignIn()
}
func (m *mockUserService) ProvisionSSO(ctx context.Context, input user.CreateUserInput) (*user.User, bool, error) {
	u, err := m.Create(ctx, input)
	return u, err == nil, err
}
func (m *mockUserService) SetRolesFromSSO(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockUserService) CountIdentitiesByProvider(_ context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}
func (m *mockUserService) GetMember(ctx context.Context, id string) (*user.User, error) {
	return m.Get(ctx, id)
}
