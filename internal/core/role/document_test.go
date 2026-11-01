package role_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/role"
)

// recordingStore remembers what Create was asked to attach.
type recordingStore struct {
	*mockStore
	created role.CreateInput
}

func (s *recordingStore) Create(ctx context.Context, input role.CreateInput) (*role.Role, error) {
	s.created = input
	return s.mockStore.Create(ctx, input)
}

func storeWithPermissions() *recordingStore {
	store := &recordingStore{mockStore: newMockStore()}
	store.permissions = []role.Permission{
		{ID: "p-view", ResourceType: "assets", Action: "view"},
		{ID: "p-query", ResourceType: "assets", Action: "query"},
		{ID: "p-glossary", ResourceType: "glossary", Action: "view"},
	}
	return store
}

func TestCreateResolvesPermissionKeys(t *testing.T) {
	store := storeWithPermissions()
	svc := role.NewService(store)

	_, err := svc.Create(context.Background(), role.CreateInput{
		Name:           "analyst",
		PermIDs:        []string{"p-glossary"},
		PermissionKeys: []string{"assets:view", "assets:query", "glossary:view"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got := slices.Clone(store.created.PermIDs)
	slices.Sort(got)
	if want := []string{"p-glossary", "p-query", "p-view"}; !slices.Equal(got, want) {
		t.Errorf("attached %v, want %v: keys and ids union without duplicates", got, want)
	}
}

func TestCreateRefusesUnknownPermissionKey(t *testing.T) {
	svc := role.NewService(storeWithPermissions())
	_, err := svc.Create(context.Background(), role.CreateInput{Name: "x", PermissionKeys: []string{"assets:fly"}})
	if !errors.Is(err, role.ErrUnknownPermission) {
		t.Fatalf("want ErrUnknownPermission, got %v", err)
	}
}

func TestGetReturnsPermissionKeys(t *testing.T) {
	store := storeWithPermissions()
	store.roles["r1"] = &role.Role{ID: "r1", Name: "viewer", Permissions: []role.Permission{store.permissions[2], store.permissions[0]}}
	r, err := role.NewService(store).Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if want := []string{"assets:view", "glossary:view"}; !slices.Equal(r.PermissionKeys, want) {
		t.Errorf("keys = %v, want %v, sorted so a document read back diffs cleanly", r.PermissionKeys, want)
	}
}

func TestValidateDocumentListsEveryProblem(t *testing.T) {
	store := storeWithPermissions()
	store.roles["r1"] = &role.Role{ID: "r1", Name: "viewer"}
	svc := role.NewService(store)
	ctx := context.Background()

	problems, err := svc.ValidateDocument(ctx, role.CreateInput{Name: "Viewer", PermissionKeys: []string{"assets:view", "assets:fly", "nope:x"}}, "")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	paths := []string{}
	for _, p := range problems {
		paths = append(paths, p.Path)
	}
	if want := []string{"name", "permission_keys[1]", "permission_keys[2]"}; !slices.Equal(paths, want) {
		t.Errorf("problem paths = %v, want %v", paths, want)
	}

	problems, _ = svc.ValidateDocument(ctx, role.CreateInput{Name: "viewer", PermissionKeys: []string{"assets:view"}}, "r1")
	if len(problems) != 0 {
		t.Errorf("a role keeping its own name is not a clash, got %v", problems)
	}

	problems, _ = svc.ValidateDocument(ctx, role.CreateInput{Name: "  "}, "")
	if len(problems) != 1 || problems[0].Path != "name" {
		t.Errorf("a blank name is a problem, got %v", problems)
	}
}

func TestGetTreatsDeletedRolesAsGone(t *testing.T) {
	store := storeWithPermissions()
	gone := time.Now()
	store.roles["r1"] = &role.Role{ID: "r1", Name: "old", DeletedAt: &gone}
	if _, err := role.NewService(store).Get(context.Background(), "r1"); !errors.Is(err, role.ErrNotFound) {
		t.Errorf("a deleted role must read as not found, got %v", err)
	}
}
