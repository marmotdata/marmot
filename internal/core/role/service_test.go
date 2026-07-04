package role_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/role"
)

// mockStore implements role.Store for unit tests.
type mockStore struct {
	roles       map[string]*role.Role
	permissions []role.Permission
	userCounts  map[string]bool // roleID → has users
}

func newMockStore() *mockStore {
	return &mockStore{
		roles:      make(map[string]*role.Role),
		userCounts: make(map[string]bool),
	}
}

func (m *mockStore) List(_ context.Context, includeDeleted bool) ([]*role.Role, error) {
	var out []*role.Role
	for _, r := range m.roles {
		if !includeDeleted && r.DeletedAt != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *mockStore) Get(_ context.Context, id string) (*role.Role, error) {
	r, ok := m.roles[id]
	if !ok {
		return nil, role.ErrNotFound
	}
	return r, nil
}

func (m *mockStore) GetByName(_ context.Context, name string) (*role.Role, error) {
	for _, r := range m.roles {
		if r.Name == name && r.DeletedAt == nil {
			return r, nil
		}
	}
	return nil, role.ErrNotFound
}

func (m *mockStore) Create(_ context.Context, input role.CreateInput) (*role.Role, error) {
	for _, r := range m.roles {
		if r.Name == input.Name && r.DeletedAt == nil {
			return nil, role.ErrAlreadyExists
		}
	}
	id := "role-" + input.Name
	r := &role.Role{ID: id, Name: input.Name, Description: input.Description}
	m.roles[id] = r
	return r, nil
}

func (m *mockStore) Update(_ context.Context, id string, input role.UpdateInput) (*role.Role, error) {
	r, ok := m.roles[id]
	if !ok {
		return nil, role.ErrNotFound
	}
	if input.Name != nil {
		r.Name = *input.Name
	}
	if input.Description != nil {
		r.Description = *input.Description
	}
	return r, nil
}

func (m *mockStore) SoftDelete(_ context.Context, id string) error {
	r, ok := m.roles[id]
	if !ok {
		return role.ErrNotFound
	}
	now := time.Now()
	r.DeletedAt = &now
	return nil
}

func (m *mockStore) AttachedPermissionIDs(_ context.Context, roleID string) ([]string, error) {
	r, ok := m.roles[roleID]
	if !ok {
		return nil, role.ErrNotFound
	}
	ids := make([]string, len(r.Permissions))
	for i, p := range r.Permissions {
		ids[i] = p.ID
	}
	return ids, nil
}

func (m *mockStore) ReplacePermissions(_ context.Context, roleID string, permIDs []string) error {
	r, ok := m.roles[roleID]
	if !ok {
		return role.ErrNotFound
	}
	perms := make([]role.Permission, 0, len(permIDs))
	for _, id := range permIDs {
		for _, p := range m.permissions {
			if p.ID == id {
				perms = append(perms, p)
			}
		}
	}
	r.Permissions = perms
	return nil
}

func (m *mockStore) HasUsers(_ context.Context, roleID string) (bool, error) {
	return m.userCounts[roleID], nil
}

func (m *mockStore) ListPermissions(_ context.Context) ([]role.Permission, error) {
	return m.permissions, nil
}

func TestService_Delete_BlocksSystemRole(t *testing.T) {
	store := newMockStore()
	store.roles["id-admin"] = &role.Role{ID: "id-admin", Name: "admin", IsSystem: true}

	svc := role.NewService(store)
	err := svc.Delete(context.Background(), "id-admin")

	if !errors.Is(err, role.ErrSystemRoleProtected) {
		t.Fatalf("expected ErrSystemRoleProtected, got %v", err)
	}
}

func TestService_Delete_BlocksWhenUsersAssigned(t *testing.T) {
	store := newMockStore()
	store.roles["id-custom"] = &role.Role{ID: "id-custom", Name: "custom", IsSystem: false}
	store.userCounts["id-custom"] = true

	svc := role.NewService(store)
	err := svc.Delete(context.Background(), "id-custom")

	if !errors.Is(err, role.ErrRoleInUse) {
		t.Fatalf("expected ErrRoleInUse, got %v", err)
	}
}

func TestService_Delete_SucceedsForCustomUnusedRole(t *testing.T) {
	store := newMockStore()
	store.roles["id-custom"] = &role.Role{ID: "id-custom", Name: "custom", IsSystem: false}

	svc := role.NewService(store)
	if err := svc.Delete(context.Background(), "id-custom"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.roles["id-custom"].DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set after soft-delete")
	}
}

func TestService_Update_BlocksAnyChangeToSystemRole(t *testing.T) {
	store := newMockStore()
	store.roles["id-admin"] = &role.Role{ID: "id-admin", Name: "admin", IsSystem: true, Description: "original"}

	svc := role.NewService(store)

	newName := "superadmin"
	if _, err := svc.Update(context.Background(), "id-admin", role.UpdateInput{Name: &newName}); !errors.Is(err, role.ErrSystemRoleProtected) {
		t.Fatalf("expected ErrSystemRoleProtected for rename, got %v", err)
	}

	newDesc := "updated"
	if _, err := svc.Update(context.Background(), "id-admin", role.UpdateInput{Description: &newDesc}); !errors.Is(err, role.ErrSystemRoleProtected) {
		t.Fatalf("expected ErrSystemRoleProtected for description change, got %v", err)
	}

	if store.roles["id-admin"].Description != "original" {
		t.Fatal("system role should not have been modified")
	}
}

func TestService_Update_AllowsChangesToCustomRole(t *testing.T) {
	store := newMockStore()
	store.roles["id-custom"] = &role.Role{ID: "id-custom", Name: "custom", IsSystem: false}

	svc := role.NewService(store)
	desc := "new description"
	r, err := svc.Update(context.Background(), "id-custom", role.UpdateInput{Description: &desc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Description != desc {
		t.Fatalf("expected description %q, got %q", desc, r.Description)
	}
}

func TestService_ReplacePermissions_BlocksSystemRole(t *testing.T) {
	perm := role.Permission{ID: "perm-1", Name: "view_assets", ResourceType: "assets", Action: "view"}

	store := newMockStore()
	store.permissions = []role.Permission{perm}
	store.roles["id-admin"] = &role.Role{ID: "id-admin", Name: "admin", IsSystem: true}

	svc := role.NewService(store)
	err := svc.ReplacePermissions(context.Background(), "id-admin", []string{perm.ID})

	if !errors.Is(err, role.ErrSystemRoleProtected) {
		t.Fatalf("expected ErrSystemRoleProtected, got %v", err)
	}
}

func TestService_ReplacePermissions_AllowsCustomRole(t *testing.T) {
	perm := role.Permission{ID: "perm-1", Name: "view_assets", ResourceType: "assets", Action: "view"}

	store := newMockStore()
	store.permissions = []role.Permission{perm}
	store.roles["id-custom"] = &role.Role{ID: "id-custom", Name: "custom", IsSystem: false}

	svc := role.NewService(store)
	if err := svc.ReplacePermissions(context.Background(), "id-custom", []string{perm.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.roles["id-custom"].Permissions) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(store.roles["id-custom"].Permissions))
	}
}

func TestService_Create_RejectsEmptyName(t *testing.T) {
	svc := role.NewService(newMockStore())
	_, err := svc.Create(context.Background(), role.CreateInput{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}
